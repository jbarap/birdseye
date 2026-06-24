## MODIFIED Requirements

### Requirement: Popup-friendly invocation

The system SHALL run the agents view as the `be dash` command — a self-contained command
suitable for launching in a transient context such as a tmux popup — SHALL allow
navigating to a selected agent's session (landing on the exact window and pane the agent
runs in), and SHALL keep the view's rendered state current by refreshing in place as agent
state changes while the view is open, without the user re-running the command. The view's
interactive actions (jump, spawn, close, delete) SHALL resolve to the **same** operation
layer the headless `be agents` verbs expose, so the dash and the verbs stay in lifecycle
parity.

#### Scenario: Runs in a popup context

- **WHEN** the agents view is launched via `tmux display-popup -E be dash`
- **THEN** the view renders within the popup and exits cleanly when dismissed

#### Scenario: Jump to selected agent's exact pane

- **WHEN** the user selects an agent in the view
- **THEN** the system switches/attaches to that agent's tmux session and focuses the
  agent's window and pane, so the user lands where the agent runs even in a split window

#### Scenario: Jump falls back when the pane is gone

- **WHEN** the selected agent's recorded pane no longer exists
- **THEN** the system still connects to the agent's session rather than failing

#### Scenario: Dash actions share the headless operation layer

- **WHEN** the user performs an interactive action in `be dash` (jump, spawn, close, or
  delete)
- **THEN** it resolves to the same operation the corresponding `be agents` verb invokes,
  so both surfaces behave identically

#### Scenario: View refreshes as state changes

- **WHEN** an agent's hook writes new state (or a new agent appears, or one becomes stale)
  while the agents view is open
- **THEN** the view re-renders to reflect the current state without the user re-running
  `be dash`

#### Scenario: Selection is preserved across refreshes

- **WHEN** the list re-renders or re-sorts because state changed
- **THEN** the cursor stays on the same agent where it still exists, and otherwise moves to
  the nearest valid row rather than resetting to the top or pointing past the end
