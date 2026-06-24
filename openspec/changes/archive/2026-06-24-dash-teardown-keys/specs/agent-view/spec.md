## ADDED Requirements

### Requirement: Two-key chord bindings

The agents view's keymap SHALL support chords that are arbitrary **two-key sequences**,
not only a single rune pressed twice, so that a chord like `dd` (a repeated key) and a
chord like `dD` (two different keys) are both bindable. The resolver SHALL arm on the
first key of any bound chord and resolve the action on the second key; if the second key
does not complete a chord with the armed first key, the armed state SHALL clear and that
second key SHALL be handled as its own binding. A single-key binding that is also the
first key of a chord SHALL be reported as a configuration conflict at load time.

#### Scenario: Repeated-key chord resolves

- **WHEN** the user presses the two keys of a repeated-key chord such as `gg`
- **THEN** the resolver triggers that chord's action

#### Scenario: Mixed-key chord resolves

- **WHEN** the user presses the two keys of a mixed chord such as `dD` (`d` then
  shift-`d`)
- **THEN** the resolver triggers that chord's action, distinct from the `dd` chord that
  shares the first key

#### Scenario: Incomplete chord falls through

- **WHEN** the user presses a key that begins a chord and then a key that does not
  complete any chord with it
- **THEN** the armed state clears and the second key is handled as its own binding rather
  than being swallowed

#### Scenario: Single key conflicting with a chord prefix is rejected

- **WHEN** configuration binds a single key that is also the first key of a bound chord
- **THEN** loading configuration fails with a clear error naming the conflict

### Requirement: Close a window

The agents view SHALL provide a rebindable **close** action (default the `dd` chord, config
key `close`) that closes the tmux window of the row under the cursor and has **no
filesystem effect**. Close SHALL be **instant** — it SHALL NOT show a confirmation —
because under the git-native layout the worktree persists when its window closes, so the
action is reversible. Closing a managed worktree's window SHALL leave the worktree on disk,
which then renders as a slot. Closing an incidental agent's window SHALL just close it.
Closing the anchor's window SHALL close that window, and if it is the session's last window
the tmux session ends. On a windowless slot (no window to close) the action SHALL be a
no-op. The close action SHALL be identical in effect to `be agents close`.

#### Scenario: Close a worktree window leaves a slot

- **WHEN** the user presses the close key on a managed worktree row with an open window
- **THEN** the window is closed, the git worktree remains on disk, and the row renders as a
  slot on the next refresh

#### Scenario: Close is instant with no confirmation

- **WHEN** the user presses the close key on any closable row
- **THEN** the window is closed immediately with no confirmation dialog

#### Scenario: Close an incidental agent

- **WHEN** the user presses the close key on an incidental agent row (no worktree)
- **THEN** only that window is closed and no worktree is touched

#### Scenario: Close the anchor window

- **WHEN** the user presses the close key on the `⌂ base` anchor row
- **THEN** the anchor's window is closed (ending the session if it was the last window),
  and no worktree is removed

#### Scenario: Close a windowless slot is a no-op

- **WHEN** the user presses the close key on a windowless slot row
- **THEN** nothing is closed and no worktree is touched

## MODIFIED Requirements

### Requirement: Remove an agent and its worktree

The agents view SHALL provide a rebindable **delete** action (default the `dD` chord,
config key `delete`) that closes the row's tmux window **and** removes its git worktree.
Delete SHALL be shown behind a confirmation rendered as a centered popup consistent with
the new-agent modal — not an inline `(y/n)` status line — so an accidental keypress is
never destructive. On confirm, a clean worktree SHALL be removed; a dirty (or otherwise
non-removable) worktree SHALL present a force-remove choice folded into the same popup
flow, defaulting to cancel, rather than discarding changes silently. On a windowless slot,
delete SHALL remove the worktree (there is no window to close). On an incidental agent row
(no worktree), delete SHALL only close the window, since there is no worktree to remove.
The anchor SHALL NOT be deletable: because it is the repository's primary worktree, git
refuses to remove it, so delete on the anchor is structurally refused. The delete action
SHALL be identical in effect to `be agents delete`.

#### Scenario: Delete asks before acting via a popup

- **WHEN** the user invokes delete on a removable worktree row
- **THEN** the view shows a centered popup confirmation and removes nothing until the user
  confirms

#### Scenario: Delete a clean worktree

- **WHEN** the user confirms delete on a managed worktree whose working tree is clean
- **THEN** the system closes its window and removes the git worktree

#### Scenario: Delete a dirty worktree folds force into the popup

- **WHEN** the user confirms delete on a managed worktree with uncommitted changes
- **THEN** the popup presents a force-remove choice defaulting to cancel, and cancelling
  leaves the worktree and its window intact

#### Scenario: Delete a windowless slot removes the worktree

- **WHEN** the user invokes delete on a windowless slot and confirms
- **THEN** the system removes that git worktree, with no window to close

#### Scenario: Delete an incidental agent removes no worktree

- **WHEN** the user invokes delete on an agent row that is not a managed worktree
- **THEN** only that window is closed and no `git worktree remove` is performed

#### Scenario: Anchor is not deletable

- **WHEN** the cursor is on the `⌂ base` anchor row and the user invokes delete
- **THEN** the system refuses, because git will not remove the repository's primary
  worktree
