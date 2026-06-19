# agent-view Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.

## Requirements

### Requirement: Agent abstraction

The system SHALL define an `Agent` abstraction that yields known agents, each with a tmux
session/window location and a status, so multiple agent types can be supported behind one
interface. The first implementation SHALL be Claude Code.

#### Scenario: Agents surfaced through the abstraction

- **WHEN** the user runs `be agents`
- **THEN** the view lists agents obtained through the `Agent` abstraction, each with its
  tmux session/window location and status

#### Scenario: No agents present

- **WHEN** no agents are known
- **THEN** the view shows an informative empty state rather than failing

### Requirement: Claude Code status via hooks

The Claude Code implementation SHALL obtain status from Claude Code hooks rather than from
scraping panes: hooks write per-session state to a known location, and `be agents` reads
that state. Each hook invocation SHALL record the id of the Claude process it is a child of,
so the agent's liveness can be derived from that process. The system SHALL provide a documented
hook configuration to wire this up, and SHALL render an explicit unknown status only for a
present agent whose written status is missing. A record whose Claude process is gone SHALL be
removed rather than shown as unknown.

#### Scenario: Status reflects hook-written state

- **WHEN** a Claude Code session's hooks have written its current state
- **THEN** `be agents` renders that session's status from the written state

#### Scenario: Hook records the session process

- **WHEN** a Claude Code hook fires
- **THEN** the written state includes the id of the Claude process that ran the hook, used
  later to decide whether the agent is still present

#### Scenario: Closed session removed rather than shown unknown

- **WHEN** a session's Claude process has exited
- **THEN** its record is removed and the session leaves the view, instead of lingering as an
  unknown row

#### Scenario: Hook configuration is provided

- **WHEN** the user follows the documented setup to install the hook configuration
- **THEN** subsequent Claude Code sessions report status to `be agents` without further
  per-session setup

### Requirement: Safe hook install and uninstall

The system SHALL provide commands to install and uninstall the Claude Code hooks into the
user's settings file, preserving all unrelated configuration and hooks, supporting a
user-supplied settings path, and operating idempotently. Uninstall SHALL remove only the
entries the tool added.

#### Scenario: Install preserves existing config

- **WHEN** the user installs hooks into a settings file that already contains other keys
  and unrelated hooks
- **THEN** the tool adds its hook entries while leaving all other keys and hooks intact,
  and records a backup of the previous file

#### Scenario: Custom settings path

- **WHEN** the user supplies a non-standard settings path
- **THEN** the tool reads and writes that path instead of the default

#### Scenario: Idempotent install

- **WHEN** the user installs hooks that are already present and unchanged
- **THEN** the tool reports no change and does not duplicate entries

#### Scenario: Uninstall removes only its own hooks

- **WHEN** the user uninstalls hooks
- **THEN** only the tool's own hook entries are removed and all other configuration and
  hooks remain

#### Scenario: Malformed settings refused

- **WHEN** the settings file is not valid JSON
- **THEN** the tool refuses to modify it and reports a clear error

### Requirement: Essential status reporting

The system SHALL report an essential status for each agent from a small, well-defined set
— at minimum: needs-attention, working, idle, and done — so the user can triage which
sessions require action.

#### Scenario: Status is shown per agent

- **WHEN** the agents view renders
- **THEN** each agent row shows one status from the defined set

#### Scenario: Needs-attention surfaced first

- **WHEN** any agent is in the needs-attention state
- **THEN** the view orders or visually distinguishes those agents so they stand out

### Requirement: Popup-friendly invocation

The system SHALL run `be agents` as a self-contained command suitable for launching in a
transient context such as a tmux popup, SHALL allow navigating to a selected agent's
session — landing on the exact window and pane the agent runs in — and SHALL keep the
view's rendered state current by refreshing in place as agent state changes while the view
is open, without the user re-running the command.

#### Scenario: Runs in a popup context

- **WHEN** `be agents` is launched via `tmux display-popup -E be agents`
- **THEN** the view renders within the popup and exits cleanly when dismissed

#### Scenario: Jump to selected agent's exact pane

- **WHEN** the user selects an agent in the view
- **THEN** the system switches/attaches to that agent's tmux session and focuses the agent's
  window and pane, so the user lands where the agent runs even in a split window

#### Scenario: Jump falls back when the pane is gone

- **WHEN** the selected agent's recorded pane no longer exists
- **THEN** the system still connects to the agent's session rather than failing

#### Scenario: View refreshes as state changes

