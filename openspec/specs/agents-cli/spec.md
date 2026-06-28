# agents-cli Specification

## Purpose
TBD - created by archiving change agents-headless-cli. Update Purpose after archive.
## Requirements
### Requirement: Headless agent verbs

The system SHALL expose agent lifecycle operations as headless `be agents` subcommands —
`list`, `status`, `spawn`, `send`, `jump`, `close`, and `delete` — usable by any client
that can run a shell, with no interactive UI required. These verbs and the `be dash` TUI
SHALL resolve to the **same** underlying operations, so an operation performed through a
verb is identical to the corresponding action performed in the dash (lifecycle parity).

#### Scenario: Verbs are available headlessly

- **WHEN** a script or another process runs `be agents <verb> …` in a non-interactive
  shell
- **THEN** the verb executes and returns its result without launching the TUI

#### Scenario: Verb and dash operations are the same

- **WHEN** the same lifecycle operation is invoked through a `be agents` verb and through
  the corresponding `be dash` action
- **THEN** both go through one shared operation layer and produce the same effect

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

### Requirement: Directory-based spawn

The system SHALL provide `be agents spawn <dir>` that takes any directory on disk, resolves it to its
repository, ensures-or-reuses that repository's **deterministic `be-<repo>` home session** (named
from the repository's identity, collision-safe), creates a sibling worktree (for a `--branch`,
applying the grouped-sibling layout), opens a tmux window in that home session rooted in the new
worktree, and starts the configured agent command - optionally seeding it with `--prompt`. Creating
the worktree SHALL provision it per the `worktree-provider` capability (carry `.worktreeinclude`
files, then the configured setup command). Unlike the agentless `be worktree add` path, spawn SHALL
NOT run the setup command inline; instead it SHALL **chain the resolved setup command into the
agent's window ahead of the agent**, so the window runs setup and then the agent (`<setup> &&
<agent>`). This keeps the spawn call from blocking on a long setup, makes setup output visible in the
agent's own pane, and leaves a failed setup at a shell rather than starting the agent in an
unprepared worktree. When no setup command is configured the window starts the agent directly. Spawn
SHALL route the new window into the repository's `be-` home even when the repository's existing
presence is in a user-made session, rather than injecting into that user session. `spawn` SHALL be
independently usable without any pre-existing tmux state, and SHALL print the new worktree's handle.
If the repository's home session already exists it SHALL be reused, not duplicated.

#### Scenario: Spawn from a bare directory

- **WHEN** a client runs `be agents spawn <dir> --branch <b>` and no tmux session exists for that
  repository
- **THEN** the system creates the repository's `be-` home session, adds a sibling worktree for `<b>`,
  opens a window rooted there, starts the agent, and prints the new handle

#### Scenario: Spawn provisions the worktree and chains setup into the window

- **WHEN** a client spawns into a repository whose `.birdseye/config.toml` defines a setup command
- **THEN** the new worktree receives its `.worktreeinclude` files and the window runs the setup
  command before the agent, so the agent starts only after setup succeeds

#### Scenario: A failed setup stops before the agent starts

- **WHEN** the chained setup command exits non-zero in the spawned window
- **THEN** the window stops at a shell showing the setup output and the agent command is not run

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

### Requirement: jump and best-effort send

The system SHALL provide `be agents jump <handle>` that connects the caller's tmux client
to the work's session/window/pane, and `be agents send <handle> <input>` that dispatches
input to the work's agent via tmux. `send` SHALL be **best-effort**: because tmux
`send-keys` has no readiness signal, the verb SHALL report what it dispatched, not confirm
receipt, and clients needing confirmation SHALL poll `status`.

#### Scenario: jump connects to the work's pane

- **WHEN** a user runs `be agents jump <handle>` from inside tmux
- **THEN** the tmux client switches to that work's session/window/pane

#### Scenario: send dispatches input best-effort

- **WHEN** a client runs `be agents send <handle> <input>`
- **THEN** the system dispatches `<input>` to the work's agent and reports it as sent,
  without guaranteeing the agent was ready to receive it

