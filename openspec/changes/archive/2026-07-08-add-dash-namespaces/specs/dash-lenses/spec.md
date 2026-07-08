## RENAMED Requirements

- FROM: `### Requirement: Stable Workspaces ordering`
- TO: `### Requirement: Stable Projects ordering`

- FROM: `### Requirement: Status as a non-positional badge in Workspaces`
- TO: `### Requirement: Status as a non-positional badge in Projects`

- FROM: `### Requirement: Incidental agents explain why they are not a workspace`
- TO: `### Requirement: Incidental agents explain why they are not a project`

## MODIFIED Requirements

### Requirement: Two always-visible lenses over one row set

The agents dashboard SHALL present two lenses over the same agent row set, stacked in a left
**sidebar**: an **Agents** lens above a **Projects** lens, with the preview pane beside them.
The two lenses SHALL be projections (ordering and grouping) of one underlying row set, not two
independently sourced lists, so a given agent is the same row in both. When a namespace is active,
that one underlying row set SHALL be the namespace-filtered set, applied upstream of both lenses so
they stay consistent (see the dash-namespaces capability). Both lenses SHALL be shown whenever the
terminal is tall enough to stack them; when it is too short to stack both, the dashboard SHALL fall
back to showing one lens at a time - the focused one - with a rebindable focus toggle between them.
The always-visible two-lens sidebar is the primary mode whenever height allows, at any width.

#### Scenario: Both lenses stack in the sidebar

- **WHEN** the dashboard opens on a terminal tall enough to stack both lens panels
- **THEN** the Agents lens and the Projects lens are both visible, the Agents lens above the
  Projects lens in the left sidebar

#### Scenario: One agent, one row in each lens

- **WHEN** an agent appears in both lenses
- **THEN** it is the same underlying row in each, so an action or selection resolves to the
  same agent regardless of which lens it was reached through

#### Scenario: Short terminal falls back to a single lens

- **WHEN** the terminal is too short to stack both lens panels
- **THEN** the dashboard shows one lens at a time - the focused one - with a rebindable toggle
  between them, rather than cramming both

### Requirement: Stable Projects ordering

The Projects lens SHALL order its sections by a stable key (repository name) and SHALL NOT
reorder sections by agent status; a section's position SHALL NOT change when one of its
agents changes status. Within a recognized repository's section, the `⌂ base` anchor row
SHALL be pinned first, worktree rows SHALL follow ordered by worktree name, and `◌ slot`
rows (worktrees with no agent) SHALL sort last. A worktree hosting multiple agents SHALL
keep its rows together in that name position. Agent status SHALL be conveyed by a per-row
indicator, never by row position.

#### Scenario: A watched section holds its position when its agent idles

- **WHEN** a section's agent transitions from working to idle (or any status change)
- **THEN** the section stays in the same position in the Projects lens, because ordering
  is by name and not by status

#### Scenario: Within-section rows are name-ordered, anchor first, slots last

- **WHEN** a repository section renders with an anchor, worktrees with agents in differing
  statuses, and empty worktree slots
- **THEN** the `⌂ base` anchor appears first, worktree rows follow in name order regardless
  of their agents' statuses, and `◌ slot` rows appear last

### Requirement: Status as a non-positional badge in Projects

In the Projects lens, a section's most-urgent live agent status SHALL be summarized as a
badge combining a status glyph and a count, so urgency is readable without moving the section.
The badge is a summary of the section's hidden content, so it SHALL be shown on the section bar
when the section is **folded** - where the rows are not visible - alongside the section's
worktree count; when the section is **expanded** the bar SHALL NOT carry the badge or worktree
count, because the agent rows convey their own statuses. The badge SHALL NOT rely on color
alone. A muted agent SHALL NOT contribute to its section's most-urgent badge - muting is a
request to stop competing for attention, so a muted agent SHALL NOT raise the badge. A folded
section with no live agents, or whose only live agents are muted, SHALL render its bar without a
status badge rather than being ranked or moved. Regardless of fold state, an incidental
section's reason for offering no worktrees SHALL remain visible (it is not conveyed by the
rows).

#### Scenario: Folded section bar shows a most-urgent-status badge

- **WHEN** a section whose most-urgent status is "working" is folded
- **THEN** its bar shows a working glyph with a count (and the worktree count), paired so the
  meaning does not depend on color alone

#### Scenario: Expanded section bar drops the summary

- **WHEN** a section is expanded
- **THEN** its bar carries neither the status badge nor the worktree count, since the visible
  agent rows already convey their statuses

#### Scenario: Folded no-agent section shows no status badge and is not reordered

- **WHEN** a folded section has no live agents
- **THEN** its bar carries no status badge and its position is unaffected by status

#### Scenario: Muted agents do not raise a section badge

- **WHEN** a folded section's only live agent is muted
- **THEN** its bar carries no status badge, so muting suppresses the section's urgency rather
  than leaving it competing for attention

### Requirement: Agents lens triage bands

