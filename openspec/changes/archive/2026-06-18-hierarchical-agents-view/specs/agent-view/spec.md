## MODIFIED Requirements

### Requirement: Refined visual presentation

The agents view SHALL present its content with a refined, readable layout — at minimum a
titled frame, status badges that are visually distinguishable per status, aligned columns,
and a clear indication of the selected row. The view SHALL group agents into a two-level
hierarchy by tmux location — tmux **session** at the top level and **window** beneath it —
so that agents sharing a session, or a window, are rendered together under a shared header
rather than scattered across the list. Each session that holds at least one agent SHALL get
a session header. Within a session, a window header SHALL appear only for a window that
holds more than one agent; a window holding a single agent SHALL render that agent directly
beneath the session header with its window shown inline, so the tree shows brackets only
where there is co-location to convey. A window SHALL be identified by its tmux window name
when one is available, falling back to its window index (for example `win 2`) otherwise; the
view SHALL NOT clutter window labels with redundant counts. Nesting SHALL be conveyed with vertical indent guides
so the tree structure is readable, and these guides SHALL NOT rely on color alone. An agent's
title SHALL NOT repeat information already shown by its enclosing header: when a title is
prefixed with its own tmux session name, that prefix SHALL be omitted in the row. Agents with
no tmux session SHALL be grouped together under a stable ungrouped heading. The presentation
SHALL degrade gracefully on narrow terminals and color-limited terminals, and SHALL not
depend on a fixed terminal size.

#### Scenario: Readable framed layout

- **WHEN** the agents view renders with agents present
- **THEN** it shows a titled frame with aligned columns, per-status badges, and a clearly
  marked selected row

#### Scenario: Co-located agents grouped together

- **WHEN** two or more agents run in the same tmux session
- **THEN** the view renders them under one session header, and agents that additionally share
  a window appear under one window header, so it is visually clear they are co-located

#### Scenario: Window shown by its name

- **WHEN** a window header or an inline window label renders for a window that has a tmux
  window name
- **THEN** the view shows that name (for example `nvim`), and only falls back to `win <index>`
  when no window name is available

#### Scenario: Single-agent window collapses

- **WHEN** a window within a session holds exactly one agent
- **THEN** that agent renders directly beneath the session header with its window indicated
  inline, without a separate window header line, even if another window in the same session
  has its own header

#### Scenario: Ungrouped agents

- **WHEN** an agent has no tmux session location
- **THEN** the view places it under a stable ungrouped heading rather than fabricating a
  session group

#### Scenario: Indent guides convey nesting

- **WHEN** the grouped list renders agents nested under session and window headers
- **THEN** vertical indent guides mark each nesting level so the hierarchy is readable, and
  the nesting remains discernible on terminals with limited or no color

#### Scenario: Titles do not repeat the session

- **WHEN** an agent's title begins with its own tmux session name (for example `arewa:api`
  under session `arewa`)
- **THEN** the row shows the title without the redundant session prefix (for example `api`),
  while titles that do not carry that prefix are shown unchanged

#### Scenario: Adapts to terminal size

- **WHEN** the view is rendered in a small popup or its size changes while open
- **THEN** it adjusts its layout to fit (for example hiding or stacking the preview) rather
  than overflowing or breaking the frame

#### Scenario: Degrades on limited color

- **WHEN** the terminal supports limited or no color
- **THEN** statuses, group headers, and the selected row remain distinguishable without
  relying solely on color

### Requirement: Needs-attention ordering across groups

The agents view SHALL order groups by their most-urgent member so that triage stays
fast under the hierarchy: the session containing the highest-priority agent SHALL appear
first, and within a session the window containing the highest-priority agent SHALL appear
first, using the same status ranking that surfaces needs-attention before working, idle,
done, and unknown. Within a single window (or single-occupant group), agents SHALL be
ordered by that same status ranking. Ungrouped agents SHALL sort among the groups by their
own most-urgent member.

#### Scenario: Most-urgent group floats to top

- **WHEN** one session has an agent that needs attention and another session's agents are
  all idle or done
- **THEN** the needs-attention agent's session is rendered above the other session

#### Scenario: Ordering within a group

