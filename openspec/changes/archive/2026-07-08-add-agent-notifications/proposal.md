## Why

birdseye already knows the moment an agent crosses into a state that wants you: a
permission/input block (`needs-attention`) or the end of a turn (`working -> idle`). But that
knowledge only surfaces inside `be dash`. If the dash is closed - which is the normal case, since
it is a popup you dismiss - a blocked agent sits waiting and you never learn until you look. The
whole premise of birdseye is that you are *not* staring at the agent's pane, so an out-of-band
push is the missing half of the triage story.

The event spine already exists. `be hook claude record` is invoked by Claude Code on every
lifecycle event, and `agents.HandleHook` already reads the previous status, computes the new
status, and resolves the agent's identity, cwd, and tmux location - all in one place, firing
whether or not the dash is running. Emitting a notification is one observed state edge away; no
polling, diffing, or new plumbing is required.

## What Changes

- Emit an **OS notification** from the hook path (`HandleHook`) on two status edges:
  - **needs-attention**: any transition **into** `StatusNeedsAttention`.
  - **finished**: a `working -> idle` transition.
  No notification fires on session start, start-of-work, or session end.
- Because `HandleHook` fires headless, notifications work with the dash closed and over SSH.
  Emission is **best-effort**: a notifier failure never fails the agent's hook chain.
- **Reuse the existing per-location mute** as the suppression control: a muted agent emits no
  notifications. No new "notification mute" concept.
- **One notification per agent per edge**: each agent notifies independently on its own qualifying
  transition. No cross-agent digest and no debounce timer - the hook is edge-driven by discrete
  real events, so each transition already notifies exactly once.
- Add a small **`Notifier` seam**. The default auto-detects the platform notifier (`notify-send`
  on Linux, `osascript` on macOS, terminal bell as a last-resort fallback) and degrades gracefully
  when none is present, like birdseye's other optional tools. A user-set **command template**
  overrides it and receives the notification as environment variables (`BE_AGENT`, `BE_STATUS`,
  `BE_REPO`, `BE_CWD`, `BE_MESSAGE`), so users can route to ntfy/Slack/Pushover without birdseye
  shipping platform code.
- Add an **`[agents.notify]` config table**: `command` (string, empty = auto), `needs_attention`
  (bool, default true), `finished` (bool, default true). It loads under the strict config loader
  and is documented in `default.toml`.

## Capabilities

### Added Capabilities

- `agent-notifications`: birdseye emits an out-of-band OS notification when a tracked agent crosses
  into `needs-attention` or finishes a turn, from the hook path so it works with the dash closed;
  gated by the existing mute and by per-edge config; delivered through an auto-detected platform
  notifier or a user command template.

## Impact

- `internal/agents/claude.go`: in `HandleHook`, capture the old status before `StatusFor`
  overwrites it, classify the edge, consult mute + config, and dispatch to the notifier (all
  best-effort). No change to how statuses are *detected*.
- `internal/agents/` (new file, e.g. `notify.go`): the `Notifier` interface, the auto-detect
  default (notify-send / osascript / bell), and the command-template notifier with its env-var
  contract.
- `internal/config/config.go`: a `Notify` struct nested on `Agents` with `command`,
  `needs_attention`, `finished`; defaults applied so an absent table means both edges on, auto
  delivery.
- `internal/config/default.toml`: document the `[agents.notify]` keys.
- No new CLI verb and no change to `be dash` rendering. `be config` surfaces the new keys for free.

## Non-Goals (deferred to a follow-up)

- **Focus/away suppression**: a dash heartbeat file so `finished` can stay quiet while you are
  actively watching. v1 fires on the edge regardless of whether the dash is open.
- **OSC escape-sequence channels** (OSC 9/777/99). The agent's pane is the one you are not
  watching, so an OS notification is the primary channel; terminal escapes add little here.
- **Digest/coalescing** across agents, and any debounce window.
