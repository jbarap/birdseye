# be-cli Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.

## Requirements

### Requirement: Single self-contained binary

The system SHALL be distributed as a single, statically-linkable Go binary named `be`
that runs without a runtime interpreter or external language dependencies.

#### Scenario: Binary runs standalone

- **WHEN** the `be` binary is copied to a machine with no Go toolchain installed
- **THEN** invoking `be` executes successfully using only the OS and optional runtime
  tools (tmux, git) that it probes for at runtime

#### Scenario: Version reporting

- **WHEN** the user runs `be --version`
- **THEN** the system prints the build version and exits with status 0

### Requirement: Command surface

The system SHALL expose a primary command surface whose default command (`be` with no
args) is the agents view, with the fuzzy session picker available as `be sessions`. The
agents view is the default because it degrades gracefully without tmux (plain triage
rows), whereas the picker requires tmux. `be agents` SHALL remain a named alias for the
default, and `be list` SHALL remain a hidden, deprecated alias for `be sessions` so
existing invocations keep working.

#### Scenario: Default command opens the agents view

- **WHEN** the user runs `be` with no arguments
- **THEN** the system opens the agents view (equivalent to `be agents`)

#### Scenario: Picker is reached via `be sessions`

- **WHEN** the user runs `be sessions`
- **THEN** the system opens the fuzzy session picker

#### Scenario: Deprecated picker alias still works

- **WHEN** the user runs `be list`
- **THEN** the system opens the session picker (the alias is accepted though hidden from
  help)

#### Scenario: Named subcommands are routed

- **WHEN** the user runs a known subcommand such as `be agents` or `be worktree`
- **THEN** the system dispatches to that subcommand's handler

#### Scenario: Unknown subcommand

- **WHEN** the user runs an unrecognized subcommand
- **THEN** the system prints an error naming the unknown command and lists available
  commands, exiting with a non-zero status

### Requirement: Configuration loading

The system SHALL load configuration from a user config file (under the platform config
directory, e.g. `~/.config/birds-eye/`), applying built-in defaults when the file or
individual keys are absent, and SHALL fail with a clear message on malformed config.

#### Scenario: Defaults when no config present

- **WHEN** no config file exists
- **THEN** the system runs with built-in defaults and all built-in providers enabled

#### Scenario: User config overrides defaults

- **WHEN** a config file enables/disables providers or sets per-provider options
- **THEN** the system honors those settings, overriding defaults

#### Scenario: Malformed config is rejected

- **WHEN** the config file contains invalid syntax or unknown required fields
- **THEN** the system prints a clear error identifying the file and problem, and exits
  non-zero without partially applying the config

### Requirement: Provider registry and extension model

The system SHALL maintain a registry of session providers, enable or disable them based
on configuration, and present a stable extension point so new providers can be added
without modifying picker, tmux, or CLI core code.

#### Scenario: Registry aggregates enabled providers

- **WHEN** the picker requests session candidates
- **THEN** the registry queries every enabled provider and aggregates their candidates

#### Scenario: Disabled provider is skipped

- **WHEN** a provider is disabled in config
- **THEN** the registry does not query it and none of its candidates appear

#### Scenario: Provider failure is isolated

- **WHEN** one provider returns an error or its required external tool is missing
- **THEN** the registry skips that provider, surfaces a non-fatal warning, and still
  returns candidates from the remaining providers
