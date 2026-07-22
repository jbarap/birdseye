## MODIFIED Requirements

### Requirement: Managed-repo recognition

The agents view SHALL recognize repositories from a Workspace snapshot re-derived on every refresh,
never persisted, and SHALL group the view by **repository** rather than by tmux session. The snapshot
SHALL be built from cheap inputs: the live tmux structure (sessions, windows, and panes with each
pane's **start path**), the existing agent hook records (each carrying its **working directory**),
and per-repo git facts read through a cache - the repository's shared git directory
(`git rev-parse --git-common-dir`), its **worktree set** (`git worktree list`), its default branch,
and dirtiness. Recognition SHALL be **git-native**: a path is classified by resolving it to a git
worktree and its repository, not by matching a path shape, so any on-disk layout is recognized.

A repository SHALL be recognized (and rendered as a section) when **any** pane's start path **or**
**any** active agent's working directory resolves to a worktree of that repository (the same
`git-common-dir`). There SHALL be no whole-session purity gate and no `managed` boolean: a stray
non-git pane, or panes spanning multiple repositories within one tmux session, SHALL NOT suppress any
repository's section; each repository is recognized independently from its own anchoring presence. A
repository's **anchor** SHALL be its git **primary worktree** (the worktree git refuses to remove),
independent of which branch is checked out there. A tmux session SHALL NOT itself be a unit of
**repository** recognition; it is a location a row resolves to. The per-tick path classification SHALL
be cheap (a cache lookup); git SHALL be consulted only on a cache miss or when performing an action.

#### Scenario: A repo is recognized from any anchoring pane

- **WHEN** at least one pane in any tmux session has a start path inside a repository's worktree
- **THEN** the view renders a section for that repository, regardless of what other panes in the
  same session resolve to

#### Scenario: A repo is recognized from an active agent's working directory

- **WHEN** a pane was started outside a repository but an active agent in it reports a working
  directory inside that repository (e.g. a tmuxp auto-`cd` workflow)
- **THEN** the view renders a section for that repository, anchored by the agent's working directory

#### Scenario: A stray pane no longer suppresses a repository

- **WHEN** a tmux session contains panes inside a repository's worktrees and also a pane that is not
  inside any git worktree
- **THEN** the repository is still recognized and rendered, and the non-git pane does not appear as a
  worktree row

#### Scenario: Panes spanning two repositories yield two sections

- **WHEN** a single tmux session has panes resolving to two different repositories
- **THEN** the view renders a section for each repository, with each pane's row under its own
  repository

#### Scenario: Recognition uses start path and agent working directory, not live cwd

- **WHEN** a pane of a recognized worktree is `cd`-ed elsewhere after starting
- **THEN** recognition is unchanged, because it is based on the pane's start path and on active
  agents' recorded working directories, not the pane's live current directory

#### Scenario: Recognition is re-derived, not stored

- **WHEN** the view refreshes
- **THEN** the repository/worktree/anchor classification is recomputed from the current tmux + git +
  agent state rather than read from any persisted orchestration state

#### Scenario: A repository with no live presence is not shown

- **WHEN** a repository has worktrees on disk but no pane and no agent resolves to it
- **THEN** the view shows no section for it, because recognition does not scan the filesystem for
  repositories

## ADDED Requirements

### Requirement: Non-git directory session rows

The agents view SHALL surface an **open** tmux session that resolves to no repository and hosts no
agent as a single jump-only **directory row**. This recognition is a fallback subordinate to
repository and agent recognition: it SHALL apply only to a session none of whose panes resolve to any
repository and none of whose panes host an active agent, so it never competes with or duplicates a
repository section or an agent row. The directory row SHALL carry the session's representative
directory - the start path of its lowest-indexed window's first pane, chosen deterministically - and
SHALL be rendered only while the session is live; a directory with no open session SHALL NOT be
surfaced (dormant directories remain the picker's domain, unlike repositories which render even
without a live window).

A directory row SHALL support jumping to its session and previewing it, and SHALL NOT offer
worktree or agent-spawn actions, because the directory is not a git repository. Attempting a spawn
action on a directory row SHALL report that the location is not a git repository rather than silently
doing nothing.

#### Scenario: An open non-git directory session appears as a jump-only row

- **WHEN** a tmux session is rooted at a directory that resolves to no git repository and hosts no
  active agent
- **THEN** the view renders a single directory row for that session, carrying the directory, with
  jump and preview available and no worktree/spawn actions

#### Scenario: A directory session with a repo pane is not double-surfaced

- **WHEN** a tmux session has at least one pane resolving to a repository (or hosts an active agent)
- **THEN** the session is represented by its repository section (or agent row) and no separate
  directory row is emitted for it

#### Scenario: A closed directory is not surfaced

- **WHEN** a directory has no open tmux session
- **THEN** the view shows no directory row for it, because directory recognition is gated on a live
  session rather than a filesystem scan

#### Scenario: Spawn is rejected on a directory row

- **WHEN** the user invokes the spawn action on a directory row
- **THEN** the view reports that the location is not a git repository and creates no worktree
