## MODIFIED Requirements

### Requirement: Refined visual presentation

The agents view SHALL present its content with a refined, readable layout built from **titled
panels**: a minimal rounded panel border in the shared border color with the panel's title embedded
in its **top border**, left-aligned and rendered in the tool-wide accent color. The list of agents
SHALL render inside such a panel titled `agents`, and when the preview is shown it SHALL render
inside such a panel titled `preview`; the view SHALL NOT show a separate floating title line above
the list, and the preview's title SHALL be the embedded border title rather than a faint inline
label that blends into the captured output. Within the list panel the view SHALL present statuses
that are visually distinguishable per status, columns whose data points sit at fixed horizontal
positions, and a clear indication of the selected row. The view SHALL group agents by tmux
**session**: each session that holds at least one agent SHALL get a session header rendered as a
full-width section bar, left-aligned to the start of the list and visually distinct from the
selected-row indication, so sessions read as clear breaks that bracket their agents. Beneath a
session there SHALL be exactly one row shape per agent: there SHALL be no window-header level and no
per-window inline-collapse special case. Each agent row SHALL present, at fixed columns: a **cursor
indicator** that is present only on the selected row; a **status indicator** combining a per-status
glyph and a short textual label, shown at the same horizontal position for every agent row
regardless of grouping, so statuses remain distinguishable without relying on color; the agent's
**window** shown as a label (its tmux window name, falling back to `win <index>`); and the agent's
**name**. Variable-length values SHALL be truncated so later columns stay aligned. The selected row
SHALL be marked both by the cursor indicator and by a full-row highlight that is visually distinct
from the session bar. The panel titles and the cursor indicator SHALL share a single accent color
that the user MAY configure; a malformed configured accent SHALL be reported as an error rather than
silently ignored. An agent's name SHALL NOT repeat its enclosing session: when the title is
prefixed with its own tmux session name, that prefix SHALL be omitted in the row. Agents with no
tmux session SHALL be grouped together under a stable ungrouped heading. The presentation SHALL
degrade gracefully on narrow terminals and color-limited terminals, and SHALL NOT depend on a fixed
terminal size.

#### Scenario: Titled list panel

- **WHEN** the agents view renders with agents present
- **THEN** the list renders inside a bordered panel whose title is embedded in the panel's top
  border (not on a separate floating line above the frame), with agent rows whose data points sit at
  fixed columns, a per-status indicator, and a clearly marked selected row

#### Scenario: Preview title embedded in its panel border

- **WHEN** the preview panel is shown beside the list
- **THEN** the preview's title is embedded in that panel's top border rather than rendered as a
  faint inline label above the captured output, so the title does not blend into the preview content

#### Scenario: Fixed-column agent row

- **WHEN** agent rows render, including ones with names or window labels of differing lengths
- **THEN** the status indicator, window label, and name each occupy the same horizontal position on
  every row, with over-long values truncated rather than shifting later columns

#### Scenario: Status legible without color

- **WHEN** the terminal supports limited or no color
- **THEN** each status is still identifiable from its glyph and short textual label, not from color
  alone

#### Scenario: Session bar brackets its agents

- **WHEN** a session holds at least one agent
- **THEN** the view renders a full-width session bar, left-aligned to the start of the list and
  visually distinct from the selected-row highlight, with that session's agents shown beneath it

#### Scenario: Window shown as a label

- **WHEN** an agent row renders for an agent whose tmux window has a name
- **THEN** the row shows that window name as a label, falling back to `win <index>` only when no
  window name is available — and the window is never rendered as its own header line

#### Scenario: Co-located agents share a window label

- **WHEN** two agents run in the same tmux window of a session
- **THEN** both rows show the same window label, making the co-location visible, even though the two
  rows need not be adjacent

#### Scenario: Selected row marked by cursor and highlight

- **WHEN** the cursor is on a row
- **THEN** that row shows the cursor indicator in its leftmost column and a full-row highlight, while
  all other rows show neither

#### Scenario: Panel titles and cursor share a configurable accent

- **WHEN** the user configures the agents-view accent color
- **THEN** the embedded panel titles and the cursor indicator both render in that color, and a
  malformed accent value is rejected with a clear error instead of being silently ignored

#### Scenario: Ungrouped agents

- **WHEN** an agent has no tmux session location
- **THEN** the view places it under a stable ungrouped heading rather than fabricating a session
  group

#### Scenario: Titles do not repeat the session

- **WHEN** an agent's title begins with its own tmux session name (for example `arewa:api` under
  session `arewa`)
- **THEN** the row shows the name without the redundant session prefix (for example `api`), while
  titles that do not carry that prefix are shown unchanged

#### Scenario: Adapts to terminal size

- **WHEN** the view is rendered in a small popup or its size changes while open
- **THEN** it adjusts its layout to fit (for example hiding or stacking the preview) rather than
  overflowing or breaking the frame

#### Scenario: Degrades on limited color

- **WHEN** the terminal supports limited or no color
- **THEN** statuses, session bars, and the selected row remain distinguishable without relying
  solely on color
