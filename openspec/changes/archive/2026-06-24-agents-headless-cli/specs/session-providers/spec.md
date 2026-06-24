## ADDED Requirements

### Requirement: Headless candidate enumeration

The system SHALL expose the provider registry's session candidates non-interactively via
`be sessions --json`, serializing the same candidates the interactive fzf picker draws —
running sessions, templates, directory roots, and worktrees — over the **same** registry,
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
