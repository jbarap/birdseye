# session-providers Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.
## Requirements
### Requirement: Provider interface

The system SHALL define a provider interface whereby each provider can enumerate session
candidates, and each candidate exposes a stable name, a display label, a type, and an
action (attach to an existing session or create a new one).

#### Scenario: Provider enumerates candidates

- **WHEN** the registry asks an enabled provider for candidates
- **THEN** the provider returns zero or more candidates, each with a name, label, type,
  and an attach-or-create action

#### Scenario: Candidate name uniqueness for resolution

- **WHEN** two providers produce candidates that would map to the same tmux session name
- **THEN** the system resolves them deterministically so selecting either yields one
  session rather than duplicate sessions

### Requirement: Running tmux sessions provider

The system SHALL provide a built-in provider that lists currently running tmux sessions
as existing-session (attach) candidates.

#### Scenario: Lists active sessions

- **WHEN** tmux is running with one or more sessions
- **THEN** the provider returns one attach candidate per running session

#### Scenario: tmux not running

- **WHEN** no tmux server is running
- **THEN** the provider returns no candidates without erroring

### Requirement: tmuxp templates provider

The system SHALL provide a built-in provider that lists `tmuxp` templates as create
candidates that, when selected, load the template into a new session.

#### Scenario: Lists available templates

- **WHEN** `tmuxp` is installed and templates exist
- **THEN** the provider returns one create candidate per template

#### Scenario: Selecting a template creates its session

- **WHEN** the user selects a tmuxp template candidate
- **THEN** the system loads that template (e.g. via `tmuxp load`) to create the session

#### Scenario: Connects to the actually-created session

- **WHEN** a loaded template's session name differs from the template file name
- **THEN** the system detects the session that was created and connects to it rather than
  assuming the name matches the template file

#### Scenario: tmuxp absent

- **WHEN** the `tmuxp` binary is not installed
- **THEN** the provider is skipped with a non-fatal warning and contributes no candidates

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

### Requirement: Headless candidate enumeration

The system SHALL expose the provider registry's session candidates non-interactively via
`be sessions --json`, serializing the same candidates the interactive fzf picker draws —
running sessions, templates, directory roots, and repositories — over the **same** registry,
with no duplication of provider logic. Each emitted candidate SHALL carry the fields a
client needs to act on it, including its name, label, type, kind (attach vs create), and
the directory it would be rooted in (where applicable), so the output can feed
`be agents spawn`.

#### Scenario: JSON enumeration mirrors the picker

- **WHEN** a client runs `be sessions --json`
- **THEN** the system prints a JSON array of the candidates the interactive picker would
  show, drawn from the same enabled providers

#### Scenario: Enumeration feeds spawn

- **WHEN** a client takes a directory-bearing candidate from `be sessions --json` and
  passes its directory to `be agents spawn`
- **THEN** the spawn operates on that directory, with discovery and spawn composing over
  one registry

#### Scenario: Disabled providers are absent from JSON

- **WHEN** a provider is disabled in config
- **THEN** its candidates do not appear in `be sessions --json`, matching the interactive
  picker

