## ADDED Requirements

### Requirement: Notifications emit from the hook path on urgent status edges

The system SHALL emit an out-of-band notification when a tracked agent crosses into a state that
wants the user, observed from the same hook invocation that records the agent's status
(`HandleHook`), so the notification fires whether or not the dash is running. Exactly two edges
SHALL notify: a transition **into** needs-attention, and a **working -> idle** transition. No other
event (session start, start-of-work, session end, or being already in a state) SHALL notify.

#### Scenario: Entering needs-attention notifies

- **WHEN** a hook event moves an agent whose previous status was not needs-attention into
  needs-attention
- **THEN** one notification is emitted for that agent

#### Scenario: Finishing a turn notifies

- **WHEN** a hook event moves an agent from working to idle
- **THEN** one notification is emitted for that agent

#### Scenario: Re-firing while already blocked does not re-notify

- **WHEN** an agent is already in needs-attention and another event keeps it in needs-attention
- **THEN** no additional notification is emitted, because only the transition *into* the state
  notifies

#### Scenario: Start-of-work and session start are silent

- **WHEN** an agent transitions into working, or a session starts
- **THEN** no notification is emitted

#### Scenario: Fires with the dash closed

- **WHEN** a qualifying edge occurs while `be dash` is not running
- **THEN** the notification is still emitted, because it originates in the hook path rather than the
  view

### Requirement: Notification emission is best-effort and never breaks the hook chain

The system SHALL treat notification emission as best-effort: any failure to load config, resolve a
notifier, or run the delivery SHALL be swallowed so the agent's hook chain and the status record are
never affected. The status record SHALL be written regardless of whether the notification succeeds.

#### Scenario: Notifier failure does not fail the hook

- **WHEN** the configured notifier command exits non-zero, or no platform notifier is available
- **THEN** the hook invocation still succeeds and the agent's status is still recorded

### Requirement: A muted agent emits no notifications

The system SHALL reuse the existing per-location mute as the notification suppression control: when
an agent's tmux location is muted, no notification SHALL be emitted for that agent on any edge. No
separate notification-specific mute SHALL exist.

#### Scenario: Muted agent is silent on both edges

- **WHEN** a muted agent transitions into needs-attention, or from working to idle
- **THEN** no notification is emitted

#### Scenario: Unmuting restores notifications

- **WHEN** an agent's location is no longer muted and it next crosses a qualifying edge
- **THEN** a notification is emitted again

### Requirement: Per-edge notification config under `[agents.notify]`

The system SHALL expose an `[agents.notify]` config table with keys `command` (string),
`needs_attention` (bool), and `finished` (bool). An absent table SHALL mean both edges notify with
automatic delivery: `needs_attention` and `finished` SHALL default to true, and an empty `command`
SHALL select automatic delivery. Setting an edge's key to false SHALL suppress notifications for
that edge only. The keys SHALL load under the strict config loader, so an unknown key SHALL be a
config error surfaced by `be config --check`.

#### Scenario: Absent config notifies on both edges automatically

- **WHEN** no `[agents.notify]` table is present
- **THEN** both the needs-attention and finished edges notify, delivered by the auto-detected
  notifier

#### Scenario: Disabling an edge suppresses only that edge

- **WHEN** `finished = false` is set
- **THEN** working-to-idle transitions do not notify, while transitions into needs-attention still
  notify

#### Scenario: Unknown key is a config error

- **WHEN** the `[agents.notify]` table contains a misspelled key
- **THEN** strict config loading reports it as an error, consistent with the rest of the config

### Requirement: Delivery through an auto-detected notifier or a user command template

The system SHALL deliver notifications through a notifier seam with two modes. When `command` is
empty, the system SHALL auto-detect a platform notifier - `notify-send` on Linux, `osascript` on
macOS - and SHALL fall back to a terminal bell, and ultimately to no-op, when none is available,
degrading gracefully rather than erroring. When `command` is set, the system SHALL run that command
and pass the notification to it as the environment variables `BE_AGENT`, `BE_STATUS`, `BE_REPO`,
`BE_CWD`, and `BE_MESSAGE`, so users can route notifications to arbitrary services without
birdseye shipping platform-specific integrations.

#### Scenario: Auto-detect uses the platform notifier

- **WHEN** `command` is empty and a supported platform notifier is on `PATH`
- **THEN** the notification is delivered through that notifier

#### Scenario: Auto-detect degrades when no notifier exists

- **WHEN** `command` is empty and no platform notifier is available
- **THEN** the system falls back to a terminal bell or no-op without erroring the hook

#### Scenario: Command template receives the notification as environment variables

- **WHEN** `command` is set and a qualifying edge fires
- **THEN** the command runs with `BE_AGENT`, `BE_STATUS`, `BE_REPO`, `BE_CWD`, and `BE_MESSAGE` set
  from the notification
