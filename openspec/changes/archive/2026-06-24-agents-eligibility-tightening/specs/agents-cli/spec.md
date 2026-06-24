## MODIFIED Requirements

### Requirement: Durable derived addressing

The system SHALL address a unit of work by a **derived** text handle computed on each
invocation from git and tmux, never from persisted birdseye state. A repository's
**primary worktree** (its base) SHALL be addressed by `<repo>` alone, where `<repo>` is
the basename of the primary worktree's directory. A **linked worktree** SHALL be
addressed by `<repo>/<worktree>`, where `<worktree>` is that worktree directory's
basename. A live or observed agent that has **no worktree** SHALL be addressed by its
tmux pane id, which survives session and window renames. Because handles are derived, an
agent the system did not spawn SHALL be addressable automatically, with no import step.
When a handle is ambiguous (two repositories in the active set share a basename), the
verb SHALL fail and list the disambiguating paths rather than act on a guess.

#### Scenario: Primary worktree addressed by repo name

- **WHEN** a client references `<repo>` with no second segment
- **THEN** the system resolves it to that repository's primary worktree (the base)

#### Scenario: Linked worktree addressed by repo/worktree

- **WHEN** a client references `<repo>/<worktree>`
- **THEN** the system resolves it to the linked worktree whose directory basename is
  `<worktree>` in repository `<repo>`

#### Scenario: Handle survives renames and restarts

- **WHEN** a tmux session or window is renamed, or the tmux server restarts, after a
  worktree is created
- **THEN** the worktree's `<repo>/<worktree>` handle still resolves, because it is derived
  from git rather than from tmux names

#### Scenario: Unspawned agent is addressable without import

- **WHEN** a client references an agent the system did not spawn (no associated worktree)
  by its tmux pane id
- **THEN** the system resolves and operates on it, with no prior import action

#### Scenario: Ambiguous handle is refused

- **WHEN** a handle could resolve to two repositories sharing a basename in the active set
- **THEN** the verb exits non-zero and lists the candidate paths instead of acting on
  either

### Requirement: Data verbs emit a stable JSON contract

The data-returning verbs (`list`, `status`) SHALL emit machine-readable JSON when given
`--json`, and a human-readable table otherwise. Each record SHALL carry a stable,
additive-only field set including the work handle, repository, worktree, branch, path,
tmux session (id and name), window, pane, status, and kind (base / worktree / slot /
agent), where `base` denotes a repository's primary worktree. Existing field names SHALL
remain stable across releases; new fields MAY be added.

#### Scenario: list emits JSON records

- **WHEN** a client runs `be agents list --json`
- **THEN** the system prints a JSON array of records, each with the documented stable
  fields, suitable for another program to parse

#### Scenario: Primary worktree reports kind base

- **WHEN** a record describes a repository's primary worktree
- **THEN** its `kind` field is `base`

#### Scenario: Human table without --json

- **WHEN** a user runs `be agents list` without `--json`
- **THEN** the system prints a human-readable table rather than JSON

#### Scenario: status reports one handle

- **WHEN** a client runs `be agents status <handle> --json`
- **THEN** the system prints the record for that handle, including its current status

### Requirement: close and delete with cross-surface parity

The system SHALL provide `be agents close <handle>` and `be agents delete <handle>`.
`close` SHALL close the work's tmux window and leave the worktree on disk (identical to
the dash close action). `delete` SHALL close the window and remove the git worktree
(identical to the dash delete action); a dirty or otherwise non-removable worktree SHALL
NOT be removed unless `--force` is given. A base (primary-worktree) handle SHALL refuse
`delete` because git will not remove a primary worktree.

#### Scenario: close keeps the worktree

- **WHEN** a client runs `be agents close <handle>`
- **THEN** the tmux window is closed and the git worktree remains on disk

#### Scenario: delete removes a clean worktree

- **WHEN** a client runs `be agents delete <handle>` on a clean worktree
- **THEN** the window is closed and the git worktree is removed

#### Scenario: delete refuses a dirty worktree without force

- **WHEN** a client runs `be agents delete <handle>` on a worktree with uncommitted
  changes and no `--force`
- **THEN** the system declines to remove the worktree and exits non-zero, leaving it
  intact

#### Scenario: delete forced on a dirty worktree

- **WHEN** a client runs `be agents delete <handle> --force` on a dirty worktree
- **THEN** the system closes the window and force-removes the worktree

#### Scenario: Base worktree refuses delete

- **WHEN** a client runs `be agents delete <repo>` against a base (primary) worktree
- **THEN** the system refuses and exits non-zero, because git cannot remove a primary
  worktree
