# tmux-backend Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.

## Requirements

### Requirement: Idempotent session creation

The system SHALL create a tmux session for a given name and working directory only if a session with
that name does not already exist, reusing the existing session otherwise. For be-created (managed)
sessions, the session **name SHALL be a deterministic function of the repository's identity** (its
`git-common-dir`), using the repository basename as the friendly part and **disambiguating on
collision** so that two repositories sharing a basename receive distinct session names. Determinism
SHALL be such that ensuring the session for a given repository always resolves to the same name, so
spawning into a repository's home is reliably idempotent.

#### Scenario: Create when absent

- **WHEN** the backend is asked to ensure a session named `X` and no such session exists
- **THEN** it creates a new detached session `X` with the requested working directory

#### Scenario: Reuse when present

- **WHEN** the backend is asked to ensure a session named `X` and session `X` already exists
- **THEN** it does not create a duplicate and targets the existing session

#### Scenario: Repository session name is deterministic and collision-safe

- **WHEN** two distinct repositories share a basename and each is ensured a home session
- **THEN** each resolves to a distinct, stable session name derived from its repository identity, so
  they do not collide into one shared session

#### Scenario: Ensuring the same repository twice resolves to one session

- **WHEN** the same repository's home session is ensured on two separate occasions
- **THEN** both resolve to the same session name, reusing the existing session on the second

### Requirement: Context-aware attach vs switch

The system SHALL connect the user to a target session using `attach-session` when invoked
from outside tmux and `switch-client` when invoked from within an existing tmux client.

#### Scenario: Attach from outside tmux

- **WHEN** the user is not inside a tmux client and selects a session
- **THEN** the backend attaches to that session

#### Scenario: Switch from inside tmux

- **WHEN** the user is already inside a tmux client and selects a session
- **THEN** the backend switches the current client to that session rather than nesting

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

### Requirement: tmux availability and errors

The system SHALL detect whether the `tmux` binary is available and SHALL surface tmux
command failures with actionable messages.

#### Scenario: tmux not installed

- **WHEN** a session operation is requested and `tmux` is not on PATH
- **THEN** the system reports that tmux is required and exits non-zero

#### Scenario: tmux command fails

- **WHEN** an underlying tmux command exits non-zero
- **THEN** the backend returns an error including the tmux stderr for diagnosis
