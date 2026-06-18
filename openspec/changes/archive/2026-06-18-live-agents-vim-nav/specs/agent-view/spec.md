## MODIFIED Requirements

### Requirement: Popup-friendly invocation

The system SHALL run `be agents` as a self-contained command suitable for launching in a
transient context such as a tmux popup, SHALL allow navigating to a selected agent's
session — landing on the exact window and pane the agent runs in — and SHALL keep the
view's rendered state current by refreshing in place as agent state changes while the view
is open, without the user re-running the command.

#### Scenario: Runs in a popup context

- **WHEN** `be agents` is launched via `tmux display-popup -E be agents`
- **THEN** the view renders within the popup and exits cleanly when dismissed

#### Scenario: Jump to selected agent's exact pane

- **WHEN** the user selects an agent in the view
- **THEN** the system switches/attaches to that agent's tmux session and focuses the agent's
  window and pane, so the user lands where the agent runs even in a split window

#### Scenario: Jump falls back when the pane is gone

- **WHEN** the selected agent's recorded pane no longer exists
- **THEN** the system still connects to the agent's session rather than failing

#### Scenario: View refreshes as state changes

- **WHEN** an agent's hook writes new state (or a new agent appears, or one becomes stale)
  while the agents view is open
- **THEN** the view re-renders to reflect the current state without the user re-running
  `be agents`

#### Scenario: Selection is preserved across refreshes

- **WHEN** the list re-renders or re-sorts because state changed
- **THEN** the cursor stays on the same agent where it still exists, and otherwise moves to
  the nearest valid row rather than resetting to the top or pointing past the end

## ADDED Requirements

### Requirement: Configurable, vim-native navigation

The agents view SHALL provide vim-native navigation motions — at minimum move up, move
down, jump to top, jump to bottom, and half-page up/down — in addition to the arrow keys,
and SHALL allow the user to rebind every navigation and command action through configuration.
Built-in defaults SHALL preserve the existing bindings so that absent configuration changes
no behavior. The view's help line SHALL reflect the active bindings.

#### Scenario: Vim motions navigate the list

- **WHEN** the user presses the bound keys for top, bottom, or half-page movement (by
  default `gg`/`G` and `ctrl+d`/`ctrl+u`)
- **THEN** the cursor jumps to the first row, the last row, or moves by half a page
  respectively, clamped to the list bounds

#### Scenario: Default bindings preserve existing behavior

- **WHEN** no navigation keys are configured
- **THEN** `j`/`k` and the arrow keys move the cursor, `enter` jumps to the selected agent,
  and `q`/`esc`/`ctrl+c` quit, exactly as before this change

#### Scenario: User rebinds an action

- **WHEN** the user configures custom keys for a navigation or command action
- **THEN** those keys drive that action and the previously default keys for it no longer do,
  unless they are also listed

#### Scenario: Help reflects active bindings

- **WHEN** the view renders its help line with custom bindings configured
- **THEN** the help line shows the configured keys for each action rather than a fixed
  hard-coded set

#### Scenario: Invalid keymap configuration is reported

- **WHEN** the keymap configuration is malformed (for example an unknown action or an empty
  key list for an action)
- **THEN** loading configuration fails with a clear error naming the problem rather than
  silently dropping the binding

### Requirement: Live preview of the selected agent's session

The agents view SHALL show a preview of the recent terminal output of the selected agent's
tmux session, captured from that session's pane, so the user can see what an agent is doing
without switching to it. The preview SHALL follow the cursor and refresh in step with the
live list refresh. The preview SHALL be a convenience view only and SHALL NOT change how an
agent's status is determined: status remains derived from hook-written state, never from the
captured pane output. When pane output cannot be captured (no such session, not running
under tmux, or capture fails), the view SHALL show an explicit placeholder rather than
failing.

#### Scenario: Preview shows the selected agent's pane

- **WHEN** an agent is selected in the view and its tmux session has pane output
- **THEN** the view shows a preview of that session's recent terminal output

#### Scenario: Preview follows the cursor

- **WHEN** the user moves the cursor to a different agent
- **THEN** the preview updates to show that agent's session output

#### Scenario: Preview refreshes with the list

- **WHEN** the live refresh occurs while an agent is selected
- **THEN** the preview re-renders with that session's current output

#### Scenario: Preview unavailable

- **WHEN** the selected agent's pane output cannot be captured
- **THEN** the view shows an explicit placeholder for the preview and continues to function

#### Scenario: Preview does not drive status

- **WHEN** the preview shows pane output for an agent
- **THEN** the agent's displayed status is still the hook-derived status and is not inferred
  from the captured output

#### Scenario: Preview targets the agent's exact pane

- **WHEN** the agent runs in a specific pane of a split window
- **THEN** the preview captures that pane (by its recorded pane id), not merely the window's
  currently-active pane

### Requirement: Refined visual presentation

The agents view SHALL present its content with a refined, readable layout — at minimum a
titled frame, status badges that are visually distinguishable per status, aligned columns,
and a clear indication of the selected row. The presentation SHALL degrade gracefully on
narrow terminals and color-limited terminals, and SHALL not depend on a fixed terminal size.

#### Scenario: Readable framed layout

- **WHEN** the agents view renders with agents present
- **THEN** it shows a titled frame with aligned columns, per-status badges, and a clearly
  marked selected row

#### Scenario: Adapts to terminal size

- **WHEN** the view is rendered in a small popup or its size changes while open
- **THEN** it adjusts its layout to fit (for example hiding or stacking the preview) rather
  than overflowing or breaking the frame

#### Scenario: Degrades on limited color

- **WHEN** the terminal supports limited or no color
- **THEN** statuses and the selected row remain distinguishable without relying solely on
  color

### Requirement: One row per tmux location

The agents view SHALL show at most one row per tmux location (pane, or window for records
without a pane id), so that multiple agent records mapping to the same location — for
example restarting Claude in the same pane — collapse to a single, most-recent entry.
Agents occupying genuinely distinct panes SHALL each remain their own row.

#### Scenario: Restarted agent does not duplicate

- **WHEN** several agent records share the same tmux pane (such as successive Claude sessions
  in one pane)
- **THEN** the view shows one row for that location, reflecting the most recently updated
  record

#### Scenario: Distinct panes stay separate

- **WHEN** two agents run in two different panes
- **THEN** the view shows both as separate rows

### Requirement: Garbage collection of aged-out state

The system SHALL remove persisted agent state once it has aged out, so the list and the
on-disk state do not grow without bound. Ended (done) sessions SHALL be removed after a
configurable retention window, and non-terminal sessions with no fresh updates (crashed or
abandoned) after a separate configurable window. A retention window of zero SHALL disable
that pruning. Pruning SHALL be safe to perform during normal use.

#### Scenario: Ended sessions are pruned

- **WHEN** a done session's state is older than the done-retention window
- **THEN** its state is deleted and it no longer appears in the view

#### Scenario: Crashed sessions are reclaimed

- **WHEN** a non-terminal session has had no updates for longer than the stale-retention
  window
- **THEN** its state is deleted and it no longer appears in the view

#### Scenario: Recent sessions are kept

- **WHEN** a session's state is within its retention window
- **THEN** the session remains in the view and its state is not deleted
