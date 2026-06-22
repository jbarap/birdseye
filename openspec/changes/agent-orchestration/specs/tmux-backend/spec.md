## ADDED Requirements

### Requirement: Enumerate tmux structure with start paths

The tmux backend SHALL enumerate the live tmux structure — sessions, their windows, and
their panes — reporting for each pane at least its session name, window index, window
name, pane id, and the pane's **start path**. This enumeration is the cheap per-tick
input the agents view reconciles into its Workspace snapshot. When no tmux server is
running the enumeration SHALL return empty rather than erroring.

#### Scenario: Structure includes pane start paths

- **WHEN** the backend enumerates the tmux structure
- **THEN** each pane entry carries its session, window index, window name, pane id, and
  start path

#### Scenario: No server enumerates empty

- **WHEN** no tmux server is running
- **THEN** the enumeration returns an empty result without error

### Requirement: Window and session lifecycle operations

The tmux backend SHALL provide operations to materialize and tear down orchestration
windows: create a new window in a session rooted at a given directory and running a
given command, kill a specific window, and kill a session. These wrap the tmux CLI
behind the same injectable runner as the rest of the backend so they stay testable, and
SHALL report tmux's error when an operation fails.

#### Scenario: Create a window rooted at a directory

- **WHEN** the backend is asked to create a window in a session at a directory with a
  command
- **THEN** it creates a window whose working directory is that directory and starts the
  command in it

#### Scenario: Kill a window

- **WHEN** the backend is asked to kill a specific window
- **THEN** that window is removed from its session

#### Scenario: Kill a session

- **WHEN** the backend is asked to kill a session
- **THEN** that session is removed

#### Scenario: Lifecycle failure reports tmux error

- **WHEN** a lifecycle operation fails (for example targeting a window that no longer
  exists)
- **THEN** the backend returns an error carrying tmux's diagnostic rather than failing
  silently
