## ADDED Requirements

### Requirement: Live level signal governs working vs idle

The system SHALL determine the **working vs idle** distinction for an agent from that agent's
**current** terminal state, read fresh on each refresh and independent of hook events, so the
distinction reflects the agent's state now and self-heals across interrupts, crashes, and
actions taken directly in the terminal. This live level signal SHALL override a stale
hook-written working/idle status. When no level signal is available for an agent, the system
SHALL fall back to the hook-written status, so behavior is never worse than hook-only.

#### Scenario: Stale working is corrected to idle

- **WHEN** an agent's most recent hook recorded **working** but its current terminal state
  indicates idle (for example, it was interrupted and no `Stop` hook fired)
- **THEN** the agent is shown **idle**, not working

#### Scenario: No level signal falls back to the hook status

- **WHEN** no live level signal can be read for an agent (it is not in tmux, its title is
  overridden, or the agent type has no detector)
- **THEN** the agent's status is the hook-written status

### Requirement: Claude level signal from the OSC terminal title

For Claude Code, the level signal SHALL be read from the agent pane's current **OSC terminal
title**, not by scraping visible pane content. A title whose first glyph is a Braille pattern
(U+2800–U+28FF) SHALL indicate **working**; a title whose first glyph is `✳` (U+2733) SHALL
indicate **idle**; any other or absent title SHALL yield **no signal**. The matcher SHALL test
the Braille glyph **class** rather than a specific frame, because the working glyph animates
within that range while the agent works.

#### Scenario: Braille title indicates working

- **WHEN** a Claude agent pane's title begins with a glyph in U+2800–U+28FF
- **THEN** the agent's level is working

#### Scenario: Sparkle title indicates idle

- **WHEN** a Claude agent pane's title begins with `✳` (U+2733)
- **THEN** the agent's level is idle

#### Scenario: Animated spinner stays working

- **WHEN** a working agent's title glyph changes from one Braille frame to another
- **THEN** the level remains working, because the matcher tests the Braille class, not a frame

#### Scenario: Unrecognized title yields no signal

- **WHEN** a pane title carries neither a Braille glyph nor `✳`
- **THEN** no level signal is produced and the hook-written status is used

### Requirement: needs-attention is a hook-set latch

A hook-set **needs-attention** state SHALL be treated as a latch that holds until the agent is
observed to move: it SHALL be cleared when the live level signal indicates **working**, or by a
subsequent hook that records a different state. While the latch holds, an idle level signal
SHALL NOT by itself clear needs-attention, so a genuinely pending permission prompt stays
visible.

#### Scenario: Attention clears when work resumes

- **WHEN** an agent is in needs-attention and its live level signal indicates working
- **THEN** its status becomes working, because resumed work means the user answered

#### Scenario: Attention is held while the level looks idle

- **WHEN** an agent is in needs-attention and its live level signal indicates idle
- **THEN** it remains needs-attention until a clearing hook fires or the level shows working

### Requirement: Bounded cost per refresh

The system SHALL obtain the live level for all agents with work bounded independent of the
number of agents where the signal source allows it. For the title source specifically, all
pane titles SHALL be obtained with a single tmux query per refresh rather than one query per
agent.

#### Scenario: Titles are read in one batched query

- **WHEN** the view refreshes with multiple Claude agents present
- **THEN** their titles are obtained from a single tmux query, not one query per agent

### Requirement: Agent-agnostic level-detector seam

The level signal SHALL be produced by a **per-agent detector** behind a common interface, so
the view and status reconciliation consume a working/idle level without knowing how it was
derived. Claude's title detector SHALL be one implementation of that interface. Agents that do
not broadcast state in their title MAY be supported by alternative detectors (terminal-activity
diffing preferred, visible-buffer matching as a last resort) without changing the view. A level
detector SHALL only resolve working vs idle; **needs-attention SHALL remain hook-sourced**
regardless of which detector is in use.

#### Scenario: Detectors are interchangeable behind one interface

- **WHEN** a new agent type supplies a level detector
- **THEN** the view consumes its working/idle level through the same interface used for Claude's
  title detector, with no change to the view

#### Scenario: needs-attention stays hook-sourced under any detector

- **WHEN** an agent's working/idle level is resolved by any detector
- **THEN** its needs-attention state still comes from hooks, not from the detector