- **WHEN** a window contains agents in differing statuses
- **THEN** within that window the agents are ordered by status rank, needs-attention first

### Requirement: Foldable sections

The agents view SHALL let the user collapse and expand a session's section through a
rebindable action (default `tab`). Folding a section SHALL hide its agents and window
headers, leaving a single collapsed session header that indicates how many agents are
hidden. The action SHALL be symmetric and operate on the section under the cursor: folding
from one of a section's agents SHALL leave the cursor on the now-collapsed header, and
expanding from that header SHALL return the cursor to the section's first agent. A folded
section's header SHALL remain navigable so the user can reach and expand it. Fold state SHALL
persist across the view's live refreshes, and a section that disappears SHALL not leave
orphaned fold state.

#### Scenario: Fold collapses a section

- **WHEN** the cursor is on an agent and the user presses the fold key
- **THEN** that agent's section collapses to a single header showing the hidden agent count,
  its agents and window headers are no longer shown, and the cursor rests on the header

#### Scenario: Unfold restores the agents

- **WHEN** the cursor is on a folded section's header and the user presses the fold key
- **THEN** the section expands, its agents are shown again, and the cursor moves to the
  section's first agent

#### Scenario: Fold state persists across refresh

- **WHEN** a section is folded and the view performs a live refresh
- **THEN** the section stays folded and the selection stays on its header

### Requirement: Configurable, vim-native navigation

The agents view SHALL provide vim-native navigation motions — at minimum move up, move
down, jump to top, jump to bottom, half-page up/down, and jump to the previous/next session
section (default `{`/`}`, mirroring vim's paragraph motions) — in addition to the arrow keys,
and SHALL allow the user to rebind every navigation and command action through configuration.
Navigation SHALL move between selectable rows — every agent of an expanded section, plus the
collapsed header of each folded section — and SHALL skip non-selectable headers (window
headers, and the header of an expanded session), so the cursor never rests on a row that is
neither an agent nor a folded section. With nothing folded this means motions move between
agents and "top"/"bottom" are the first and last agent. Built-in defaults SHALL preserve the
existing bindings so that absent configuration changes no behavior. The view's help line SHALL
reflect the active bindings.

#### Scenario: Vim motions navigate the list

- **WHEN** the user presses the bound keys for top, bottom, or half-page movement (by
  default `gg`/`G` and `ctrl+d`/`ctrl+u`)
- **THEN** the cursor jumps to the first agent, the last agent, or moves by half a page of
  agents respectively, clamped to the list bounds

#### Scenario: Cursor skips non-selectable headers

- **WHEN** the user moves the cursor up or down through a grouped list with no folded sections
- **THEN** the cursor moves from one agent to the next adjacent agent and never rests on a
  window header or an expanded session header

#### Scenario: Jump between sections

- **WHEN** the user presses the next-section / previous-section keys (default `}` / `{`)
- **THEN** the cursor jumps to the first selectable row of the following session, or to the
  top of the current section and then the previous section respectively, clamped to the list
  bounds

#### Scenario: Folded headers are reachable

- **WHEN** a section is folded and the user moves the cursor through the list
- **THEN** the cursor can land on that folded section's header (so it can be expanded), while
  still skipping window headers and expanded session headers

#### Scenario: Default bindings preserve existing behavior

- **WHEN** no navigation keys are configured
- **THEN** `j`/`k` and the arrow keys move the cursor, `enter` jumps to the selected agent,
  and `q`/`esc`/`ctrl+c` quit, exactly as before this change

#### Scenario: User rebinds an action

- **WHEN** the user configures custom keys for a navigation or command action
- **THEN** those keys drive that action and the previously default keys for it no longer do,
  unless they are also listed

#### Scenario: Help reflects active bindings

- **WHEN** the view renders its help line with custom bindings configured
- **THEN** the help line shows the configured keys for each action rather than a fixed
  hard-coded set

#### Scenario: Invalid keymap configuration is reported

- **WHEN** the keymap configuration is malformed (for example an unknown action or an empty
  key list for an action)
- **THEN** loading configuration fails with a clear error naming the problem rather than
  silently dropping the binding
