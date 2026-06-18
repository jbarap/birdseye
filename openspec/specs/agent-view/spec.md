# agent-view Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.

## Requirements

### Requirement: Agent abstraction

The system SHALL define an `Agent` abstraction that yields known agents, each with a tmux
session/window location and a status, so multiple agent types can be supported behind one
interface. The first implementation SHALL be Claude Code.

#### Scenario: Agents surfaced through the abstraction

- **WHEN** the user runs `be agents`
- **THEN** the view lists agents obtained through the `Agent` abstraction, each with its
  tmux session/window location and status

#### Scenario: No agents present

- **WHEN** no agents are known
- **THEN** the view shows an informative empty state rather than failing

### Requirement: Claude Code status via hooks

The Claude Code implementation SHALL obtain status from Claude Code hooks rather than from
scraping panes: hooks write per-session state to a known location, and `be agents` reads
that state. The system SHALL provide a documented hook configuration to wire this up, and
SHALL render missing or stale state as an explicit unknown status rather than guessing.

#### Scenario: Status reflects hook-written state

- **WHEN** a Claude Code session's hooks have written its current state
- **THEN** `be agents` renders that session's status from the written state

#### Scenario: Hooks not configured or state stale

- **WHEN** no hook state exists for a detected session, or the state is stale
- **THEN** the view shows an explicit unknown status instead of inferring working/done

#### Scenario: Hook configuration is provided

- **WHEN** the user follows the documented setup to install the hook configuration
- **THEN** subsequent Claude Code sessions report status to `be agents` without further
  per-session setup

### Requirement: Safe hook install and uninstall

The system SHALL provide commands to install and uninstall the Claude Code hooks into the
user's settings file, preserving all unrelated configuration and hooks, supporting a
user-supplied settings path, and operating idempotently. Uninstall SHALL remove only the
entries the tool added.

#### Scenario: Install preserves existing config

- **WHEN** the user installs hooks into a settings file that already contains other keys
  and unrelated hooks
- **THEN** the tool adds its hook entries while leaving all other keys and hooks intact,
  and records a backup of the previous file

#### Scenario: Custom settings path

- **WHEN** the user supplies a non-standard settings path
- **THEN** the tool reads and writes that path instead of the default

#### Scenario: Idempotent install

- **WHEN** the user installs hooks that are already present and unchanged
- **THEN** the tool reports no change and does not duplicate entries

#### Scenario: Uninstall removes only its own hooks

- **WHEN** the user uninstalls hooks
- **THEN** only the tool's own hook entries are removed and all other configuration and
  hooks remain

#### Scenario: Malformed settings refused

- **WHEN** the settings file is not valid JSON
- **THEN** the tool refuses to modify it and reports a clear error

### Requirement: Essential status reporting

The system SHALL report an essential status for each agent from a small, well-defined set
— at minimum: needs-attention, working, idle, and done — so the user can triage which
sessions require action.

#### Scenario: Status is shown per agent

- **WHEN** the agents view renders
- **THEN** each agent row shows one status from the defined set

#### Scenario: Needs-attention surfaced first

- **WHEN** any agent is in the needs-attention state
- **THEN** the view orders or visually distinguishes those agents so they stand out

### Requirement: Popup-friendly invocation

The system SHALL run `be agents` as a self-contained command suitable for launching in a
transient context such as a tmux popup, and SHALL allow navigating to a selected agent's
session.

#### Scenario: Runs in a popup context

- **WHEN** `be agents` is launched via `tmux display-popup -E be agents`
- **THEN** the view renders within the popup and exits cleanly when dismissed

#### Scenario: Jump to selected agent

- **WHEN** the user selects an agent in the view
- **THEN** the system switches/attaches to that agent's tmux session via the tmux backend

### Requirement: Extensible agent implementations

The system SHALL keep the `Agent` abstraction as the extension point so additional agent
types and richer status workflows can be added without changing the view's command
interface.

#### Scenario: Adding a new agent type

- **WHEN** a new `Agent` implementation is added internally
- **THEN** its agents appear in `be agents` without changes to the command's invocation
  or output contract
