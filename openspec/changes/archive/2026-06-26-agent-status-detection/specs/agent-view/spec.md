## MODIFIED Requirements

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
