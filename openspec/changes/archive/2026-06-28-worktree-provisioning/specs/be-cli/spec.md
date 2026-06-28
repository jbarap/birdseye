## MODIFIED Requirements

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
