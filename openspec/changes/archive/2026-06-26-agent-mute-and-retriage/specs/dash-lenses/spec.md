## MODIFIED Requirements

### Requirement: Agents lens triage bands

The Agents lens SHALL render a flat list of agents grouped into fixed bands in the fixed order
`NEEDS YOU`, `IDLE`, `WORKING`, `MUTED`. The order reflects how much an agent wants the user's
attention: a blocked agent first, then one that has yielded its turn and is waiting on the user,
then one that is busy and needs nothing, then agents the user has muted. All four band headers
SHALL always be rendered in that order as full-width section bars consistent with the Workspaces
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
collapsing its agents onto a navigable header stand-in, consistent with how the Workspaces lens
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
  showing the collapsed state and the hidden count, mirroring a folded Workspaces section

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

### Requirement: Status as a non-positional badge in Workspaces

In the Workspaces lens, a section's most-urgent live agent status SHALL be summarized as a
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
