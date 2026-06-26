# agent-view Specification

## Purpose

TBD: created by archiving change birds-eye-foundation. Update Purpose after archive.
## Requirements
### Requirement: Agent abstraction

The system SHALL define an `Agent` abstraction that yields known agents, each with a tmux
session/window location, a **working directory**, and a status, so multiple agent types can be
supported behind one interface. The working directory SHALL be persisted on the agent record (the
hook already reports it) so the reconciler can resolve it to a repository for recognition and
grouping. The first implementation SHALL be Claude Code.

#### Scenario: Agents surfaced through the abstraction

- **WHEN** the user runs `be agents`
- **THEN** the view lists agents obtained through the `Agent` abstraction, each with its
  tmux session/window location, working directory, and status

#### Scenario: Agent record carries its working directory

- **WHEN** an agent hook record is ingested and persisted
- **THEN** the persisted record retains the agent's working directory, not only a derived title

#### Scenario: No agents present

- **WHEN** no agents are known
- **THEN** the view shows an informative empty state rather than failing

### Requirement: Claude Code status via hooks

The Claude Code implementation SHALL source an agent's **identity** (session id, working
directory), **existence** (the Claude process the hook is a child of, used for liveness),
**needs-attention**, and **done** state from Claude Code hooks: hooks write per-session state to
a known location, and `be agents` reads that state. Status SHALL be derived from the hook event
together with its JSON payload — the event name is the primary signal, and the payload MAY
refine it where that yields a more accurate status. In particular, a `Notification` event whose
message is Claude's idle "waiting for your input" nudge SHALL map to **idle**, while a
permission/approval notification — and any notification whose message is not recognized — SHALL
map to **needs-attention**. The **working vs idle** distinction displayed to the user SHALL be
governed by the live level signal defined in the `agent-status-detection` capability, which is
read fresh on each refresh and overrides a stale hook-written working/idle status; when no level
signal is available, the hook-written status is used. Each hook invocation SHALL record the id
of the Claude process it is a child of, so the agent's liveness can be derived from that
process. The system SHALL provide a documented hook configuration to wire this up, and SHALL
render an explicit unknown status only for a present agent whose written status is missing and
for which no live level signal is available. A record whose Claude process is gone SHALL be
removed rather than shown as unknown.

#### Scenario: Hook-written attention and end state are rendered

- **WHEN** a session's hooks have written a needs-attention or done state
- **THEN** `be agents` renders that state, while the working/idle level is governed separately
  per the `agent-status-detection` capability

#### Scenario: Idle notification does not show as needs-attention

- **WHEN** a `Notification` hook fires carrying Claude's idle "waiting for your input" message
- **THEN** the session's recorded status is idle rather than needs-attention

#### Scenario: Permission and unrecognized notifications still need attention

- **WHEN** a `Notification` hook fires for a permission/approval prompt, or with a message the
  tool does not recognize
- **THEN** the session's recorded status is needs-attention

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
entries the tool added. These hook operations SHALL be reached through the hooks tier of
`be agents install <type>` / `be agents uninstall <type>` (with `be hook claude
install`/`uninstall` retained as hidden, deprecated aliases); the install/uninstall
behavior and its safety guarantees SHALL be unchanged by that reachability change.

#### Scenario: Install preserves existing config

- **WHEN** the user installs hooks into a settings file that already contains other keys
  and unrelated hooks
- **THEN** the tool adds its hook entries while leaving all other keys and hooks intact,
  and records a backup of the previous file

#### Scenario: Install reached through `be agents install`

- **WHEN** the user installs the hooks via `be agents install claude --hooks`
- **THEN** the same hook entries are installed with the same safety guarantees as the
  deprecated `be hook claude install`

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

The system SHALL run the agents view as the `be dash` command — a self-contained command
suitable for launching in a transient context such as a tmux popup — SHALL allow
navigating to a selected agent's session (landing on the exact window and pane the agent
runs in), and SHALL keep the view's rendered state current by refreshing in place as agent
state changes while the view is open, without the user re-running the command. The view's
interactive actions (jump, spawn, close, delete) SHALL resolve to the **same** operation
layer the headless `be agents` verbs expose, so the dash and the verbs stay in lifecycle
parity.

#### Scenario: Runs in a popup context

