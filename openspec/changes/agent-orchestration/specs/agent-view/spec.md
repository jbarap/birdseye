## ADDED Requirements

### Requirement: Managed-repo recognition

The agents view SHALL recognize managed repositories from a Workspace snapshot
re-derived on every refresh, never persisted. The snapshot SHALL be built from cheap
inputs: the live tmux structure (sessions, windows, and panes with each pane's
**start path**), the existing agent hook records, and per-repo git facts (default
branch, worktree set, dirtiness) read through a cache. Classification SHALL use the
pane **start path**, not its current directory, so navigating inside a window never
changes recognition. A session SHALL be recognized as a managed repo iff at least one
of its windows started in a `<repo>/<default-branch>` directory; the lowest-index such
window SHALL be the canonical **anchor**, and a second such window SHALL NOT
de-recognize the session. Windows whose start path is under the same `<repo>/`
container (but is not the anchor) SHALL be that repo's **managed worktrees**. The
per-tick path classification SHALL be cheap (a cache lookup); git SHALL be consulted
only on a cache miss or when performing an action, never per tick for every window.

#### Scenario: Session with a main anchor is recognized

- **WHEN** a tmux session has a window whose start path is `<repo>/<default-branch>`
- **THEN** the view recognizes that session as a managed repo anchored on that window

#### Scenario: No anchor means not managed

- **WHEN** a session has windows under a `<repo>/` container but none started in the
  repo's default-branch directory
- **THEN** the session is not recognized as managed and renders exactly as a plain
  session

#### Scenario: A second main window does not de-manage

- **WHEN** a recognized managed session gains a second window also started in
  `<repo>/<default-branch>`
- **THEN** the session stays managed and the lowest-index anchor remains canonical

#### Scenario: Recognition uses start path, not current directory

- **WHEN** a window of a managed worktree is `cd`-ed elsewhere
- **THEN** recognition is unchanged because it is based on the pane's start path

#### Scenario: Recognition is re-derived, not stored

- **WHEN** the view refreshes
- **THEN** the managed/worktree/anchor classification is recomputed from the current
  tmux + git state rather than read from any persisted orchestration state

### Requirement: Managed-repo presentation

The agents view SHALL render a managed repository's session bar with a managed
indicator combining the worktree glyph and the worktree count (so the indicator does
not rely on color alone), extending the session-bar rule so a managed repo shows a bar
even when it has no live agents. Beneath it the view SHALL render an `⌂ base` anchor
row for the default-branch checkout and one **worktree row** per managed-worktree
window, reusing the existing fixed-column grid with no new indent level. A worktree
row's pinned status gutter SHALL show its agent's status when a worktree has a live
agent, and an `◌ slot` indicator (glyph plus short label) when it has none, so an
empty worktree is visibly a spawn target without relying on color. Agent-ness and
worktree-ness SHALL be independent: an incidental window in a managed session that has
a live agent but is not a worktree SHALL render as an ordinary agent row, unchanged
from a plain session. New status-gutter and indicator glyphs (`◌ slot`, `⌂ base`)
SHALL be defined in the shared theme package.

#### Scenario: Managed session bar carries an indicator

- **WHEN** a managed repo renders
- **THEN** its session bar shows the worktree glyph and the worktree count, and the bar
  appears even if the repo currently has no live agents

#### Scenario: Anchor row

- **WHEN** a managed repo's default-branch checkout has no agent
- **THEN** the view renders an `⌂ base` anchor row for it

#### Scenario: Worktree with an agent shows agent status

- **WHEN** a managed worktree window has a live agent
- **THEN** its row's status gutter shows that agent's status at the pinned column

#### Scenario: Worktree without an agent shows a slot

- **WHEN** a managed worktree window has no live agent
- **THEN** its row's status gutter shows `◌ slot` as a spawn target, distinguishable
  without color

#### Scenario: Incidental agent in a managed session is unchanged

- **WHEN** a managed session contains a window with a live agent that is not a worktree
  of the repo
- **THEN** that window renders as an ordinary agent row, the same as in a plain session

### Requirement: Create a new agent in a managed repo

The agents view SHALL provide a rebindable action (default `n`) that, when the cursor
is in a managed repo, creates a new agent: it SHALL prompt for a worktree name, create
that worktree via the worktree primitive, open a tmux window rooted in the new worktree
directory, and start the configured agent command in it. The new worktree SHALL appear
as a worktree row on the next refresh. The action SHALL be available only for managed
repos.