- **WHEN** an agent's hook writes new state (or a new agent appears, or one becomes stale)
  while the agents view is open
- **THEN** the view re-renders to reflect the current state without the user re-running
  `be agents`

#### Scenario: Selection is preserved across refreshes

- **WHEN** the list re-renders or re-sorts because state changed
- **THEN** the cursor stays on the same agent where it still exists, and otherwise moves to
  the nearest valid row rather than resetting to the top or pointing past the end

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

### Requirement: Live preview of the selected agent's session

The agents view SHALL show a preview of the recent terminal output of the selected agent's
tmux session, captured from that session's pane, so the user can see what an agent is doing
without switching to it. The preview SHALL follow the cursor and refresh in step with the
live list refresh. The preview SHALL be a convenience view only and SHALL NOT change how an
agent's status is determined: status remains derived from hook-written state, never from the
captured pane output. When pane output cannot be captured (no such session, not running
under tmux, or capture fails), the view SHALL show an explicit placeholder rather than
failing.

#### Scenario: Preview shows the selected agent's pane

- **WHEN** an agent is selected in the view and its tmux session has pane output
- **THEN** the view shows a preview of that session's recent terminal output

#### Scenario: Preview follows the cursor

- **WHEN** the user moves the cursor to a different agent
- **THEN** the preview updates to show that agent's session output

#### Scenario: Preview refreshes with the list

- **WHEN** the live refresh occurs while an agent is selected
- **THEN** the preview re-renders with that session's current output

#### Scenario: Preview unavailable

- **WHEN** the selected agent's pane output cannot be captured
- **THEN** the view shows an explicit placeholder for the preview and continues to function

#### Scenario: Preview does not drive status

- **WHEN** the preview shows pane output for an agent
- **THEN** the agent's displayed status is still the hook-derived status and is not inferred
  from the captured output

#### Scenario: Preview targets the agent's exact pane

- **WHEN** the agent runs in a specific pane of a split window
- **THEN** the preview captures that pane (by its recorded pane id), not merely the window's
  currently-active pane

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

### Requirement: One row per tmux location

The agents view SHALL show at most one row per tmux location (pane, or window for records
without a pane id), so that multiple agent records mapping to the same location — for
example restarting Claude in the same pane — collapse to a single, most-recent entry.
Agents occupying genuinely distinct panes SHALL each remain their own row.

#### Scenario: Restarted agent does not duplicate

- **WHEN** several agent records share the same tmux pane (such as successive Claude sessions
  in one pane)
- **THEN** the view shows one row for that location, reflecting the most recently updated
  record

#### Scenario: Distinct panes stay separate

- **WHEN** two agents run in two different panes
- **THEN** the view shows both as separate rows

### Requirement: Garbage collection of aged-out state

The system SHALL remove persisted agent state once its agent is no longer present, so the
list and the on-disk state do not grow without bound. Presence SHALL be determined by the
liveness of the process that wrote the state — the Claude session process — not by elapsed
time and not by the existence of a tmux pane. On each read of the agent list, the system SHALL
delete the state of any record whose recorded process is not currently running, and SHALL
retain the state of any record whose process is still running regardless of how long ago its
status was written. A record that cannot be associated with a running process — including one
written before process ids were tracked — SHALL be reclaimed rather than retained
indefinitely. The system SHALL NOT use time-based retention windows. Pruning SHALL be safe to
perform during normal use.

#### Scenario: Dead process is reclaimed

- **WHEN** the process that wrote an agent's state is no longer running
- **THEN** that state is deleted and the agent no longer appears, on the next read — even if
  its tmux pane (a leftover shell) is still open

#### Scenario: Live process keeps its agent

- **WHEN** an agent's recorded process is still running
- **THEN** its state is retained no matter how old its last status is, and it is never pruned
  on a timer

#### Scenario: Unidentifiable record is reclaimed

- **WHEN** a record has no recorded process id (for example it predates process tracking)
- **THEN** it is treated as not associated with a live process and is removed, rather than
  lingering forever

#### Scenario: Ended agent leaves when its process exits

- **WHEN** an agent has ended (done) and its Claude process has exited
- **THEN** its state is removed on the next read, rather than being kept for a retention window

### Requirement: Extensible agent implementations

The system SHALL keep the `Agent` abstraction as the extension point so additional agent
types and richer status workflows can be added without changing the view's command
interface.

#### Scenario: Adding a new agent type

- **WHEN** a new `Agent` implementation is added internally
- **THEN** its agents appear in `be agents` without changes to the command's invocation
  or output contract