- **WHEN** the agents view is launched via `tmux display-popup -E be dash`
- **THEN** the view renders within the popup and exits cleanly when dismissed

#### Scenario: Jump to selected agent's exact pane

- **WHEN** the user selects an agent in the view
- **THEN** the system switches/attaches to that agent's tmux session and focuses the
  agent's window and pane, so the user lands where the agent runs even in a split window

#### Scenario: Jump falls back when the pane is gone

- **WHEN** the selected agent's recorded pane no longer exists
- **THEN** the system still connects to the agent's session rather than failing

#### Scenario: Dash actions share the headless operation layer

- **WHEN** the user performs an interactive action in `be dash` (jump, spawn, close, or
  delete)
- **THEN** it resolves to the same operation the corresponding `be agents` verb invokes,
  so both surfaces behave identically

#### Scenario: View refreshes as state changes

- **WHEN** an agent's hook writes new state (or a new agent appears, or one becomes stale)
  while the agents view is open
- **THEN** the view re-renders to reflect the current state without the user re-running
  `be dash`

#### Scenario: Selection is preserved across refreshes

- **WHEN** the list re-renders or re-sorts because state changed
- **THEN** the cursor stays on the same agent where it still exists, and otherwise moves to
  the nearest valid row rather than resetting to the top or pointing past the end

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

### Requirement: Managed-repo recognition

The agents view SHALL recognize repositories from a Workspace snapshot re-derived on every refresh,
never persisted, and SHALL group the view by **repository** rather than by tmux session. The snapshot
SHALL be built from cheap inputs: the live tmux structure (sessions, windows, and panes with each
pane's **start path**), the existing agent hook records (each carrying its **working directory**),
and per-repo git facts read through a cache - the repository's shared git directory
(`git rev-parse --git-common-dir`), its **worktree set** (`git worktree list`), its default branch,
and dirtiness. Recognition SHALL be **git-native**: a path is classified by resolving it to a git
worktree and its repository, not by matching a path shape, so any on-disk layout is recognized.

A repository SHALL be recognized (and rendered as a section) when **any** pane's start path **or**
**any** active agent's working directory resolves to a worktree of that repository (the same
`git-common-dir`). There SHALL be no whole-session purity gate and no `managed` boolean: a stray
non-git pane, or panes spanning multiple repositories within one tmux session, SHALL NOT suppress any
repository's section; each repository is recognized independently from its own anchoring presence. A
repository's **anchor** SHALL be its git **primary worktree** (the worktree git refuses to remove),
independent of which branch is checked out there. A tmux session SHALL NOT itself be a unit of
recognition; it is a location a row resolves to. The per-tick path classification SHALL be cheap (a
cache lookup); git SHALL be consulted only on a cache miss or when performing an action.

#### Scenario: A repo is recognized from any anchoring pane

- **WHEN** at least one pane in any tmux session has a start path inside a repository's worktree
- **THEN** the view renders a section for that repository, regardless of what other panes in the
  same session resolve to

#### Scenario: A repo is recognized from an active agent's working directory

- **WHEN** a pane was started outside a repository but an active agent in it reports a working
  directory inside that repository (e.g. a tmuxp auto-`cd` workflow)
- **THEN** the view renders a section for that repository, anchored by the agent's working directory

#### Scenario: A stray pane no longer suppresses a repository

- **WHEN** a tmux session contains panes inside a repository's worktrees and also a pane that is not
  inside any git worktree
- **THEN** the repository is still recognized and rendered, and the non-git pane does not appear as a
  worktree row

#### Scenario: Panes spanning two repositories yield two sections

- **WHEN** a single tmux session has panes resolving to two different repositories
- **THEN** the view renders a section for each repository, with each pane's row under its own
  repository

#### Scenario: Recognition uses start path and agent working directory, not live cwd

- **WHEN** a pane of a recognized worktree is `cd`-ed elsewhere after starting
- **THEN** recognition is unchanged, because it is based on the pane's start path and on active
  agents' recorded working directories, not the pane's live current directory

#### Scenario: Recognition is re-derived, not stored

- **WHEN** the view refreshes
- **THEN** the repository/worktree/anchor classification is recomputed from the current tmux + git +
  agent state rather than read from any persisted orchestration state

