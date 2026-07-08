## Context

Every Claude Code lifecycle event lands in one function: `agents.HandleHook` (invoked by
`be hook claude record`). It reads the prior per-session record, maps the event to a status via
`StatusFor`, resolves the agent's tmux location and cwd, and persists the record - best-effort,
swallowing errors so a hook misconfiguration can never block the agent. This runs as a short-lived
process on every event, independent of whether `be dash` is open.

That makes `HandleHook` the natural and only sensible home for notifications. A notification exists
*because you are not looking*; a notifier living in the dash refresh loop would only fire while you
are already watching the state change, which is pointless. The hook path fires headless and over
SSH, and it already has the exact state transition in scope - the previous status is `rec.Status`
right before `StatusFor` overwrites it.

birdseye's status vocabulary is already the notification taxonomy: `needs-attention` is "come
answer me", `working -> idle` is "your turn is done". The mute the user already sets to park an
agent is already "leave this one alone". So the feature is mostly *observing an edge the code
already computes* and *reusing intents that already exist*, plus a thin delivery seam.

## Goals / Non-Goals

**Goals**
- Fire an OS notification on exactly two edges - into `needs-attention`, and `working -> idle` -
  from the hook path, working with the dash closed.
- Never break the agent's hook chain: all notification work is best-effort.
- Reuse the existing per-location mute as the suppression control; no new user concept.
- Keep delivery hackable: auto-detect a sane default, allow a command template for anything else.
- One notification per agent per qualifying edge, with no timers or cross-agent state.

**Non-Goals**
- No change to status *detection* (title detector, needs-attention latch, batched tmux query).
- No focus/away suppression, no OSC escape channels, no digest/debounce (all deferred).
- No new CLI verb; `be config` surfaces the keys without help from this change.

## Decisions

### The notifier lives in `HandleHook`, keyed off the observed edge

`HandleHook` already does `rec, _ := readRecord(dir, in.SessionID)` and then, for non-`SessionEnd`
events, `rec.Status = StatusFor(event, in)`. We capture `old := rec.Status` before that assignment
and compare against the new value. The edge classifier is deliberately narrow:

```
into needs-attention : new == StatusNeedsAttention && old != StatusNeedsAttention
finished             : old == StatusWorking && new == StatusIdle
```

Everything else is silent. `into needs-attention` is defined as *entering* the state, not being in
it, so a `Notification` hook that re-fires while already blocked does not re-notify - which is what
makes "one per agent per edge" fall out for free without a debounce. `finished` is scoped tightly
to `working -> idle` so that an idle nudge (`Notification` whose message maps to `StatusIdle`)
arriving while already idle does not read as a fresh finish.

Dispatch happens *after* the record is written and is fully wrapped so any error (config parse,
missing notifier binary, command failure) is discarded. The status write must not depend on the
notification succeeding.

### Suppression reuses the per-location mute

Mute is stored per tmux location in `mutes.json` (`readMutes`), keyed by `locationKey(pane,
session, window, id)`. The hook path has all four inputs (`resolveTmux` gives pane/session/window;
`in.SessionID` gives the id), so it computes the same key the view uses and skips emission when the
location is muted. This means muting an agent in the dash silences both its band priority *and* its
pushes through one intent, with no second toggle. Reading the mute file on each hook event is cheap
and matches how the rest of the hook path already touches the state dir.

### Config: `[agents.notify]`, strict, three keys

A `Notify` struct nests on the existing `Agents` config struct:

```
[agents.notify]
command         = ""     # empty -> auto-detect platform notifier
needs_attention = true
finished        = true
```

`command` empty selects the auto-detect notifier. The two booleans gate their respective edges;
both default true, so an absent `[agents.notify]` table means "notify on both, deliver
automatically". Defaults are applied in the same place the other `Agents` defaults are (so the
zero-value bool ambiguity is handled explicitly: absence means true, which the loader encodes by
defaulting the struct before overlaying user TOML - see the Risks note). Loading stays strict, so a
typo'd key is a `be config --check` error like any other.

### Delivery is a `Notifier` seam, default auto-detect, override by command

```go
type Notifier interface {
    Notify(n Notification) error
}
type Notification struct {
    Agent   string // title, e.g. the record's Title (session:dir)
    Status  Status // needs-attention | idle
    Repo    string // resolved repo label when known, else ""
    CWD     string
    Message string // human line, e.g. "birdseye: <agent> needs you"
}
```

- **auto-detect** (`command == ""`): pick the first available of `notify-send` (Linux),
  `osascript` (macOS); if neither is on `PATH`, fall back to emitting a terminal bell, and if even
  that is not sensible, no-op. Mirrors birdseye's "optional tool degrades gracefully" stance - a
  box without a notifier is not an error.
- **command template** (`command != ""`): run the user's command with the notification exported as
  environment variables `BE_AGENT`, `BE_STATUS`, `BE_REPO`, `BE_CWD`, `BE_MESSAGE`. This is the
  ntfy/Slack/Pushover escape hatch and keeps platform-specific integrations out of birdseye's tree.

The seam exists so the auto-detect default and the command path are two implementations behind one
call, and so a fake notifier can assert emission in tests without spawning processes.

### Why not OSC escape sequences

cmux and similar tools lean on OSC 9/777/99 to the terminal. For birdseye that channel is nearly
inert: the agent's own pane is exactly the pane you are not watching, and the outer terminal only
matters while the dash is open - which is when you are watching and would not want the push anyway.
The OS notification is the channel that reaches you when the tool's premise (you are away) holds, so
v1 ships only that. OSC support remains a possible later channel behind the same seam.

## Risks / Trade-offs

- **Bool defaults vs. strict TOML.** A missing `[agents.notify]` must mean *on*, but a bare `bool`
  zero-values to `false`. Resolve by seeding the `Notify` struct with `needs_attention = true,
  finished = true` before decoding user TOML over it (the same defaulting pattern already used for
  `Refresh`/`Command`/`Split`), so absence reads as true while an explicit `false` still wins.
- **No away-suppression in v1.** With the dash open you can get a `finished` push for an agent you
  are watching finish. Accepted for v1 to keep scope small; the deferred focus-file follow-up closes
  it. Mute is the available manual mitigation.
- **Reading mute + config on every hook event.** Two small file reads per event in a short-lived
  process. Negligible next to the tmux calls `HandleHook` already makes, and it keeps the notifier
  stateless.
- **Repo label may be absent at hook time.** The hook has cwd but not the reconciler's repo
  resolution. `Repo` is best-effort (empty when unresolved); the notification is still meaningful
  from `Agent` + `Status` alone.
