## MODIFIED Requirements

### Requirement: Managed-repo recognition

The agents view SHALL recognize managed repositories from a Workspace snapshot
re-derived on every refresh, never persisted. The snapshot SHALL be built from cheap
inputs: the live tmux structure (sessions, windows, and panes with each pane's **start
path**), the existing agent hook records, and per-repo git facts read through a cache —
the repository's shared git directory (`git rev-parse --git-common-dir`), its **worktree
set** (`git worktree list`), its default branch, and dirtiness. Recognition SHALL be
**git-native**: a window's start path is classified by resolving it to a git worktree and
its repository, not by matching a `<repo>/<default-branch>` path shape, so any on-disk
layout is recognized — including worktrees a user or another tool created. A session
SHALL be recognized as a managed repo iff at least one of its windows started inside a
git worktree of a repository. A repository's **anchor** SHALL be its git **primary
worktree** (the worktree git refuses to remove), independent of which branch is checked
out there; the window started in that primary worktree SHALL be the canonical anchor
window. Windows started in other worktrees of the same repository (same
`git-common-dir`) SHALL be that repo's **managed worktrees**. The per-tick path
classification SHALL be cheap (a cache lookup); git SHALL be consulted only on a cache
miss or when performing an action, never per tick for every window.

#### Scenario: Session with a primary-worktree window is recognized

- **WHEN** a tmux session has a window whose start path is a repository's git primary
  worktree
- **THEN** the view recognizes that session as a managed repo anchored on that window

#### Scenario: Anchor is the primary worktree regardless of branch

- **WHEN** a repository's primary worktree has a non-default branch checked out
- **THEN** the view still treats that primary worktree's window as the anchor, because
  the anchor is defined by git's primary worktree, not by the branch name

#### Scenario: No worktree window means not managed

- **WHEN** a session has no window started inside any git worktree of a recognized
  repository
- **THEN** the session is not recognized as managed and renders exactly as a plain
  session

#### Scenario: Recognition uses start path, not current directory

- **WHEN** a window of a managed worktree is `cd`-ed elsewhere
- **THEN** recognition is unchanged because it is based on the pane's start path

#### Scenario: Recognition is re-derived, not stored

- **WHEN** the view refreshes
- **THEN** the managed/worktree/anchor classification is recomputed from the current
  tmux + git state rather than read from any persisted orchestration state

#### Scenario: Third-party worktree is recognized

- **WHEN** a window started in a worktree created by a tool other than `be worktree add`,
  belonging to a recognized repository
- **THEN** the view classifies it as a managed worktree of that repository, because
  recognition reads the git worktree set rather than a bird's-eye path convention

### Requirement: Managed-repo presentation

The agents view SHALL render a managed repository's session bar with a managed indicator
combining the worktree glyph and the worktree count (so the indicator does not rely on
color alone), extending the session-bar rule so a managed repo shows a bar even when it
has no live agents. Beneath it the view SHALL render an `⌂ base` anchor row for the
repository's primary worktree and one **worktree row** per worktree in the repository's
git worktree set. Worktree rows SHALL be enumerated from the **git worktree set**, not
only from open tmux windows: a worktree with an open window renders against that window,
and a worktree with **no open tmux window** SHALL still render as a row showing an
`◌ slot` indicator (a spawn target). A worktree row's pinned status gutter SHALL show its
agent's status when the worktree's window has a live agent, and an `◌ slot` indicator
(glyph plus short label) when it has no agent or no window, so an empty or windowless
worktree is visibly a spawn target without relying on color. Agent-ness and worktree-ness
SHALL be independent: an incidental window in a managed session that has a live agent but
is not a worktree SHALL render as an ordinary agent row, unchanged from a plain session.
The `◌ slot` and `⌂ base` glyphs SHALL be defined in the shared theme package.

#### Scenario: Managed session bar carries an indicator

- **WHEN** a managed repo renders
- **THEN** its session bar shows the worktree glyph and the worktree count, and the bar
  appears even if the repo currently has no live agents

#### Scenario: Anchor row reflects the primary worktree

- **WHEN** a managed repo's primary worktree has no agent
- **THEN** the view renders an `⌂ base` anchor row for it

#### Scenario: Worktree with an agent shows agent status

- **WHEN** a managed worktree's window has a live agent
- **THEN** its row's status gutter shows that agent's status at the pinned column

#### Scenario: Worktree without an agent shows a slot

- **WHEN** a managed worktree has an open window but no live agent
- **THEN** its row's status gutter shows `◌ slot` as a spawn target, distinguishable
  without color

#### Scenario: Windowless worktree surfaces as a slot

- **WHEN** a repository's git worktree set contains a worktree with no open tmux window
- **THEN** the view still renders a row for it showing `◌ slot`, so the worktree is a
  visible spawn target rather than vanishing from the view

#### Scenario: Incidental agent in a managed session is unchanged

- **WHEN** a managed session contains a window with a live agent that is not a worktree
  of the repo
- **THEN** that window renders as an ordinary agent row, the same as in a plain session
