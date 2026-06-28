## MODIFIED Requirements

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
