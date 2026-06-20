## MODIFIED Requirements

### Requirement: Claude Code status via hooks

The Claude Code implementation SHALL obtain status from Claude Code hooks rather than from
scraping panes: hooks write per-session state to a known location, and `be agents` reads
that state. Status SHALL be derived from the hook event together with its JSON payload — the
event name is the primary signal, and the payload MAY refine it where that yields a more
accurate status. In particular, a `Notification` event whose message is Claude's idle
"waiting for your input" nudge SHALL map to **idle**, while a permission/approval notification —
and any notification whose message is not recognized — SHALL map to **needs-attention**. Each
hook invocation SHALL record the id of the Claude process it is a child of, so the agent's
liveness can be derived from that process. The system SHALL provide a documented hook
configuration to wire this up, and SHALL render an explicit unknown status only for a present
agent whose written status is missing. A record whose Claude process is gone SHALL be removed
rather than shown as unknown.

#### Scenario: Status reflects hook-written state

- **WHEN** a Claude Code session's hooks have written its current state
- **THEN** `be agents` renders that session's status from the written state

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
