## REMOVED Requirements

### Requirement: Needs-attention ordering across groups

**Reason**: The dashboard splits into two lenses (`dash-lenses`). Encoding cross-group
urgency in section position made the section a user was watching jump the instant its agent
changed status. Stable ordering now lives in the Workspaces lens (sections by name; within a
section the `⌂ base` anchor first, worktrees by name, slots last), and cross-group urgency
is expressed in the Agents lens's fixed status bands instead of by moving sections.

**Migration**: Replaced by `dash-lenses` requirements "Stable Workspaces ordering" (stable
section and within-section ordering, status as a per-row indicator) and "Agents lens triage
bands" (urgent agents surface in the always-first `NEEDS YOU` band, recency-ordered within
each band).

## MODIFIED Requirements

### Requirement: Managed-repo presentation

The agents view SHALL render each recognized repository as a section whose bar carries a repository
indicator combining the worktree glyph and the worktree count (so the indicator does not rely on
color alone), and the bar SHALL appear even when the repository has no live agents. The section bar
SHALL additionally carry a most-urgent-status summary badge for the repository's live agents,
combining a status glyph and a count so urgency is readable without moving the section and without
relying on color alone; a repository with no live agents SHALL render no status badge. Beneath the bar
the view SHALL render an `⌂ base` anchor row for the repository's primary worktree and one row per
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

#### Scenario: Repository section bar carries a most-urgent-status badge

- **WHEN** a recognized repository has live agents whose most-urgent status is, e.g., "working"
- **THEN** its section bar additionally shows a working glyph with a count, paired so the meaning
  does not rely on color alone, without changing the section's position

#### Scenario: Repository with no live agents shows no status badge

- **WHEN** a recognized repository has no live agents
- **THEN** its section bar shows the worktree indicator but no status badge

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
