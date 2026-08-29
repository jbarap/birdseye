# agent-status-detection Specification

## Purpose
TBD - created by archiving change agent-status-detection. Update Purpose after archive.
## Requirements
### Requirement: Hook status is authoritative; observation may only disprove

An agent's status SHALL be the status its hooks recorded. Hooks are a documented contract that
reports what actually happened, where an agent's terminal is an undocumented rendering detail
that changes between releases. Terminal observation SHALL therefore hold exactly one power: it
MAY demote a **working** record whose pane it disproves, and it SHALL NOT set, promote, or
otherwise alter any other status. When no observation is available, the hook-written status
SHALL stand, so behavior is never worse than hook-only.

#### Scenario: Stale working is corrected to idle

- **WHEN** an agent's most recent hook recorded **working** but its pane is observed inert (for
  example, it was interrupted and no `Stop` hook fired)
- **THEN** the agent is shown **idle**, not working

#### Scenario: Observation never promotes

- **WHEN** an agent's hook-written status is idle, unknown, or needs-attention
- **THEN** no terminal observation can change it; only a subsequent hook event can

#### Scenario: No observation falls back to the hook status

- **WHEN** no observation is available for an agent (it is not in tmux, its pane cannot be
  captured, or the agent type has no detector)
- **THEN** the agent's status is the hook-written status

### Requirement: Claude working claims are disproved by pane quiescence

For Claude Code, a **working** record SHALL be disproved by sampling the agent pane's rendered
screen across successive observations and finding it byte-identical for longer than a
quiescence window. A pane whose screen has changed within the window, or that carries no recent
sample, SHALL yield **no signal**.

Detection SHALL NOT key on any specific glyph, wording, or screen position. It rests only on a
working agent animating something - Claude draws a spinner and a ~1Hz elapsed-seconds counter -
which survives the UI redesigns that glyph matching does not. The quiescence window SHALL
exceed the animation period by a wide margin so a working agent can never be mistaken for a
still one.

#### Scenario: A frozen pane disproves working

- **WHEN** a Claude agent recorded as working has a pane whose captured screen is unchanged for
  longer than the quiescence window
- **THEN** the agent is shown idle

#### Scenario: An animating pane keeps working

- **WHEN** a working agent's pane redraws (its spinner or elapsed counter advances)
- **THEN** the stillness clock restarts and the agent remains working

#### Scenario: Unwatched stillness is not evidence

- **WHEN** a pane's most recent sample is older than the freshness bound
- **THEN** no signal is produced, because stillness nobody observed proves nothing

#### Scenario: A first sighting yields no signal

- **WHEN** a pane is sampled for the first time
- **THEN** no signal is produced, because quiescence requires two samples separated in time

### Requirement: Observations are shared across processes

Because quiescence is measured between samples separated in time, it SHALL NOT be held in
process memory alone. Samples SHALL be persisted to a shared store under the state root, which
both the dash refresh and every hook invocation update, so a short-lived hook process can judge
stillness it did not itself observe. Concurrent writers SHALL be safe: a lost update MAY only
delay disproving a working record, never assert one.

#### Scenario: A hook judges a sibling it did not sample

- **WHEN** a hook fires and a sibling record's pane was last sampled by a dash refresh
- **THEN** the hook reads that sample from the shared store and applies the same correction the
  dash renders

### Requirement: needs-attention is a hook-set latch

A hook-set **needs-attention** state SHALL hold until a subsequent hook records a different
state. Terminal observation SHALL NOT clear it, so a genuinely pending permission prompt stays
visible for as long as it is pending.

#### Scenario: Attention is held regardless of what the pane shows

- **WHEN** an agent is in needs-attention, whether its pane is inert or animating
- **THEN** it remains needs-attention until a clearing hook fires

### Requirement: Bounded cost per refresh

Observation SHALL be scoped to the agents whose status it can change: only panes of records
claiming to be **working** SHALL be captured. Records already idle, in needs-attention, or
without a pane SHALL cost no capture, keeping a refresh to a handful of captures rather than
one per pane on the tmux server.

#### Scenario: Only working panes are captured

- **WHEN** the view refreshes with a mix of working, idle, and needs-attention agents
- **THEN** only the working agents' panes are captured

#### Scenario: A failed capture is isolated

- **WHEN** one pane cannot be captured during a refresh
- **THEN** that pane alone loses its signal and falls back to hook status, while its siblings
  are still judged

### Requirement: Agent-agnostic level-detector seam

The observation signal SHALL be produced by a **per-agent detector** behind a common interface,
so the view and status reconciliation consume it without knowing how it was derived. The
interface SHALL be one-directional: a detector's only outputs are "this working claim is
disproved" and "no signal". It SHALL be structurally unable to report that an agent *is*
working, so terminal noise - a user scrolling or typing in a pane - can never manufacture a
status, only delay a correction. Claude's quiescence detector SHALL be one implementation.

#### Scenario: Detectors are interchangeable behind one interface

- **WHEN** a new agent type supplies a detector
- **THEN** the view consumes its signal through the same interface used for Claude's quiescence
  detector, with no change to the view

#### Scenario: A detector cannot assert working

- **WHEN** any detector observes activity on an agent's pane
- **THEN** it produces no signal rather than asserting working, and the hook status stands