#### Scenario: A repository with no live presence is not shown

- **WHEN** a repository has worktrees on disk but no pane and no agent resolves to it
- **THEN** the view shows no section for it, because recognition does not scan the filesystem for
  repositories

### Requirement: Managed-repo presentation

The agents view SHALL render each recognized repository as a section whose bar carries a repository
indicator combining the worktree glyph and the worktree count (so the indicator does not rely on
color alone), and the bar SHALL appear even when the repository has no live agents. Beneath it the
view SHALL render an `⌂ base` anchor row for the repository's primary worktree and one row per
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

### Requirement: Create a session from the agents view

The agents view SHALL provide a rebindable action (default `s`) that opens a new tmux
session through the same fuzzy session picker as `be sessions` — drawing on the same
providers (running sessions, templates, directory roots, worktrees). The picker SHALL be
presented by temporarily releasing the terminal to it (so the picker's full-screen UI runs
cleanly) and restoring the agents view afterward. The selected candidate's session SHALL be
**materialized without attaching**: the system creates (or reuses) the session but does not
switch the tmux client to it, so the user stays in the agents view. The new session SHALL
appear in the view on the next refresh, with the cursor moved onto it. Cancelling the picker
SHALL leave the view unchanged. The action SHALL require orchestration support (tmux) and be
absent otherwise.

#### Scenario: Open a session without leaving the view

- **WHEN** the user invokes the new-session action and selects a candidate
- **THEN** the system ensures that candidate's tmux session exists, does not attach to it,
  and the agents view stays in front with the new session listed and focused on refresh

#### Scenario: Cancelling the picker is a no-op

- **WHEN** the user dismisses the picker without selecting a candidate
- **THEN** no session is created or changed and the view is left exactly as it was

#### Scenario: New-session action requires tmux

- **WHEN** the agents view runs without tmux (no orchestration)
- **THEN** the new-session action is unavailable and omitted from the help line

### Requirement: Create a new agent in a managed repo

The agents view SHALL provide a rebindable action (default `n`) that, when the cursor is in a
recognized repository's section, creates a new agent through a small two-field form: a **branch**
name and a **worktree** directory name, where the worktree field auto-fills from a filesystem-safe
slug of the branch until the user edits it directly. The action SHALL create that worktree on that
branch via the worktree primitive, ensure-or-reuse the repository's deterministic `be-<repo>` home
session, open a tmux window there rooted in the new worktree directory, and start the configured
agent command. When the repository's only current presence is in a user-made (non-`be-`) session,
the action SHALL still route the new window into the repository's `be-` home rather than injecting
into the user's session. The new worktree SHALL appear as a row on the next refresh.

#### Scenario: New agent creates worktree, window, and agent in the be- home

- **WHEN** the user invokes the new-agent action within a recognized repository and supplies a branch
- **THEN** the system creates the worktree, ensures the repository's `be-<repo>` session, opens a
  window rooted in the worktree, starts the configured agent command, and the worktree appears as a
  row on refresh

#### Scenario: Spawn routes to the be- home even when presence is in a user session

- **WHEN** the repository is recognized only from a pane or agent in a user-made session, and the
  user invokes the new-agent action
- **THEN** the system creates or reuses the repository's `be-<repo>` session for the new window
  rather than injecting a window into the user's session

#### Scenario: Worktree name derives from the branch until edited

- **WHEN** the user types a branch name containing characters unsafe for a directory (e.g.
  `feature/login`)
- **THEN** the worktree field shows a slugified form until the user edits it, after which it stops
  tracking the branch

### Requirement: Remove an agent and its worktree

The agents view SHALL provide a rebindable **delete** action (default the `dD` chord, config key
`delete`) that closes the row's tmux window **and** removes its git worktree, behind a confirmation
rendered as a centered popup. Delete SHALL be permitted only when the target is the **sole row of its
worktree**: if the worktree hosts more than one row (e.g. two agents), delete SHALL be refused with a
notice instructing the user to close the other occupants first, so a worktree is never removed out
from under a co-located agent. On confirm for a sole occupant, a clean worktree SHALL be removed; a
dirty (or otherwise non-removable) worktree SHALL present a force-remove choice folded into the same
popup flow, defaulting to cancel. On a windowless slot, delete SHALL remove the worktree. On an
incidental agent row with no worktree, delete SHALL only close the window. The anchor SHALL NOT be
deletable. The delete action SHALL be identical in effect to `be agents delete`.

