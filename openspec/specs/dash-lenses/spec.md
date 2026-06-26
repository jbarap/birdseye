# dash-lenses Specification

## Purpose
The `be dash` agents dashboard presents two always-visible lenses over one shared agent row
set: an **Agents** triage lens (flat, fixed status bands, recency within band) on the left
and a **Workspaces** topology lens (repository → worktree → slot tree, stable name order) on
the right, plus a shared preview. This capability defines how those lenses are sourced,
ordered, focused, folded, and linked, so topology (stable position) and attention (volatile
urgency) stop competing for a single sort key.

## Requirements
### Requirement: Two always-visible lenses over one row set

The agents dashboard SHALL present two lenses over the same agent row set, both visible at
once: a **Workspaces** lens and an **Agents** lens, alongside the preview pane. The two
lenses SHALL be projections (ordering and grouping) of one underlying row set, not two
independently sourced lists, so a given agent is the same row in both. When the terminal is
too narrow to render both lenses and the preview, the dashboard SHALL fall back to showing
one lens at a time with a rebindable toggle, sharing the single preview; the always-visible
two-lens layout is the primary mode whenever width allows.

#### Scenario: Both lenses render side by side

- **WHEN** the dashboard opens on a terminal wide enough for both lenses and the preview
- **THEN** the Workspaces lens and the Agents lens are both visible at the same time

#### Scenario: One agent, one row in each lens

- **WHEN** an agent appears in both lenses
- **THEN** it is the same underlying row in each, so an action or selection resolves to the
  same agent regardless of which lens it was reached through

#### Scenario: Narrow terminal falls back to a toggle

- **WHEN** the terminal is too narrow to fit both lenses plus the preview
- **THEN** the dashboard shows one lens at a time with a rebindable toggle between them and
  a shared preview, rather than truncating both

### Requirement: Stable Workspaces ordering

The Workspaces lens SHALL order its sections by a stable key (repository name) and SHALL NOT
reorder sections by agent status; a section's position SHALL NOT change when one of its
agents changes status. Within a recognized repository's section, the `⌂ base` anchor row
SHALL be pinned first, worktree rows SHALL follow ordered by worktree name, and `◌ slot`
rows (worktrees with no agent) SHALL sort last. A worktree hosting multiple agents SHALL
keep its rows together in that name position. Agent status SHALL be conveyed by a per-row
indicator, never by row position.

#### Scenario: A watched section holds its position when its agent idles

- **WHEN** a section's agent transitions from working to idle (or any status change)
- **THEN** the section stays in the same position in the Workspaces lens, because ordering
  is by name and not by status

#### Scenario: Within-section rows are name-ordered, anchor first, slots last

- **WHEN** a repository section renders with an anchor, worktrees with agents in differing
  statuses, and empty worktree slots
- **THEN** the `⌂ base` anchor appears first, worktree rows follow in name order regardless
  of their agents' statuses, and `◌ slot` rows appear last

### Requirement: Status as a non-positional badge in Workspaces

In the Workspaces lens, each section bar SHALL carry a summary of its most-urgent live
agent status as a badge combining a status glyph and a count, so urgency is readable without
moving the section. The badge SHALL NOT rely on color alone. A section with no live agents
SHALL render its bar without a status badge rather than being ranked or moved.

#### Scenario: Section bar shows a most-urgent-status badge

- **WHEN** a section has agents whose most-urgent status is "working"
- **THEN** its bar shows a working glyph with a count, paired so the meaning does not depend
  on color alone

#### Scenario: No-agent section shows no status badge and is not reordered

- **WHEN** a section has no live agents
- **THEN** its bar carries no status badge and its position is unaffected by status

### Requirement: Agents lens triage bands

The Agents lens SHALL render a flat list of agents grouped into fixed status bands in the
fixed order `NEEDS YOU`, `WORKING`, `IDLE`, `DONE`. All four band headers SHALL always be
rendered in that order as full-width section bars consistent with the Workspaces lens's
section bars - a populated band bold and carrying its agent count, an empty band rendered
faint - so band positions are predictable and an empty `NEEDS YOU` band reads as an explicit
"all clear". Within a band, agents SHALL be ordered by most-recent status change, newest
first; when a per-agent change time is unavailable, the band SHALL fall back to name order. A
status change SHALL move an agent between bands but SHALL NOT reorder unrelated agents within
a band. A populated band SHALL be foldable, collapsing its agents onto a navigable header
stand-in, consistent with how the Workspaces lens folds a section; an empty band has nothing
to collapse and SHALL NOT be foldable. The lifetime of a `DONE` agent SHALL be governed by
the existing process-liveness garbage collection, not by any triage-specific time window: a
done agent SHALL remain in the `DONE` band only while its process is alive and SHALL be
reclaimed when that process exits.

