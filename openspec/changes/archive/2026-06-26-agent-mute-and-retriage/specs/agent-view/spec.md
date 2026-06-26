## MODIFIED Requirements

### Requirement: Managed-repo presentation

The agents view SHALL render each recognized repository as a section whose bar carries a
repository indicator: the worktree glyph SHALL always be shown on the bar (so a managed section
is recognizable at a glance and does not rely on color alone), while the worktree **count** and
the most-urgent-status **badge** are a summary of the section's content and SHALL be shown on
the bar only when the section is **folded** - where the rows are hidden. When the section is
expanded the bar SHALL NOT carry the worktree count or the status badge, because the agent rows
beneath already convey their statuses and the worktrees are themselves listed. The badge SHALL
combine a status glyph and a count so urgency is readable without moving the section and without
relying on color alone; a folded repository with no live agents SHALL render no status badge.
The bar SHALL appear even when the repository has no live agents. Beneath the bar the view SHALL
render an `⌂ base` anchor row for the repository's primary worktree and one row per worktree in
the repository's git worktree set, enumerated from the **git worktree set** rather than only
from open tmux windows: a worktree with no open tmux window SHALL still render as an `◌ slot`
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

#### Scenario: Repository section bar always shows the worktree glyph

- **WHEN** a recognized repository renders, expanded or folded
- **THEN** its section bar shows the worktree glyph, and the bar appears even if the repository
  currently has no live agents

#### Scenario: Folded section bar carries the count and a most-urgent-status badge

- **WHEN** a recognized repository with live agents whose most-urgent status is, e.g., "working"
  is folded
- **THEN** its section bar shows the worktree count and a working glyph with a count, paired so
  the meaning does not rely on color alone, without changing the section's position

#### Scenario: Expanded section bar drops the count and badge

- **WHEN** a recognized repository is expanded
- **THEN** its section bar carries neither the worktree count nor the status badge, since the
  visible rows already list the worktrees and convey agent statuses

#### Scenario: Folded repository with no live agents shows no status badge

- **WHEN** a folded recognized repository has no live agents
- **THEN** its section bar shows the worktree count but no status badge

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

### Requirement: Claude Code status via hooks

The Claude Code implementation SHALL source an agent's **identity** (session id, working
directory), **existence** (the Claude process the hook is a child of, used for liveness), and
**needs-attention** state from Claude Code hooks: hooks write per-session state to a known
location, and `be agents` reads that state. Status SHALL be derived from the hook event
together with its JSON payload — the event name is the primary signal, and the payload MAY
refine it where that yields a more accurate status. In particular, a `Notification` event whose
message is Claude's idle "waiting for your input" nudge SHALL map to **idle**, while a
permission/approval notification — and any notification whose message is not recognized — SHALL
map to **needs-attention**. The **working vs idle** distinction displayed to the user SHALL be
governed by the live level signal defined in the `agent-status-detection` capability, which is
read fresh on each refresh and overrides a stale hook-written working/idle status; when no level
signal is available, the hook-written status is used. A `SessionEnd` hook SHALL NOT produce a
displayed status of its own; an ended session is removed by process-liveness reclamation when
its Claude process exits, not by a terminal status value. Each hook invocation SHALL record the
id of the Claude process it is a child of, so the agent's liveness can be derived from that
process. The system SHALL provide a documented hook configuration to wire this up, and SHALL
render an explicit unknown status only for a present agent whose written status is missing and
for which no live level signal is available. A record whose Claude process is gone SHALL be
removed rather than shown as unknown.

#### Scenario: Hook-written attention state is rendered

- **WHEN** a session's hooks have written a needs-attention state
- **THEN** `be agents` renders that state, while the working/idle level is governed separately
  per the `agent-status-detection` capability

#### Scenario: Session end produces no terminal status

- **WHEN** a `SessionEnd` hook fires for a session
- **THEN** the agent does not gain a distinct end-of-life status; it continues to show its last
  detected status until its Claude process exits, at which point liveness reclamation removes it