#### Scenario: Delete is refused when the worktree has other occupants

- **WHEN** the user invokes delete on a row whose worktree also hosts another agent row
- **THEN** the system refuses with a notice and removes nothing, leaving the user to close the other
  occupant first

#### Scenario: Delete a clean sole-occupant worktree

- **WHEN** the user confirms delete on a worktree whose working tree is clean and which has no other
  rows
- **THEN** the system closes its window and removes the git worktree

#### Scenario: Delete a dirty worktree folds force into the popup

- **WHEN** the user confirms delete on a sole-occupant worktree with uncommitted changes
- **THEN** the popup presents a force-remove choice defaulting to cancel, and cancelling leaves the
  worktree and its window intact

#### Scenario: Delete a windowless slot removes the worktree

- **WHEN** the user invokes delete on a windowless slot and confirms
- **THEN** the system removes that git worktree, with no window to close

#### Scenario: Anchor is not deletable

- **WHEN** the cursor is on the `⌂ base` anchor row and the user invokes delete
- **THEN** the system refuses, because git will not remove the repository's primary worktree

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

### Requirement: Two-key chord bindings

The agents view's keymap SHALL support chords that are arbitrary **two-key sequences**,
not only a single rune pressed twice, so that a chord like `dd` (a repeated key) and a
chord like `dD` (two different keys) are both bindable. The resolver SHALL arm on the
first key of any bound chord and resolve the action on the second key; if the second key
does not complete a chord with the armed first key, the armed state SHALL clear and that
second key SHALL be handled as its own binding. A single-key binding that is also the
first key of a chord SHALL be reported as a configuration conflict at load time.

#### Scenario: Repeated-key chord resolves

- **WHEN** the user presses the two keys of a repeated-key chord such as `gg`
- **THEN** the resolver triggers that chord's action

#### Scenario: Mixed-key chord resolves

- **WHEN** the user presses the two keys of a mixed chord such as `dD` (`d` then
  shift-`d`)
- **THEN** the resolver triggers that chord's action, distinct from the `dd` chord that
  shares the first key

#### Scenario: Incomplete chord falls through

- **WHEN** the user presses a key that begins a chord and then a key that does not
  complete any chord with it
- **THEN** the armed state clears and the second key is handled as its own binding rather
  than being swallowed

#### Scenario: Single key conflicting with a chord prefix is rejected

- **WHEN** configuration binds a single key that is also the first key of a bound chord
- **THEN** loading configuration fails with a clear error naming the conflict

### Requirement: Close a window

The agents view SHALL provide a rebindable **close** action (default the `dd` chord, config
key `close`) that closes the tmux window of the row under the cursor and has **no
filesystem effect**. Close SHALL be **instant** — it SHALL NOT show a confirmation —
because under the git-native layout the worktree persists when its window closes, so the
action is reversible. Closing a managed worktree's window SHALL leave the worktree on disk,
which then renders as a slot. Closing an incidental agent's window SHALL just close it.
Closing the anchor's window SHALL close that window, and if it is the session's last window
the tmux session ends. On a windowless slot (no window to close) the action SHALL be a
no-op. The close action SHALL be identical in effect to `be agents close`.

#### Scenario: Close a worktree window leaves a slot

- **WHEN** the user presses the close key on a managed worktree row with an open window
- **THEN** the window is closed, the git worktree remains on disk, and the row renders as a
  slot on the next refresh

#### Scenario: Close is instant with no confirmation

- **WHEN** the user presses the close key on any closable row
- **THEN** the window is closed immediately with no confirmation dialog

#### Scenario: Close an incidental agent

- **WHEN** the user presses the close key on an incidental agent row (no worktree)
- **THEN** only that window is closed and no worktree is touched

#### Scenario: Close the anchor window

- **WHEN** the user presses the close key on the `⌂ base` anchor row
- **THEN** the anchor's window is closed (ending the session if it was the last window),
  and no worktree is removed

#### Scenario: Close a windowless slot is a no-op

- **WHEN** the user presses the close key on a windowless slot row
- **THEN** nothing is closed and no worktree is touched

