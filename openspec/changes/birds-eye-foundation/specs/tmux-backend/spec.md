## ADDED Requirements

### Requirement: Idempotent session creation

The system SHALL create a tmux session for a given name and working directory only if a
session with that name does not already exist, reusing the existing session otherwise.

#### Scenario: Create when absent

- **WHEN** the backend is asked to ensure a session named `X` and no such session exists
- **THEN** it creates a new detached session `X` with the requested working directory

#### Scenario: Reuse when present

- **WHEN** the backend is asked to ensure a session named `X` and session `X` already exists
- **THEN** it does not create a duplicate and targets the existing session

### Requirement: Context-aware attach vs switch

The system SHALL connect the user to a target session using `attach-session` when invoked
from outside tmux and `switch-client` when invoked from within an existing tmux client.

#### Scenario: Attach from outside tmux

- **WHEN** the user is not inside a tmux client and selects a session
- **THEN** the backend attaches to that session

#### Scenario: Switch from inside tmux

- **WHEN** the user is already inside a tmux client and selects a session
- **THEN** the backend switches the current client to that session rather than nesting

### Requirement: tmux availability and errors

The system SHALL detect whether the `tmux` binary is available and SHALL surface tmux
command failures with actionable messages.

#### Scenario: tmux not installed

- **WHEN** a session operation is requested and `tmux` is not on PATH
- **THEN** the system reports that tmux is required and exits non-zero

#### Scenario: tmux command fails

- **WHEN** an underlying tmux command exits non-zero
- **THEN** the backend returns an error including the tmux stderr for diagnosis
