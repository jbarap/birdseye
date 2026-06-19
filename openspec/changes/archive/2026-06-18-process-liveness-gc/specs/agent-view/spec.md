## MODIFIED Requirements

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
