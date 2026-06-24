## 1. Generalize the chord resolver (`internal/agents/keymap.go`)

- [x] 1.1 Replace the same-rune chord representation with two-key chords: `chords map[[2]string]Action` plus a `prefix` set of first-keys.
- [x] 1.2 Generalize `buildResolver` conflict checks: a single-key binding equal to any chord prefix is a conflict; duplicate chord/single bindings still error.
- [x] 1.3 Replace `ActionDeleteAgent` with `ActionClose` (config key `close`, default `dd`) and `ActionDelete` (config key `delete`, default `dD`); update `AllActions` and `DefaultKeymap`.
- [x] 1.4 Update/extend `doubleChord`/`singleRune` helpers (or replace) to parse two-key chords from config strings like `"dd"`, `"dD"`, `"gg"`.

## 2. Dispatch (`internal/agents/view.go`)

- [x] 2.1 Change `m.pending` from `rune` to `string` (armed first key; "" = none).
- [x] 2.2 Rewrite the chord completion path: look up `(pending, key)` in `chords`; on miss clear `pending` and fall through to handle `key` normally.
- [x] 2.3 Arm `pending` when a pressed key is in the chord `prefix` set.

## 3. Split teardown operations (`internal/agents/actions.go`)

- [x] 3.1 Wire `ActionClose` to a close-window-only path (no worktree removal, no confirmation), leaving managed worktrees as slots; no-op on a windowless slot.
- [x] 3.2 Wire `ActionDelete` to the close-window-and-remove-worktree path; on a windowless slot remove the worktree with no window to close; on an incidental agent close the window only.
- [x] 3.3 Refuse `ActionDelete` on the anchor (primary worktree); confirm `ActionClose` on the anchor closes its window.

## 4. Confirmation popup (`internal/agents/view.go`, `internal/agents/actions.go`)

- [x] 4.1 Render the delete confirmation as the shared centered popup (consistent with the new-agent modal); retire the inline `(y/n)` status line.
- [x] 4.2 Fold the dirty-force choice into the same popup flow, defaulting to cancel; remove the separate `handleConfirmForceKey` inline step or repoint it at the popup.

## 5. Docs & guard

- [x] 5.1 README `[agents.keys]`: replace `delete_agent` with `close` (`dd`) and `delete` (`dD`); update the teardown description (close = safe window-only, delete = worktree removal behind a popup) and the chord note (chords are now any two-key sequence).
- [x] 5.2 DESIGN.md: update any teardown-key references to the dd/dD split.

## 6. Verification

- [x] 6.1 Add resolver unit tests covering `gg`, `dd`, `dD` (shared prefix, distinct actions), incomplete-chord fall-through, and the single-vs-prefix conflict error.
- [x] 6.2 `just check` (go vet + go test); confirm the theme guard test still passes.
- [x] 6.3 Manual smoke in `be dash`: `dd` closes a worktree window to a slot instantly; `dD` removes a worktree behind the popup; dirty worktree shows the folded force choice; anchor refuses `dD`.
