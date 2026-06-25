## MODIFIED Requirements

### Requirement: Agent abstraction

The system SHALL define an `Agent` abstraction that yields known agents, each with a tmux
session/window location, a **working directory**, and a status, so multiple agent types can be
supported behind one interface. The working directory SHALL be persisted on the agent record (the
hook already reports it) so the reconciler can resolve it to a repository for recognition and
grouping. The first implementation SHALL be Claude Code.

#### Scenario: Agents surfaced through the abstraction

- **WHEN** the user runs `be agents`
- **THEN** the view lists agents obtained through the `Agent` abstraction, each with its
  tmux session/window location, working directory, and status

#### Scenario: Agent record carries its working directory

- **WHEN** an agent hook record is ingested and persisted
- **THEN** the persisted record retains the agent's working directory, not only a derived title

#### Scenario: No agents present

- **WHEN** no agents are known
- **THEN** the view shows an informative empty state rather than failing

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
recognition; it is a location a row resolves to. The per-tick path classification SHALL be cheap (a
cache lookup); git SHALL be consulted only on a cache miss or when performing an action.

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

### Requirement: Managed-repo presentation

The agents view SHALL render each recognized repository as a section whose bar carries a repository
indicator combining the worktree glyph and the worktree count (so the indicator does not rely on
color alone), and the bar SHALL appear even when the repository has no live agents. Beneath it the
view SHALL render an `⌂ base` anchor row for the repository's primary worktree and one row per
worktree in the repository's git worktree set, enumerated from the **git worktree set** rather than
only from open tmux windows: a worktree with no open tmux window SHALL still render as an `◌ slot`
spawn target.

A worktree MAY render as **multiple rows when it hosts multiple active agents**: the view SHALL show
**one row per agent**, with the worktree label repeating, rather than folding co-located agents into
a single row. A worktree with no agent SHALL render as a single base or slot row. The view SHALL
remain a two-level tree (section -> row); multiple agents in one worktree SHALL NOT introduce a third
indent level. A row whose pane lives outside the repository's `be-` home session SHALL carry an
`[in: <session>]` locator hint naming that session. When a worktree has windows in more than one
session, the base/slot row SHALL resolve to a deterministic pane, preferring an agent-bearing pane
and then the `be-` home session's pane. The `◌ slot` and `⌂ base` glyphs SHALL be defined in the
shared theme package.

#### Scenario: Repository section bar carries an indicator

- **WHEN** a recognized repository renders
- **THEN** its section bar shows the worktree glyph and the worktree count, and the bar appears even
  if the repository currently has no live agents

#### Scenario: Anchor row reflects the primary worktree

- **WHEN** a repository's primary worktree has no agent
- **THEN** the view renders an `⌂ base` anchor row for it

#### Scenario: Worktree with an agent shows agent status

- **WHEN** a worktree's window has a live agent
- **THEN** its row's status gutter shows that agent's status at the pinned column

#### Scenario: Two agents in one worktree show two rows

- **WHEN** a single worktree hosts two active agents
- **THEN** the view shows two rows, each with that worktree's label and its own agent's status,
  without adding a third indent level

#### Scenario: Windowless worktree surfaces as a slot

- **WHEN** a repository's git worktree set contains a worktree with no open tmux window
- **THEN** the view still renders a row for it showing `◌ slot`, so the worktree is a visible spawn
  target

#### Scenario: A row outside the be- home shows a locator hint

- **WHEN** a row's pane lives in a tmux session other than the repository's `be-` home
- **THEN** the row carries an `[in: <session>]` hint naming that session

### Requirement: Create a new agent in a managed repo

