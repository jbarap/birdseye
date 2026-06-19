## MODIFIED Requirements

### Requirement: Refined visual presentation

The agents view SHALL present its content with a refined, readable layout — at minimum a titled
frame, statuses that are visually distinguishable per status, columns whose data points sit at
fixed horizontal positions, and a clear indication of the selected row. The view SHALL group agents
by tmux **session**: each session that holds at least one agent SHALL get a session header rendered
as a full-width section bar, left-aligned to the start of the list and visually distinct from the
selected-row indication, so sessions read as clear breaks that bracket their agents. Beneath a
session there SHALL be exactly one row shape per agent: there SHALL be no window-header level and no
per-window inline-collapse special case. Each agent row SHALL present, at fixed columns: a **cursor
indicator** that is present only on the selected row; a **status indicator** combining a per-status
glyph and a short textual label, shown at the same horizontal position for every agent row
regardless of grouping, so statuses remain distinguishable without relying on color; the agent's
**window** shown as a label (its tmux window name, falling back to `win <index>`); and the agent's
**name**. Variable-length values SHALL be truncated so later columns stay aligned. The selected row
SHALL be marked both by the cursor indicator and by a full-row highlight that is visually distinct
from the session bar. The view title and the cursor indicator SHALL share a single accent color
that the user MAY configure; a malformed configured accent SHALL be reported as an error rather than
silently ignored. An agent's name SHALL NOT repeat its enclosing session: when the title is
prefixed with its own tmux session name, that prefix SHALL be omitted in the row. Agents with no
tmux session SHALL be grouped together under a stable ungrouped heading. The presentation SHALL
degrade gracefully on narrow terminals and color-limited terminals, and SHALL NOT depend on a fixed
terminal size.

#### Scenario: Readable framed layout

- **WHEN** the agents view renders with agents present
- **THEN** it shows a titled frame, agent rows whose data points sit at fixed columns, a per-status
  indicator, and a clearly marked selected row

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

#### Scenario: Title and cursor share a configurable accent

- **WHEN** the user configures the agents-view accent color
- **THEN** the title and the cursor indicator both render in that color, and a malformed accent value
  is rejected with a clear error instead of being silently ignored

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

### Requirement: Needs-attention ordering across groups

The agents view SHALL order sessions by their most-urgent member so that triage stays fast: the
session containing the highest-priority agent SHALL appear first, using the status ranking that
surfaces needs-attention before working, idle, done, and unknown. Within a session, agents SHALL be
ordered by that same status ranking (then by name) regardless of which window each occupies; agents
that share a window SHALL NOT be forced to be adjacent. Ungrouped agents SHALL sort among the
sessions by their own most-urgent member.

#### Scenario: Most-urgent group floats to top

- **WHEN** one session has an agent that needs attention and another session's agents are all idle
  or done
- **THEN** the needs-attention agent's session is rendered above the other session

#### Scenario: Ordering within a session

- **WHEN** a session contains agents in differing statuses across one or more windows
- **THEN** the agents are ordered by status rank, needs-attention first, regardless of window, and
  agents sharing a window are not specially grouped together

### Requirement: Foldable sections

The agents view SHALL let the user collapse and expand a session's section through a rebindable
action (default `tab`). Folding a section SHALL hide its agents, leaving a single collapsed session
header that indicates how many agents are hidden. The action SHALL be symmetric and operate on the
section under the cursor: folding from one of a section's agents SHALL leave the cursor on the
now-collapsed header, and expanding from that header SHALL return the cursor to the section's first
agent. A folded section's header SHALL remain navigable so the user can reach and expand it. Fold
state SHALL persist across the view's live refreshes, and a section that disappears SHALL not leave
orphaned fold state.

#### Scenario: Fold collapses a section

- **WHEN** the cursor is on an agent and the user presses the fold key
- **THEN** that agent's section collapses to a single header showing the hidden agent count, its
  agents are no longer shown, and the cursor rests on the header

#### Scenario: Unfold restores the agents

- **WHEN** the cursor is on a folded section's header and the user presses the fold key
- **THEN** the section expands, its agents are shown again, and the cursor moves to the section's
  first agent

#### Scenario: Fold state persists across refresh

- **WHEN** a section is folded and the view performs a live refresh
- **THEN** the section stays folded and the selection stays on its header

### Requirement: Configurable, vim-native navigation

The agents view SHALL provide vim-native navigation motions — at minimum move up, move down, jump to
top, jump to bottom, half-page up/down, and jump to the previous/next session section (default
`{`/`}`, mirroring vim's paragraph motions) — in addition to the arrow keys, and SHALL allow the
user to rebind every navigation and command action through configuration. Navigation SHALL move
between selectable rows — every agent of an expanded section, plus the collapsed header of each
folded section — and SHALL skip non-selectable rows (the header of an expanded session), so the
cursor never rests on a row that is neither an agent nor a folded section header. With nothing
folded this means motions move between agents and "top"/"bottom" are the first and last agent.
Built-in defaults SHALL preserve the existing bindings so that absent configuration changes no
behavior. The view's help line SHALL reflect the active bindings.

#### Scenario: Vim motions navigate the list

- **WHEN** the user presses the bound keys for top, bottom, or half-page movement (by default
  `gg`/`G` and `ctrl+d`/`ctrl+u`)
- **THEN** the cursor jumps to the first agent, the last agent, or moves by half a page of agents
  respectively, clamped to the list bounds

#### Scenario: Cursor skips non-selectable headers

- **WHEN** the user moves the cursor up or down through a grouped list with no folded sections
- **THEN** the cursor moves from one agent to the next adjacent agent and never rests on an expanded
  session header

#### Scenario: Jump between sections

- **WHEN** the user presses the next-section / previous-section keys (default `}` / `{`)
- **THEN** the cursor jumps to the first selectable row of the following session, or to the top of
  the current section and then the previous section respectively, clamped to the list bounds

#### Scenario: Folded headers are reachable

- **WHEN** a section is folded and the user moves the cursor through the list
- **THEN** the cursor can land on that folded section's header (so it can be expanded), while still
  skipping expanded session headers

#### Scenario: Default bindings preserve existing behavior

- **WHEN** no navigation keys are configured
- **THEN** `j`/`k` and the arrow keys move the cursor, `enter` jumps to the selected agent, and
  `q`/`esc`/`ctrl+c` quit, exactly as before this change

#### Scenario: User rebinds an action

- **WHEN** the user configures custom keys for a navigation or command action
- **THEN** those keys drive that action and the previously default keys for it no longer do, unless
  they are also listed

#### Scenario: Help reflects active bindings

- **WHEN** the view renders its help line with custom bindings configured
- **THEN** the help line shows the configured keys for each action rather than a fixed hard-coded set

#### Scenario: Invalid keymap configuration is reported

- **WHEN** the keymap configuration is malformed (for example an unknown action or an empty key list
  for an action)
- **THEN** loading configuration fails with a clear error naming the problem rather than silently
  dropping the binding
