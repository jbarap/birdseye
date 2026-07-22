## MODIFIED Requirements

### Requirement: Directory / zoxide provider

The system SHALL provide a built-in provider that offers directories (from `zoxide` and/or
configured roots) as create candidates that start a new session rooted in the chosen
directory. The session name SHALL be derived from the directory as `<base>-<hash>`, where
`base` is the sanitized directory basename and `hash` is a short digest of the directory's
cleaned absolute path. Because the name incorporates the full path, two distinct directories
sharing a basename SHALL produce distinct session names rather than colliding, and names that
differ only in characters the sanitizer folds together (e.g. `.`, `:`, spaces) SHALL likewise
remain distinct. The name derivation SHALL be deterministic - a directory always resolves to
the same session name - so it does not depend on the live session or candidate set.

#### Scenario: Lists candidate directories

- **WHEN** `zoxide` is available or directory roots are configured
- **THEN** the provider returns create candidates for those directories

#### Scenario: Creating a session from a directory

- **WHEN** the user selects a directory candidate
- **THEN** the system creates a tmux session whose working directory is that directory and
  whose name is `<base>-<hash>` derived from the directory's basename and path

#### Scenario: Distinct directories sharing a basename do not collide

- **WHEN** two configured or zoxide-known directories share a basename but live at different
  paths (e.g. `~/a/v3` and `~/b/v3`)
- **THEN** each yields a candidate with a distinct session name, and both remain selectable in
  the picker (neither is dropped by name-based deduplication)

#### Scenario: zoxide absent and no roots configured

- **WHEN** neither `zoxide` nor configured directory roots are available
- **THEN** the provider contributes no candidates without erroring