The agents view SHALL provide a rebindable action (default `n`) that, when the cursor is in a
recognized repository's section, creates a new agent through a small two-field form: a **branch**
name and a **worktree** directory name, where the worktree field auto-fills from a filesystem-safe
slug of the branch until the user edits it directly. The action SHALL create that worktree on that
branch via the worktree primitive, ensure-or-reuse the repository's deterministic `be-<repo>` home
session, open a tmux window there rooted in the new worktree directory, and start the configured
agent command. When the repository's only current presence is in a user-made (non-`be-`) session,
the action SHALL still route the new window into the repository's `be-` home rather than injecting
into the user's session. The new worktree SHALL appear as a row on the next refresh.

#### Scenario: New agent creates worktree, window, and agent in the be- home

- **WHEN** the user invokes the new-agent action within a recognized repository and supplies a branch
- **THEN** the system creates the worktree, ensures the repository's `be-<repo>` session, opens a
  window rooted in the worktree, starts the configured agent command, and the worktree appears as a
  row on refresh

#### Scenario: Spawn routes to the be- home even when presence is in a user session

- **WHEN** the repository is recognized only from a pane or agent in a user-made session, and the
  user invokes the new-agent action
- **THEN** the system creates or reuses the repository's `be-<repo>` session for the new window
  rather than injecting a window into the user's session

#### Scenario: Worktree name derives from the branch until edited

- **WHEN** the user types a branch name containing characters unsafe for a directory (e.g.
  `feature/login`)
- **THEN** the worktree field shows a slugified form until the user edits it, after which it stops
  tracking the branch

### Requirement: Remove an agent and its worktree

The agents view SHALL provide a rebindable **delete** action (default the `dD` chord, config key
`delete`) that closes the row's tmux window **and** removes its git worktree, behind a confirmation
rendered as a centered popup. Delete SHALL be permitted only when the target is the **sole row of its
worktree**: if the worktree hosts more than one row (e.g. two agents), delete SHALL be refused with a
notice instructing the user to close the other occupants first, so a worktree is never removed out
from under a co-located agent. On confirm for a sole occupant, a clean worktree SHALL be removed; a
dirty (or otherwise non-removable) worktree SHALL present a force-remove choice folded into the same
popup flow, defaulting to cancel. On a windowless slot, delete SHALL remove the worktree. On an
incidental agent row with no worktree, delete SHALL only close the window. The anchor SHALL NOT be
deletable. The delete action SHALL be identical in effect to `be agents delete`.

#### Scenario: Delete is refused when the worktree has other occupants

- **WHEN** the user invokes delete on a row whose worktree also hosts another agent row
- **THEN** the system refuses with a notice and removes nothing, leaving the user to close the other
  occupant first

#### Scenario: Delete a clean sole-occupant worktree

- **WHEN** the user confirms delete on a worktree whose working tree is clean and which has no other
  rows
- **THEN** the system closes its window and removes the git worktree

#### Scenario: Delete a dirty worktree folds force into the popup

- **WHEN** the user confirms delete on a sole-occupant worktree with uncommitted changes
- **THEN** the popup presents a force-remove choice defaulting to cancel, and cancelling leaves the
  worktree and its window intact

#### Scenario: Delete a windowless slot removes the worktree

- **WHEN** the user invokes delete on a windowless slot and confirms
- **THEN** the system removes that git worktree, with no window to close

#### Scenario: Anchor is not deletable

- **WHEN** the cursor is on the `⌂ base` anchor row and the user invokes delete
- **THEN** the system refuses, because git will not remove the repository's primary worktree

## REMOVED Requirements

### Requirement: One row per tmux location

**Reason**: Superseded by the repo-first row model, where row identity is "one row per active agent"
(by pane) and "one row per agentless worktree" (base / slot). Collapsing successive records in the
same pane to the most-recent entry is retained as agent-record behavior, but the row model is no
longer defined by tmux location: two agents in one worktree are two rows, and a worktree with no
agent is a row even with no window.
**Migration**: The de-duplication of successive agent records sharing one pane moves under the
`Agent abstraction` / reconciler (most-recent record per pane). Distinct panes remain distinct rows
as before; agentless worktrees gain rows from the git worktree set per `Managed-repo presentation`.
