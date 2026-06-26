## Why

Agent status is hook-driven: each Claude Code hook event writes a status, and the dash
displays whatever the last event said. But status is **edge-triggered while displayed as a
level** - between events there is no signal, so any transition that fires no hook freezes
the display on a stale value. The result is the bugs we see: "working" lingers after an Esc
interrupt or crash (no `Stop` ever fired), and "needs-attention" lingers after a permission
prompt is answered directly in the terminal. Hooks are excellent at *identity* and *semantic
events* and structurally blind to *current level*, which is exactly what an at-a-glance dash
needs to be correct about.

## What Changes

- **Split status into the signals each source is actually good at.** Hooks remain the source
  for identity (session id, working directory), existence (process liveness), **needs-attention**
  (the `Notification` event is an explicit "I am blocked on you"), and **done** (session end /
  dead process). A new **level signal** determines **working vs idle** from the agent's
  *current* terminal state, read fresh on each dash refresh - so it is always true *now* and
  self-heals across interrupts, crashes, and in-terminal actions.
- **For Claude, the level signal is the OSC terminal title**, not pane scraping. Claude
  broadcasts its state in the title it sets: a Braille spinner glyph (U+2800-U+28FF) while
  working, a `✳` sparkle (U+2733) when idle. tmux exposes this as `#{pane_title}`, read with
  one `display-message` call per agent pane - no `capture-pane`, no visible-layout parsing.
  This is both level-true and far more stable than scraping the TUI, because the title is a
  deliberate out-of-band state channel rather than incidental layout.
- **Define precedence between the signals.** The level signal overrides a stale hook status
  for the working/idle distinction. A fresh `needs-attention` latch (set by a `Notification`)
  holds until the agent is observed to move - the level signal showing the agent working, or a
  later clearing hook - which kills the stale-attention case in the common path.
- **Introduce a per-agent detector seam so the design is agent-agnostic within reason.** OSC
  title is a strong but not universal convention (Claude and Codex broadcast rich state in the
  title; opencode and Gemini put nothing there). The view asks a detector for an agent's level
  state without knowing *how* it was read. Claude (title-based) ships first. For agents that
  broadcast no title, the preferred fallback is **terminal-activity diffing** - watch whether
  the pane's content is changing versus quiescent to tell working from idle (as coder's
  `agentapi` does), a level-true signal with no fragile UI strings - with visible-buffer regex
  reserved as a last resort. needs-attention still comes from the hook, so a fallback only has
  to resolve working vs idle.
- **Revise the documented "status from hooks, not pane scraping" stance.** The `agent.go`
  package header, the `agent-view` spec requirement, and the DESIGN/CLAUDE notes that assert
  hook-only status are updated to describe the layered model. This is a deliberate, knowing
  reversal of an earlier design choice, not an oversight.

## Capabilities

### New Capabilities

- `agent-status-detection`: How an agent's live status is determined - the layered model
  (hook-sourced identity/existence/attention/done plus a per-agent level signal for
  working/idle), the OSC-title level source for Claude, the precedence rules that let the
  level signal correct stale hook state, and the per-agent detector seam that keeps the model
  agent-agnostic.

### Modified Capabilities

- `agent-view`: The "Claude Code status via hooks" requirement is relaxed - it no longer
  asserts status comes from hooks *rather than* from the pane. It now sources identity,
  existence, needs-attention, and done from hooks, and defers the working/idle level signal to
  `agent-status-detection`. The needs-attention and idle-notification scenarios are preserved.

## Impact

- **Code**: `internal/agents` - `StatusFor`/`HandleHook` (hook now owns attention/done/identity,
  not the working/idle level), a new level-detector reading `#{pane_title}` per agent pane on
  refresh, and the source reconciliation that merges hook record + level signal with the
  precedence rules. `internal/tmux` already exposes `display-message`; the read path adds a
  current-pane-title query. The dash refresh does one extra cheap tmux read per agent row.
- **Docs/philosophy**: `agent.go` package header, `DESIGN.md`, and `CLAUDE.md` notes that state
  "not scraping panes" are revised to the layered model.
- **No new external dependencies.** No change to the hook install contract; existing hook
  records keep working (the level signal is additive and degrades to hook-only when a title is
  absent).
- **Prior art / licensing.** The OSC-title technique is credited to
  [herdr](https://github.com/ogulcancelik/herdr) (added to the README prior-art list), which
  improved on our existing hook model. herdr is AGPL-3.0; birdseye copies none of its code or
  detection manifests - the title glyphs are derived from our own observation of Claude and the
  implementation is clean-room, so there is no license entanglement. (birdseye now carries an
  MIT `LICENSE`.)
