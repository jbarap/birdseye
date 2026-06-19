## Why

The current agents view mixes two row shapes: a window that holds one agent collapses into an
inline label, while a window with several agents becomes its own header with deeper indentation.
The same datum — the window — therefore lives in two different places depending on count, which
forces the reader to re-parse the layout per session. We want one consistent row shape where every
data point sits at a fixed column, so the list scans top-to-bottom without re-orienting.

## What Changes

- **BREAKING (visual):** Replace the session→window header tree with a flat, fixed-column layout.
  Each agent row is, left to right: a **cursor column** (a glyph on the selected row, blank
  otherwise), a **status gutter** (status glyph + short word, color-coded, pinned at the same
  column regardless of grouping), an indent, a fixed-width **gray window column**, then the
  **white instance name**. Columns are static so variable-length names never shift later columns;
  overflow truncates with `…`.
- **Demote the window to a column, never a header.** A window is no longer a grouping level with
  its own header line and indent. It is shown as a gray label on every agent row (its tmux window
  name, falling back to `win <index>`). The single-agent-window inline-collapse special case and
  the per-window vertical indent guides are removed.
- **Session rows become full-width section bars:** left-aligned to the start of the list (after
  the cursor column), drawn with a blue-ish full-row background and bold text, so sessions read as
  clear breaks bracketing their agents.
- **Cursor row gets a full-row highlight** (background tint spanning the row) in addition to the
  cursor glyph, so the eye can track from the status gutter across to the name.
- **Drop co-location contiguity.** Agents within a session are ordered purely by status urgency;
  agents that happen to share a window are no longer forced adjacent. Co-location is conveyed by a
  shared gray window label, not by grouping or ordering.
- Status is shown as **glyph + 4-char word** (e.g. `● attn`, `◐ work`, `○ idle`, `✓ done`,
  `· unkn`) so statuses stay legible without relying on color.

## Capabilities

### New Capabilities

(none — this reshapes existing presentation behavior)

### Modified Capabilities

- `agent-view`: The **Refined visual presentation** requirement is rewritten for the fixed-column
  layout (cursor column, status gutter, session bars, gray window column, white name) and drops
  window headers, single-agent inline collapse, and per-nesting indent guides. The
  **Needs-attention ordering across groups** requirement drops the window-level ordering tier
  (windows are no longer a grouping level). The **Foldable sections** and **Configurable,
  vim-native navigation** requirements are adjusted to remove references to window headers as
  non-selectable rows, since window headers no longer exist.

## Impact

- `internal/agents/view.go`: rendering (`groupAgents`, `renderRows`, `itemContent`, `renderItem`,
  `itemDepth`, `guides`), item kinds (`kindWindow` removed), styles, and within-session ordering.
- `internal/agents/view_test.go`: grouping/ordering and render expectations updated.
- No change to the `Agent` data model, the Source/liveness layer, hooks, or navigation/fold
  mechanics beyond the header-row vocabulary.