#### Scenario: All four bands always render, faint when empty

- **WHEN** the Agents lens renders with no agent needing attention
- **THEN** the `NEEDS YOU` section bar still appears, faint, above the other bands,
  signalling "all clear" without changing band positions

#### Scenario: Folding a band collapses its agents

- **WHEN** the fold action is invoked on a populated band in the Agents lens
- **THEN** the band's agents are hidden and its header remains as a navigable stand-in
  showing the collapsed state and the hidden count, mirroring a folded Workspaces section

#### Scenario: Most-urgent band is first

- **WHEN** an agent needs attention
- **THEN** it appears in the `NEEDS YOU` band, which is the first band in the lens

#### Scenario: A flicker moves a row between bands, not past its peers

- **WHEN** one agent transitions working to idle while its band-peers are unchanged
- **THEN** that agent moves from the `WORKING` band to the `IDLE` band, and the ordering of
  the other agents within each band is unchanged

#### Scenario: A done agent leaves when its process exits

- **WHEN** a `DONE` agent's process exits
- **THEN** it is reclaimed by the existing liveness garbage collection and leaves the `DONE`
  band, with no separate triage retention timer

### Requirement: Lens focus and linked selection

The Agents lens SHALL be presented on the leading (left) side and the Workspaces lens on the
trailing (right) side, and the dashboard SHALL open with the Agents lens focused, so triage
is the default view. Exactly one lens SHALL hold focus at a time. A rebindable spatial action
(default `h`/`l`, left/right) SHALL move focus between the side-by-side lenses, mapping the
leading direction to the Agents lens and the trailing direction to the Workspaces lens;
within-lens navigation (default `j`/`k`) SHALL move the cursor in the focused lens. The
focused lens's current selection SHALL drive the preview. When that selection corresponds to
an agent that also appears in the other lens, the other lens SHALL show that agent with a
secondary (mirror) highlight distinct from the focused cursor. When focus moves to the other
lens, the cursor SHALL land on that counterpart agent if one exists (on the band/section
header when the counterpart sits inside a folded band or section), otherwise on a sensible
default (band top or the lens's last position). A selection with no counterpart in the other
lens - a section header, an `⌂ base` anchor, or an `◌ slot` - SHALL show no mirror highlight.

#### Scenario: Dashboard opens focused on the Agents lens

- **WHEN** the dashboard opens
- **THEN** the Agents (triage) lens on the left holds focus, and its selection drives the
  preview

#### Scenario: Selecting in one lens mirrors in the other

- **WHEN** an agent is selected in the focused lens and that agent also appears in the other
  lens
- **THEN** the other lens shows that agent with a secondary highlight distinct from the
  focused cursor, and the preview reflects the focused selection

#### Scenario: Switching focus lands on the counterpart

- **WHEN** focus moves from one lens to the other while an agent with a counterpart is
  selected
- **THEN** the newly focused lens places its cursor on that same agent

#### Scenario: Non-agent selection shows no mirror

- **WHEN** the focused selection is a section header, an `⌂ base` anchor, or an `◌ slot`
- **THEN** no mirror highlight is shown in the other lens, since there is no counterpart
  agent

### Requirement: Incidental agents explain why they are not a workspace

The Workspaces lens SHALL group agents whose working directory does not resolve to a
recognized git repository under an incidental section, and that section SHALL state inline
why it offers no worktrees or slots (for example, the directory is not a git repository),
rather than presenting an opaque label. Such a section SHALL still render through the same
section-then-row structure as recognized repositories, not as a bare one-off entry.

#### Scenario: No-repo section states its reason

- **WHEN** an agent runs in a directory that is not a recognized git repository
- **THEN** it appears under an incidental section whose label or rows state the reason (e.g.
  "not a git repo"), so the user understands why no worktrees or slots are offered

#### Scenario: Incidental agents use the same structure

- **WHEN** the incidental section renders
- **THEN** it uses the same section-then-row layout as a recognized repository, not a
  special bare entry

