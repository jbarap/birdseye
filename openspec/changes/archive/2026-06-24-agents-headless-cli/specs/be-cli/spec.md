## MODIFIED Requirements

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