The Agents lens SHALL render a flat list of agents grouped into fixed bands in the fixed order
`NEEDS YOU`, `IDLE`, `WORKING`, `MUTED`. The order reflects how much an agent wants the user's
attention: a blocked agent first, then one that has yielded its turn and is waiting on the user,
then one that is busy and needs nothing, then agents the user has muted. All four band headers
SHALL always be rendered in that order as full-width section bars consistent with the Projects
lens's section bars - a populated band bold and carrying its agent count, an empty band rendered
faint - so band positions are predictable and an empty `NEEDS YOU` band reads as an explicit
"all clear". Within a band, agents SHALL be ordered by most-recent status change, newest first;
when a per-agent change time is unavailable, the band SHALL fall back to name order. A status
change SHALL move an agent between the `NEEDS YOU`, `IDLE`, and `WORKING` bands but SHALL NOT
reorder unrelated agents within a band. The `MUTED` band SHALL hold exactly the agents the user
has muted, regardless of their detected status, and a muted agent's row SHALL continue to show
its real status indicator so the user can still see what it is doing; an agent SHALL leave the
`MUTED` band when it is unmuted or when it is reclaimed by process-liveness garbage collection.
There SHALL be no terminal "done" band: an ended agent is removed by the existing
process-liveness garbage collection, not parked in a band. A populated band SHALL be foldable,
collapsing its agents onto a navigable header stand-in, consistent with how the Projects lens
folds a section; an empty band has nothing to collapse and SHALL NOT be foldable.

#### Scenario: All four bands always render, faint when empty

- **WHEN** the Agents lens renders with no agent needing attention
- **THEN** the `NEEDS YOU` section bar still appears, faint, above the other bands,
  signalling "all clear" without changing band positions

#### Scenario: Idle ranks above working

- **WHEN** the lens holds one idle agent and one working agent and neither needs attention
- **THEN** the idle agent appears in the `IDLE` band above the working agent in the `WORKING`
  band, because an idle agent is waiting on the user while a working one needs nothing

#### Scenario: Folding a band collapses its agents

- **WHEN** the fold action is invoked on a populated band in the Agents lens
- **THEN** the band's agents are hidden and its header remains as a navigable stand-in
  showing the collapsed state and the hidden count, mirroring a folded Projects section

#### Scenario: Most-urgent band is first

- **WHEN** an agent needs attention
- **THEN** it appears in the `NEEDS YOU` band, which is the first band in the lens

#### Scenario: A flicker moves a row between bands, not past its peers

- **WHEN** one agent transitions working to idle while its band-peers are unchanged
- **THEN** that agent moves from the `WORKING` band to the `IDLE` band, and the ordering of
  the other agents within each band is unchanged

#### Scenario: Muting moves an agent to the last band

- **WHEN** the user mutes an agent
- **THEN** it moves into the `MUTED` band, which is the last band, while still showing its real
  status indicator, and unmuting it returns it to the band implied by its current status

#### Scenario: An ended agent leaves without a done band

- **WHEN** an agent's session ends and its process exits
- **THEN** it is reclaimed by the existing liveness garbage collection and disappears, rather
  than moving into any terminal band

### Requirement: Incidental agents explain why they are not a project

The Projects lens SHALL group agents whose working directory does not resolve to a
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

### Requirement: Lenses stack in a sidebar beside a full-height preview

The dashboard body SHALL be composed as a fixed-width left sidebar holding the stacked lenses and a
preview pane filling the remaining width at full height. The preview SHALL NOT share the sidebar's
column, and the lenses SHALL NOT share the preview's rows. Within the sidebar, the Agents pane and
the Projects pane SHALL divide the available height so that, together, they stand as tall as the
preview beside them and never overflow the terminal height.

#### Scenario: Preview fills the width beside the sidebar

- **WHEN** the preview is shown
- **THEN** it is rendered to the right of the sidebar, spanning the width the sidebar leaves and the
  full height of the body

#### Scenario: Sidebar height splits between the two lenses

- **WHEN** both lenses are stacked
- **THEN** the Agents pane and the Projects pane each take a share of the sidebar height, each
  keeping a minimum, and the stacked panels do not exceed the terminal height

### Requirement: Lens focus and linked selection

The Agents lens SHALL be presented above (in the sidebar) and the Projects lens below it, and the
dashboard SHALL open with the Agents lens focused, so triage is the default view. Exactly one lens
SHALL hold focus at a time. A rebindable focus action (see the vertical key pair) SHALL move focus
between the stacked lenses, mapping the up direction to the Agents lens and the down direction to
the Projects lens; within-lens navigation (default `j`/`k`) SHALL move the cursor in the focused
lens. The focused lens's current selection SHALL drive the preview. When that selection corresponds
to an agent that also appears in the other lens, the other lens SHALL show that agent with a
secondary (mirror) highlight distinct from the focused cursor. When focus moves to the other
lens, the cursor SHALL land on that counterpart agent if one exists (on the band/section
header when the counterpart sits inside a folded band or section), otherwise on a sensible
default (band top or the lens's last position). A selection with no counterpart in the other
lens - a section header, an `⌂ base` anchor, or an `◌ slot` - SHALL show no mirror highlight.

#### Scenario: Dashboard opens focused on the Agents lens

- **WHEN** the dashboard opens
- **THEN** the Agents (triage) lens above the Projects lens holds focus, and its selection drives
  the preview

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

### Requirement: Lens focus switches on a vertical key pair

Because the lenses are stacked vertically, the dashboard SHALL bind the lens focus-switch to a
vertical key pair by default: one key focuses the Agents lens (above) and another focuses the
Projects lens (below). The bindings SHALL be rebindable, and the previous horizontal keys SHALL
remain bound as aliases so existing muscle memory keeps working.

#### Scenario: Focus moves down to the Projects lens

- **WHEN** the Agents lens is focused and the user presses the focus-down key
- **THEN** the Projects lens becomes focused, driving the preview and the linked selection

#### Scenario: Focus moves up to the Agents lens

- **WHEN** the Projects lens is focused and the user presses the focus-up key
- **THEN** the Agents lens becomes focused
