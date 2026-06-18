## ADDED Requirements

### Requirement: fzf-backed fuzzy finder

The system SHALL implement the picker by shelling out to `fzf`. `fzf` is a hard
requirement with no built-in fallback; when it is unavailable the picker SHALL report this
and exit non-zero.

#### Scenario: fzf drives the picker

- **WHEN** the picker opens and `fzf` is available
- **THEN** candidates are presented through `fzf` for fuzzy filtering and selection

#### Scenario: fzf missing

- **WHEN** the picker is invoked and `fzf` is not on PATH
- **THEN** the system prints a clear message that `fzf` is required (with an install hint)
  and exits non-zero, taking no session action

### Requirement: Fuzzy-searchable candidate list

The system SHALL present aggregated session candidates in a fuzzy-finder interface that
filters candidates as the user types, supporting keyboard navigation and selection.

#### Scenario: Incremental fuzzy filtering

- **WHEN** the user types characters in the picker
- **THEN** the visible candidate list narrows to fuzzy matches of the typed query

#### Scenario: Empty query shows all candidates

- **WHEN** the picker opens with an empty query
- **THEN** all available candidates are shown, grouped/labeled by type

### Requirement: Candidates grouped and labeled by type

The system SHALL distinguish existing sessions from creatable candidates and label each
candidate with its source type, so the user can tell at a glance whether selecting it
attaches to a running session or creates a new one.

#### Scenario: Existing vs creatable distinction

- **WHEN** the candidate list contains both running tmux sessions and tmuxp templates
- **THEN** each entry is labeled with its type and whether it attaches or creates

#### Scenario: Configurable ordering

- **WHEN** config specifies an ordering or labels for candidate types
- **THEN** the picker presents types in that order with those labels

### Requirement: Selection dispatch

The system SHALL route the selected candidate to its associated action and SHALL exit
cleanly without performing any action when the user cancels.

#### Scenario: Selecting an existing session

- **WHEN** the user selects a candidate representing a running session
- **THEN** the system invokes the tmux backend to attach to or switch to that session

#### Scenario: Selecting a creator candidate

- **WHEN** the user selects a candidate that represents a way to create a session
- **THEN** the system invokes that candidate's create action and then attaches/switches
  to the resulting session

#### Scenario: Cancelling the picker

- **WHEN** the user cancels (e.g. presses Esc or Ctrl-C) without selecting
- **THEN** the system exits with no session created or changed

### Requirement: Empty and no-tty handling

The system SHALL behave predictably when there are no candidates and when standard input
is not an interactive terminal.

#### Scenario: No candidates available

- **WHEN** no provider returns any candidate
- **THEN** the picker shows an informative empty-state message rather than a blank list

#### Scenario: Non-interactive invocation

- **WHEN** the picker is invoked without an interactive TTY
- **THEN** the system prints a clear message that an interactive terminal is required and
  exits non-zero instead of hanging
