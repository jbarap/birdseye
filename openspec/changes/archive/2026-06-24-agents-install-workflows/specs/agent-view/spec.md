## MODIFIED Requirements

### Requirement: Safe hook install and uninstall

The system SHALL provide commands to install and uninstall the Claude Code hooks into the
user's settings file, preserving all unrelated configuration and hooks, supporting a
user-supplied settings path, and operating idempotently. Uninstall SHALL remove only the
entries the tool added. These hook operations SHALL be reached through the hooks tier of
`be agents install <type>` / `be agents uninstall <type>` (with `be hook claude
install`/`uninstall` retained as hidden, deprecated aliases); the install/uninstall
behavior and its safety guarantees SHALL be unchanged by that reachability change.

#### Scenario: Install preserves existing config

- **WHEN** the user installs hooks into a settings file that already contains other keys
  and unrelated hooks
- **THEN** the tool adds its hook entries while leaving all other keys and hooks intact,
  and records a backup of the previous file

#### Scenario: Install reached through `be agents install`

- **WHEN** the user installs the hooks via `be agents install claude --hooks`
- **THEN** the same hook entries are installed with the same safety guarantees as the
  deprecated `be hook claude install`

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