#### Scenario: New agent creates worktree, window, and agent

- **WHEN** the user invokes the new-agent action within a managed repo and supplies a
  worktree name
- **THEN** the system creates the worktree, opens a tmux window rooted there, starts the
  configured agent command, and the worktree appears as a row on refresh

#### Scenario: New agent unavailable outside a managed repo

- **WHEN** the cursor is in a plain (non-managed) session
- **THEN** the new-agent action does not attempt to create a worktree

### Requirement: Remove an agent and its worktree

The agents view SHALL provide a rebindable action (default `d`) to remove the agent
under the cursor. For a managed worktree row, removal SHALL kill the worktree's tmux
window and remove the git worktree, guarded by a clean/dirty check: a clean worktree is
removed directly, while a dirty (or otherwise non-removable) worktree SHALL surface an
in-view confirmation offering to force-remove or cancel, defaulting to cancel. For an
incidental agent row (an agent that is not a managed worktree), removal SHALL only
kill/forget the agent and SHALL NOT remove any worktree. The anchor SHALL NOT be
removable through this action.

#### Scenario: Delete a clean worktree

- **WHEN** the user invokes delete on a managed worktree whose working tree is clean
- **THEN** the system kills its window and removes the git worktree without prompting

#### Scenario: Delete a dirty worktree prompts

- **WHEN** the user invokes delete on a managed worktree with uncommitted changes
- **THEN** the view shows a confirmation to force-remove or cancel, and cancelling
  leaves the worktree and its window intact

#### Scenario: Delete an incidental agent removes no worktree

- **WHEN** the user invokes delete on an agent row that is not a managed worktree
- **THEN** only that agent/window is removed and no `git worktree remove` is performed

#### Scenario: Anchor is not deletable

- **WHEN** the cursor is on the `⌂ base` anchor row and the user invokes delete
- **THEN** the system does not remove the anchor or the default-branch checkout

### Requirement: Configurable refresh interval and agent command

The agents view's live refresh interval SHALL be configurable (`[agents] refresh`,
default one second) rather than hardcoded, and the command used to spawn an agent for
the new-agent action SHALL be configurable (`[agents] command`, default `claude`) so the
orchestration layer is not welded to a single agent type. Invalid values SHALL be
reported as a configuration error rather than silently ignored.

#### Scenario: Refresh interval honored

- **WHEN** `[agents] refresh` is set to a valid duration
- **THEN** the view refreshes at that interval instead of the default

#### Scenario: Agent command honored

- **WHEN** `[agents] command` is set and the user creates a new agent
- **THEN** the configured command is started in the new worktree's window

#### Scenario: Invalid value reported

- **WHEN** `[agents] refresh` or `[agents] command` is malformed
- **THEN** configuration loading fails with a clear error rather than silently using a
  default

## MODIFIED Requirements

### Requirement: Needs-attention ordering across groups

The agents view SHALL order sessions by their most-urgent member so that triage stays fast: the
session containing the highest-priority agent SHALL appear first, using the status ranking that
surfaces needs-attention before working, idle, done, and unknown. Within a plain session, agents
SHALL be ordered by that same status ranking (then by name) regardless of which window each
occupies; agents that share a window SHALL NOT be forced to be adjacent. Within a **managed
repo**, the `⌂ base` anchor row SHALL be pinned first as the repo's head, the repo's worktree rows
SHALL follow ordered by status rank (then by name) so a blocked worktree still floats up among its
siblings, and worktree rows in the `◌ slot` state (no agent) SHALL sort last. Ungrouped agents
SHALL sort among the sessions by their own most-urgent member.

#### Scenario: Most-urgent group floats to top

- **WHEN** one session has an agent that needs attention and another session's agents are all idle
  or done
- **THEN** the needs-attention agent's session is rendered above the other session

#### Scenario: Ordering within a session

- **WHEN** a session contains agents in differing statuses across one or more windows
- **THEN** the agents are ordered by status rank, needs-attention first, regardless of window, and
  agents sharing a window are not specially grouped together

#### Scenario: Anchor pinned and slots last in a managed repo

- **WHEN** a managed repo renders with an anchor, worktrees with agents in differing statuses, and
  empty worktree slots
- **THEN** the `⌂ base` anchor row appears first, worktree rows follow by status rank (most-urgent
  first), and `◌ slot` rows appear last
