## Context

Status today is hook-sourced end to end. `HandleHook` (internal/agents/claude.go) writes a
per-session `record` on each Claude Code hook event; `StatusFor` maps the event to one of
working / idle / needs-attention / done. On each dash refresh `ClaudeSource.Agents()` reads
the records, GCs any whose pid is dead, dedups by tmux location, and returns `[]Agent`. The
record carries identity (session id, cwd), location (session/window/pane), liveness (pid),
and the last hook's status.

The flaw is structural: status is **edge-triggered but displayed as a level**. Between hook
events there is no signal, so a transition that fires no hook (Esc interrupt, crash, a
permission answered directly in the terminal) freezes the display on a stale value. We
confirmed by live probe that Claude broadcasts its current state out-of-band in the OSC
terminal title - a Braille spinner glyph (U+2800-U+28FF) while working, a `✳` (U+2733) when
idle - which tmux exposes as `#{pane_title}`. That is a level-true signal we can read fresh on
every refresh.

Constraints: keep the hook install contract unchanged; keep existing records valid; do not
regress the existing needs-attention / idle-notification behavior; stay cheap on refresh
(the dash redraws live).

## Goals / Non-Goals

**Goals:**

- Make the working/idle distinction reflect the agent's *current* terminal state, so it
  self-heals across interrupts, crashes, and in-terminal actions.
- Keep hooks authoritative for what they are good at: identity, existence (pid), needs-attention
  (the explicit `Notification`), and done (`SessionEnd` / dead process).
- Degrade gracefully: when no title is readable (not in tmux, title overridden, non-Claude
  terminal), fall back to the hook status - never worse than today.
- Leave a clean per-agent seam so title-less agents can be supported later without the view
  knowing how a level was derived.

**Non-Goals:**

- Building the buffer-regex or activity-diff fallback detectors now (seam only; Claude/title
  ships).
- Detecting needs-attention from the screen/title. It stays hook-sourced.
- Replacing hooks. This is correction, not removal.
- Lifting the title's task summary into the row label (a tempting follow-on; out of scope here).

## Decisions

**1. Correct at read time; leave the record shape unchanged.**
`StatusFor` keeps emitting its best guess (including working/idle) into the record. The new
level signal overrides working/idle in `ClaudeSource.Agents()`, at read time, per agent that
has a `TmuxPane`. *Why over having the hook stop emitting working/idle:* the recorded status is
the only signal when no title is readable, so keeping it preserves a safe fallback and keeps
the hook write path untouched. The override is additive.

**2. Read the title via one batched tmux call.**
A new `tmux.Client.PaneTitles()` runs `tmux list-panes -a -F '#{pane_id}\t#{pane_title}'` once
per refresh and returns a `map[paneID]title`. `Agents()` looks each agent's pane up in that
map. *Why over per-pane `display-message`:* one call for all panes instead of N. *Why over
`capture-pane` scraping:* the title is a deliberate state channel, not incidental layout - the
whole point is to avoid parsing the TUI.

**3. Title→level matcher: Braille range for working, exact `✳` for idle, in one place.**

```
first glyph in U+2800..U+28FF  → working   (spinner animates across the range; match the range)
first glyph == U+2733 (✳)      → idle
anything else / empty          → no signal (ok=false → fall back to hook status)
```

Kept beside `idleNotification` as a documented, versioned matcher, since the glyphs are a
Claude-version detail and this is the one spot to update if they change.

Measured behavior (live probe): while working, the title cycles Braille frames (`⠐` ↔ `⠂`,
both in-range) at ~1 Hz; idle is a static `✳`. So the working glyph *changes over time* -
which makes range-matching mandatory (a single-frame match would flicker working → no-signal
every second) and confirms no debounce is needed: the signal never leaves the Braille class
while working and never momentarily blanks. We therefore match the glyph *class*, not a frame,
and do **not** infer working from "the title changed" (coarser and timing-sensitive; reserved
for the activity-diff fallback).

**4. Reconciliation precedence (the core function).**
Given hook status `H` (from the record) and title level `T` (working / idle / none):

| `H` | `T = working` | `T = idle` | `T = none` |
|---|---|---|---|
| needs-attention | working (latch cleared) | needs-attention (held) | needs-attention (held) |
| working | working | **idle** | working |
| idle | working | idle | idle |
| unknown | working | idle | unknown |
| done | done | done | done |

needs-attention is a hook-set **latch**: it holds until the agent is observed to move - the
title showing working (the user clearly answered) or a later clearing hook. The bolded cell is
the headline bug fix (stale "working" after an interrupt reads idle from the title).

**5. Per-agent level-detector seam.**
A minimal interface so the reconciler does not bake in "title":

```go
// levelDetector reports an agent's live working/idle level from current terminal
// state, independent of hooks. ok=false means "no signal; use the hook status".
type levelDetector interface {
    Level(paneID string, titles map[string]string) (Status, bool)
}
```

Claude's title detector is the only implementation now. Title-less agents (opencode, Gemini)
get future detectors - preferred order **activity-diff** (agentapi-style: pane content changing
vs quiescent → working vs idle; needs cross-refresh snapshot state and debounce) then
buffer-regex as last resort. The reconciler and view never learn which source answered.

**6. Inject the title fetcher for tests.**
`ClaudeSource` gains a `titles func() (map[string]string, error)` field defaulting to the tmux
call, mirroring the existing overridable `alive func(pid int) bool`. Reconciliation is then a
pure function of (record, title map) and unit-testable with no tmux.

## Risks / Trade-offs

- **Glyph drift across Claude versions** → single documented matcher (Decision 3); a wrong/absent
  glyph degrades to hook fallback, never to a crash.
- **Title overridden by a shell/wrapper** (precmd setting the title) → `T = none` → hook
  fallback. Low risk while Claude is the foreground TUI controlling its own title.
- **needs-attention residual**: Esc-dismissing a permission leaves no clearing hook; if the
  resulting idle title is indistinguishable from a still-blocked one, the latch shows stale
  attention until the next hook. → Accepted; self-heals on next event. Optional later close: a
  single buffer check for the permission box, used only for this case.
- **Extra tmux call per refresh** → one batched `list-panes`; negligible next to the existing
  per-record `ps` liveness checks.
- **Latch could clear too eagerly** if a blocked agent ever shows a spinner title → needs the
  blocked-title observation below; until known, only `T = working` clears the latch (a spinner
  means active work, the safe signal).

## Migration Plan

Purely additive; no data migration. Existing records stay valid (the level signal is an
overlay). Steps: add `PaneTitles()` to tmux.Client; add the title detector + matcher; thread
the injected fetcher and reconciliation into `Agents()`; update docs that assert hook-only
status (`agent.go` package header, `agent-view` spec, DESIGN.md / CLAUDE.md, README status
table). Rollback = revert the code; records remain readable by the old path.

## Open Questions

- **What does Claude's title show while blocked on a permission prompt** - spinner, sparkle, or a
  distinct string? We could not observe a blocked agent during the probe. If it is distinct, we
  could detect needs-attention from the title too and lean less on the latch (and close the Esc
  residual). This is the key empirical unknown to resolve before finalizing the latch rules.
- ~~**Debounce for the title?**~~ Resolved by the live probe (see Decision 3): the working
  glyph cycles within the Braille range at ~1 Hz and never blanks, so the glyph-class match
  needs no debounce. (Debounce is still needed for the future activity-diff detector.)
- **Lift the title's task summary as the row label?** Out of scope here, but the title already
  carries a far better label than `session:basename`; worth a follow-on change.