#### Scenario: Idle notification does not show as needs-attention

- **WHEN** a `Notification` hook fires carrying Claude's idle "waiting for your input" message
- **THEN** the session's recorded status is idle rather than needs-attention

#### Scenario: Permission and unrecognized notifications still need attention

- **WHEN** a `Notification` hook fires for a permission/approval prompt, or with a message the
  tool does not recognize
- **THEN** the session's recorded status is needs-attention

### Requirement: Essential status reporting

The system SHALL report an essential status for each agent from a small, well-defined set
— at minimum: needs-attention, working, and idle — so the user can triage which sessions
require action. The set SHALL NOT include a terminal "done" status; an ended agent is removed
by process-liveness reclamation rather than parked in a done state. The user-assigned **mute**
flag is orthogonal to this set: it does not replace an agent's status but deprioritizes the
agent, which continues to report one of the essential statuses.

#### Scenario: Status is shown per agent

- **WHEN** the agents view renders
- **THEN** each agent row shows one status from the defined set, regardless of whether the agent
  is muted

#### Scenario: Needs-attention surfaced first

- **WHEN** any agent is in the needs-attention state
- **THEN** the view orders or visually distinguishes those agents so they stand out

### Requirement: Process-liveness state reclamation

The system SHALL govern an agent record's lifetime solely by the liveness of the process that
wrote the state — the Claude session process — not by elapsed time and not by the existence of a
tmux pane. On each read of the agent list, the system SHALL delete the state of any record whose
recorded process is not currently running, and SHALL retain the state of any record whose
process is still running regardless of how long ago its status was written. A record that cannot
be associated with a running process — including one written before process ids were tracked —
SHALL be reclaimed rather than retained indefinitely. The system SHALL NOT use time-based
retention windows. Pruning SHALL be safe to perform during normal use.

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

- **WHEN** an agent's session has ended and its Claude process has exited
- **THEN** its state is removed on the next read, rather than being kept for a retention window
  or held in a terminal status

## ADDED Requirements

### Requirement: Manual mute toggle

The agents view SHALL let the user mute and unmute the selected agent with a rebindable command
action (default `m`), surfaced in the help line like the other command actions. Mute SHALL be a
persisted per-agent flag that is distinct from the agent's status: muting SHALL NOT change which
status (needs-attention, working, idle) the agent reports, only its priority and placement. A
muted agent SHALL be ordered below every unmuted agent regardless of its detected status, and
its row SHALL continue to show its real status indicator so the user can still see what it is
doing. The mute flag SHALL follow the agent's tmux location so that it survives list refreshes
and a new session rotating into the same location, and SHALL be reclaimed by the same
process-liveness reclamation that removes the agent. A muted agent that transitions **into**
needs-attention SHALL be automatically unmuted, so a genuine block is never hidden; transitions
between working and idle SHALL NOT clear the mute.

#### Scenario: Toggling mute on the selected agent

- **WHEN** the user presses the bound mute key (default `m`) on the selected agent
- **THEN** that agent becomes muted and is deprioritized below all unmuted agents, and pressing
  the key again unmutes it and returns it to its detected position

#### Scenario: Muted agent keeps its real status

- **WHEN** an agent is muted while it is working
- **THEN** its row still shows the working indicator, and unmuting it returns it directly to the
  position implied by its current detected status with no stale restore

#### Scenario: Mute survives a session rotating in place

- **WHEN** a muted agent's Claude session is restarted in the same tmux location
- **THEN** the replacement agent at that location is still muted, rather than reappearing at full
  priority

#### Scenario: A new block auto-unmutes

- **WHEN** a muted agent transitions into needs-attention (for example a permission prompt)
- **THEN** its mute is cleared automatically and it returns to the needs-attention priority

#### Scenario: Routine churn does not unmute

- **WHEN** a muted agent transitions between working and idle
- **THEN** it stays muted, because those transitions are the ordinary work the user chose to
  silence
