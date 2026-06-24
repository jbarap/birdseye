## Why

Teardown in the dash is gated behind a confirmation whose *text* decides what happens, and the
anchor cannot be removed at all — leaving a stuck session once its agents are gone. Users blind-
confirm popups after the first time, so safety should live in the *keybinding*, not the dialog: the
outcome should be obvious from the key the user pressed.

## What Changes

- `dd` = **close** (safe, tmux-only): close the row's window. The worktree stays on disk and drops to
  a slot. No filesystem effect.
- `dD` = **delete** (destructive, disk): close the window *and* `git worktree remove` the worktree; a
  dirty worktree escalates to a force confirmation.
- `dd` is instant (window-only, disk-safe). Only `dD` shows a confirmation, rendered as a centered
  popup consistent with the new-agent modal, with the dirty-force step folded into it. The inline
  `(y/n)` status line is retired in favor of the modal.
- Generalize the chord resolver from same-rune doubles (`dd`, `gg`) to arbitrary two-key sequences,
  so `dD` (two different keys) is bindable.
- `close`/`delete` mean exactly what `be agents close`/`delete` mean (cross-surface parity).
- On the anchor: `dd` closes its window (closing the session's last window ends the session); `dD` is
  structurally refused because git will not remove a primary worktree.
- **BREAKING (config):** the `[agents.keys]` action `delete_agent` is replaced by two actions,
  `close` (default `dd`) and `delete` (default `dD`); existing `delete_agent` bindings stop being
  recognized.

## Capabilities

### New Capabilities

- (none — this change modifies an existing capability)

### Modified Capabilities

- `agent-view`: `dd`/`dD` teardown semantics; instant action vs popup confirmation; two-key chord
  resolution; anchor teardown behavior.

## Impact

- `internal/agents/keymap.go`: new delete-worktree action; generalize the resolver to two-key
  sequences.
- `internal/agents/actions.go`, `internal/agents/view.go`: split close vs delete paths; render the
  `dD` confirmation as the shared popup; drop the inline confirm line.
- **Depends on** `worktree-sibling-layout` so a closed window leaves a git-listed worktree that
  renders as a visible slot.
