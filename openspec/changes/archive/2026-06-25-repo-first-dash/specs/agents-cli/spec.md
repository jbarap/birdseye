## MODIFIED Requirements

### Requirement: Directory-based spawn

The system SHALL provide `be agents spawn <dir>` that takes any directory on disk, resolves it to its
repository, ensures-or-reuses that repository's **deterministic `be-<repo>` home session** (named
from the repository's identity, collision-safe), creates a sibling worktree (for a `--branch`,
applying the grouped-sibling layout), opens a tmux window in that home session rooted in the new
worktree, and starts the configured agent command - optionally seeding it with `--prompt`. Spawn
SHALL route the new window into the repository's `be-` home even when the repository's existing
presence is in a user-made session, rather than injecting into that user session. `spawn` SHALL be
independently usable without any pre-existing tmux state, and SHALL print the new worktree's handle.
If the repository's home session already exists it SHALL be reused, not duplicated.

#### Scenario: Spawn from a bare directory

- **WHEN** a client runs `be agents spawn <dir> --branch <b>` and no tmux session exists for that
  repository
- **THEN** the system creates the repository's `be-` home session, adds a sibling worktree for `<b>`,
  opens a window rooted there, starts the agent, and prints the new handle

#### Scenario: Spawn reuses an existing home session

- **WHEN** the repository for `<dir>` already has its `be-` home session
- **THEN** `spawn` adds the worktree and window into that existing home session rather than creating
  a duplicate

#### Scenario: Spawn routes to the be- home rather than a user session

- **WHEN** the repository's only existing presence is a pane or agent in a user-made session, and a
  client spawns into that repository
- **THEN** the new window is created in the repository's `be-` home session, not injected into the
  user's session

#### Scenario: Spawn seeds an initial prompt

- **WHEN** a client runs `be agents spawn <dir> --branch <b> --prompt <p>`
- **THEN** the started agent receives `<p>` as its initial input

### Requirement: Data verbs emit a stable JSON contract

The data-returning verbs (`list`, `status`) SHALL emit machine-readable JSON when given `--json`, and
a human-readable table otherwise. Each record SHALL carry a stable, additive-only field set including
the work handle, repository, worktree, branch, path, **agent working directory**, tmux session (id
and name), window, pane, status, and kind (base / worktree / slot / agent), where `base` denotes a
repository's primary worktree. Existing field names SHALL remain stable across releases; new fields
MAY be added.

#### Scenario: list emits JSON records

- **WHEN** a client runs `be agents list --json`
- **THEN** the system prints a JSON array of records, each with the documented stable fields,
  suitable for another program to parse

#### Scenario: Records expose the agent working directory

- **WHEN** a record describes a live agent
- **THEN** its JSON carries the agent's working directory field

#### Scenario: Primary worktree reports kind base

- **WHEN** a record describes a repository's primary worktree
- **THEN** its `kind` field is `base`

#### Scenario: status reports one handle

- **WHEN** a client runs `be agents status <handle> --json`
- **THEN** the system prints the record for that handle, including its current status
