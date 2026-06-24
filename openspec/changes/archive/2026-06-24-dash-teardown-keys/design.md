## Context

Teardown in the dash is one action, `dd` (the `delete_agent` chord), gated behind a
confirmation whose *text* decides what happens: on a clean worktree it removes the git
worktree on confirm; on a dirty one it escalates to a second force confirmation. The
confirmation is rendered as an inline `(y/n)` status line. Two problems:

1. **Safety lives in the dialog, not the key.** Users blind-confirm a popup they have seen
   before, so reading the verb in the dialog is not real protection. The outcome should be
   obvious from the key pressed.
2. **The chord resolver only supports same-rune doubles.** `internal/agents/keymap.go`'s
   `doubleChord` matches a two-letter string of one repeated rune (`dd`, `gg`); `m.pending`
   in `view.go` is a single `rune`, and completion requires the second rune to equal the
   armed one. A chord of two *different* keys (`dD`) is unrepresentable.

This change splits teardown by intent into two keys, makes only the destructive one
confirm, and generalizes the chord machinery so `dD` is bindable. It depends on
`worktree-sibling-layout` (so a closed window leaves a git-listed worktree that renders as
a slot) and shares the close/delete *operations* defined by `agents-headless-cli`.

## Goals / Non-Goals

**Goals:**

- `dd` = **close**: close the row's tmux window only, instantly, with no confirmation and
  no filesystem effect.
- `dD` = **delete**: close the window *and* remove the git worktree, behind a centered
  popup confirmation, with the dirty-force choice folded into that popup.
- Generalize the chord resolver from same-rune doubles to arbitrary two-key sequences.
- Keep the dash's `close`/`delete` identical in effect to `be agents close`/`delete`.

**Non-Goals:**

- The headless `be agents close`/`delete` verbs themselves (defined in
  `agents-headless-cli`); this change wires the *keys* to the shared operations.
- The slot/anchor model (defined in `worktree-sibling-layout`); this change consumes it.
- New navigation motions or any rebinding mechanics beyond two-key chord support.

## Decisions

### Safety is a property of the key, not the dialog

`dd` is always safe (window only) and `dD` is always destructive (worktree too). The user
chooses the outcome by which key they press, before any dialog.

- *Why:* a confirmation the user has dismissed once stops being read; encoding safety in
  the binding means the safe action cannot accidentally do the destructive thing
  regardless of confirmation fatigue.
- *Alternative considered (rejected):* keep one key and make the confirmation clearer.
  Rejected — this was the original design; the failure mode is behavioral, not wording.

### Only `dD` confirms; `dd` is instant

`dd` closes the window with no prompt. Closing a window is non-destructive under the new
layout: the worktree persists and the row drops to a slot, so the action is effectively
reversible (re-open a window in the worktree). `dD` removes a worktree from disk, which is
not reversible, so it keeps a confirmation.

- *Why:* confirmations should be reserved for irreversible effects; gating the reversible
  one trains exactly the blind-confirm habit this change removes.

### Generalize the resolver to two-key sequences

Replace the same-rune chord representation with a general two-key chord:

```
resolver:
  single  map[string]Action          // unchanged
  chords  map[[2]string]Action        // (firstKey, secondKey) -> action
  prefix  map[string]bool             // firstKeys that begin some chord
```

The model's `pending rune` becomes `pending string` (the armed first key; "" = none). On a
keypress: if a chord is armed, look up `(pending, key)`; on hit apply, on miss clear
`pending` and fall through to handle `key` normally. Otherwise, if `key` is a chord prefix,
arm it; else dispatch as a single. The existing load-time conflict checks generalize: a
single-key binding equal to any chord prefix is a conflict.

- *Why:* `dd` and `dD` share the prefix `d` but differ on the second key, which the
  rune-equality model cannot express. A `[2]string` chord keyed on Bubble Tea key strings
  (`"d"`, `"D"`) handles repeated-rune chords (`gg`) and mixed chords (`dD`) uniformly.
- *Alternative considered:* special-case `dD` alongside the existing same-rune path.
  Rejected — two parallel chord mechanisms is the kind of special-casing the design
  language warns against; one general representation is simpler.

### Config actions: `close` and `delete` replace `delete_agent`

`[agents.keys]` gains `close` (default `dd`) and `delete` (default `dD`); `delete_agent` is
removed. This is a breaking config change.

- *Why:* the action names should match the cross-surface vocabulary (`be agents
  close`/`delete`); keeping `delete_agent` bound to the now-*safe* `dd` would be actively
  misleading.
- *Trade-off:* users with a custom `delete_agent` binding must re-bind. Acceptable given
  the surrounding breaking changes; called out in the proposal and README.

### Confirmation is the shared centered popup; force folds in

`dD` renders the new-agent modal's centered popup, not the inline `(y/n)` line. A clean
worktree confirms-and-removes; a dirty (or otherwise non-removable) worktree presents the
force-remove choice within the same popup flow, defaulting to cancel.

- *Why:* one modal style across new-agent and delete is consistent (DESIGN.md's single
  visual language), and folding force into the popup removes the separate inline second
  step.

### Behavior by row kind

| Row kind | `dd` (close) | `dD` (delete) |
|---|---|---|
| Worktree with window + agent | close window → slot | popup confirm → remove worktree |
| Worktree with window, no agent (slot-with-window) | close window → windowless slot | popup confirm → remove worktree |
| Windowless slot | no-op (no window) | popup confirm → remove worktree |
| Incidental agent (no worktree) | close window | close window (no worktree to remove) |
| Anchor (primary worktree) | close window (last window ends session) | refused (git won't remove a primary worktree) |

- *Why `dD` on an incidental agent = close:* there is no worktree to remove, so the
  destructive key degrades to the safe effect rather than erroring.
- *Why `dD` on a slot is allowed:* removing a stray/leftover worktree from the dash is
  useful, and there is no window to close first.

## Risks / Trade-offs

- **Muscle memory.** [Users who learned old `dd` (confirm-then-remove) now get an instant
  window close on `dd`] → mitigated: the new `dd` is non-destructive (worktree survives as
  a slot), and the help line reflects the new bindings. The destructive effect moved to
  `dD`, which still confirms.
- **Instant `dd` closes a window with no prompt.** [A misfire closes a window] → acceptable
  because it is reversible (the worktree remains; re-open a window), which is the whole
  justification for not prompting.
- **Chord-resolver regression risk.** [Generalizing the resolver could break `gg`] →
  mitigated by keeping `gg` expressed in the same `[2]string` model and covering it in
  resolver unit tests alongside `dd`/`dD`.

## Migration Plan

1. Generalize `keymap.go`: `[2]string` chords, prefix set, generalized conflict checks;
   add `close`/`delete` actions, remove `delete_agent`.
2. Update `view.go` dispatch: `pending string`, two-key completion, fall-through.
3. Split the orchestrator teardown call sites: `dd` → close-window-only; `dD` → the
   existing close+remove path behind the popup; retire the inline `(y/n)` line.
4. Render the `dD` confirmation as the shared centered popup; fold the dirty-force choice
   into it.
5. Update README `[agents.keys]` docs and the delete description; update DESIGN.md if it
   references the teardown keys.

Rollback is reverting the release; no persisted state.

## Open Questions

- Should `dD` on a clean worktree confirm at all, or remove immediately like `dd` closes?
  Leaning confirm (disk removal is irreversible), but a "clean = no prompt" variant is
  defensible. Decide during apply.
- Exact popup affordance for the dirty case: a second button in the same popup vs a
  re-rendered popup with a force option. Either satisfies "folded in"; pick during apply.
