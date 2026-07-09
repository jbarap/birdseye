## ADDED Requirements

### Requirement: Escalations push immediately

birdseye SHALL push an out-of-band notification the moment a tracked agent enters
needs-attention (a block on the user), and SHALL treat this as the one push class that always
fires when enabled. The escalation SHALL be structurally gated: it fires only on a transition
*into* needs-attention driven by a real cause - a permission/approval prompt, or a notification
whose message birdseye does not recognize as benign - never on merely being in the state. A
re-fired block while already in needs-attention SHALL NOT re-notify. An escalation SHALL never be
batched, delayed, or folded into the digest.

#### Scenario: Entering needs-attention pushes at once

- **WHEN** a tracked agent transitions from any non-attention status into needs-attention
- **THEN** birdseye pushes one notification immediately, independent of what other agents are doing

#### Scenario: Re-fired block does not re-notify

- **WHEN** a needs-attention agent receives another event that keeps it in needs-attention
- **THEN** birdseye pushes no additional notification for that agent

#### Scenario: Escalation is exempt from batching

- **WHEN** an agent enters needs-attention while other agents are still working
- **THEN** the escalation is delivered immediately and is not held for the terminal digest

### Requirement: A routine agent finish is pull-only

birdseye SHALL NOT push a notification for a single tracked agent's `working -> idle` finish on
its own. A detached/orchestrated agent (session `Kind == "bg"`) finishing a turn is routine
progress: it SHALL update the agent's record for the `be agents` pull surface and stay silent.
The user's own foreground/interactive agent is not silenced by this rule; its finish participates
in the run-settled digest below.

#### Scenario: One child agent finishing is silent

- **WHEN** a detached agent transitions `working -> idle` while at least one other tracked agent is
  still working
- **THEN** birdseye pushes no notification, and the agent's new idle status is recorded for the view

#### Scenario: Progress is visible on the pull surface

- **WHEN** a routine finish is suppressed as a push
- **THEN** the agent's updated status SHALL be readable from the `be agents` view without any push

### Requirement: The run settling emits one terminal digest

birdseye SHALL emit exactly one out-of-band digest notification when the set of working tracked
agents drains to empty - when the last working agent settles - rather than one ping per finished
agent. The digest SHALL report how many agents finished and SHALL name any that ended in a
non-benign state (e.g. needs-attention) first. A finish that leaves other agents still working
SHALL NOT emit a digest.

#### Scenario: Last agent to settle triggers the digest

- **WHEN** a tracked agent transitions `working -> idle` and no other tracked agent remains working
- **THEN** birdseye pushes exactly one digest notification summarizing the agents that finished

#### Scenario: A finish with siblings still working is held

- **WHEN** an agent finishes but at least one other tracked agent is still working
- **THEN** birdseye pushes no digest yet; the finish is only reflected in the pull surface

#### Scenario: Anomalies are surfaced first in the digest

- **WHEN** the run settles and one or more agents ended in needs-attention
- **THEN** the digest names those agents before the routine count

### Requirement: Mute and policy suppress pushes uniformly

A muted agent SHALL never produce any push - escalation or digest. The notification policy flags
SHALL independently gate the escalation class and the terminal digest, so a user can disable
either without disabling the other. When configuration cannot be loaded, birdseye SHALL fall back
to defaults with both classes enabled so notifications keep working past a config error.

#### Scenario: Muted agent is silent

- **WHEN** a muted agent enters needs-attention or is the last agent to settle
- **THEN** birdseye pushes no notification for it

#### Scenario: Escalation and digest gate independently

- **WHEN** the terminal-digest flag is disabled but the escalation flag is enabled
- **THEN** needs-attention still pushes while run-settled digests are suppressed

#### Scenario: Config error falls back to enabled

- **WHEN** the notification configuration cannot be loaded
- **THEN** birdseye uses defaults with both the escalation and digest classes enabled

### Requirement: Delivery is best-effort and pluggable

Notification delivery SHALL be best-effort and downstream of the state write: a delivery failure
SHALL never fail the hook chain or lose a status update. Delivery SHALL route through the
user's configured command template when set (exporting the notification fields as environment
variables), otherwise through the auto-detected platform notifier, degrading to a terminal bell
and then a no-op when none is present.

#### Scenario: Delivery failure does not disturb the hook

- **WHEN** the notifier returns an error
- **THEN** the hook still completes successfully and the agent's status write is preserved

#### Scenario: User command template receives the notification

- **WHEN** a notification fires and a command template is configured
- **THEN** birdseye runs the command with the notification's fields available as environment
  variables
