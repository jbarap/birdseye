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

The system SHALL expose a command surface that requires an explicit subcommand: the bare
`be` invocation (no arguments) SHALL NOT launch any view and SHALL instead report that a
subcommand is required, naming the available commands. The TUI SHALL be reached as
`be dash` (the live agents view, which degrades gracefully without tmux). `be agents`
SHALL be the headless verb namespace (its subcommands defined by the agents-cli
capability), no longer an alias for the TUI. The fuzzy session picker SHALL remain
`be sessions`, and `be list` SHALL remain a hidden, deprecated alias for `be sessions` so
existing invocations keep working.

#### Scenario: Bare invocation requires a subcommand

- **WHEN** the user runs `be` with no arguments
- **THEN** the system prints that a subcommand is required, lists the available commands
  (including `be dash`), and exits non-zero without launching a view

#### Scenario: TUI is reached via `be dash`

- **WHEN** the user runs `be dash`
- **THEN** the system opens the live agents view (the former bare-`be` TUI)

#### Scenario: `be agents` is the headless namespace

- **WHEN** the user runs `be agents` with a verb such as `be agents list`
- **THEN** the system dispatches to the headless agent verb rather than opening the TUI

#### Scenario: Picker is reached via `be sessions`

- **WHEN** the user runs `be sessions`
- **THEN** the system opens the fuzzy session picker

#### Scenario: Deprecated picker alias still works

- **WHEN** the user runs `be list`
- **THEN** the system opens the session picker (the alias is accepted though hidden from
  help)

#### Scenario: Named subcommands are routed

- **WHEN** the user runs a known subcommand such as `be dash` or `be worktree`
- **THEN** the system dispatches to that subcommand's handler

#### Scenario: Unknown subcommand

- **WHEN** the user runs an unrecognized subcommand
- **THEN** the system prints an error naming the unknown command and lists available
  commands, exiting with a non-zero status

### Requirement: Configuration loading

The system SHALL load configuration from a user config file (under the platform config
directory, e.g. `~/.config/birdseye/`), applying built-in defaults when the file or
individual keys are absent, and SHALL fail with a clear message on malformed config. The
system SHALL also support a **repo-local** `.birdseye/config.toml` that uses the **same schema
as the user config** and, for repository-scoped operations, is layered on top of the user
config to produce the effective configuration: a key the repo-local config sets overrides the
user-level value, and any key it omits is inherited. Layering SHALL follow these merge
semantics - a scalar the project sets overrides, a table is merged key-by-key, and a list is
replaced wholesale - so a project can override any user-level setting (for example the worktree
`setup` command or the `[agents]` command) while inheriting the rest. The repo-local config
SHALL be discovered relative to the repository the operation targets (resolved from git, not
the process working directory alone), so it applies regardless of which worktree the command is
run from. Overlaying SHALL NOT mutate the in-memory user configuration. A malformed repo-local
config SHALL be rejected with the same clear, file-identifying error as a malformed user
config. Process-wide, multi-repository surfaces (the dash, the picker) have no single
repository in scope and SHALL use the user configuration directly.

#### Scenario: Defaults when no config present

- **WHEN** no config file exists
- **THEN** the system runs with built-in defaults and all built-in providers enabled

#### Scenario: User config overrides defaults

- **WHEN** a config file enables/disables providers or sets per-provider options
- **THEN** the system honors those settings, overriding defaults

#### Scenario: Repo-local config overrides the user value and inherits the rest

- **WHEN** a repository's `.birdseye/config.toml` sets a `[worktree] setup` and an `[agents]`
  command while leaving other keys unset, and a repository-scoped operation runs against it
- **THEN** the effective configuration uses the repo-local `setup` and agent command and
  inherits every key the repo-local config did not set from the user config

#### Scenario: Overlaying does not mutate the user config

- **WHEN** the effective configuration is computed for one repository
- **THEN** the in-memory user configuration is unchanged, so a later operation on a different
  repository sees the unaltered user values

#### Scenario: Malformed config is rejected

- **WHEN** the user config file or a repo-local `.birdseye/config.toml` contains invalid syntax
  or unknown required fields
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

