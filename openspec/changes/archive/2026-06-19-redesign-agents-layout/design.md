## Context

`be agents` currently renders a session→window→agent tree (`groupAgents` in
`internal/agents/view.go`). A window with one agent is collapsed: the agent renders inline at the
session's indent and carries its window name as a trailing label. A window with several agents gets
its own header line and the agents nest one level deeper with vertical indent guides. The window
datum therefore appears in two different roles — inline column vs. section header — depending on the
agent count, and the agent rows have two different shapes and indents. That inconsistency makes the
list re-orient per session.

This redesign keeps the session grouping but flattens everything below it into one fixed-column row
shape, so every data point has a stable horizontal position. It is a presentation change only:
the `Agent` model, the hook/liveness layer, and the fold/navigation mechanics are unchanged except
where their vocabulary referenced the now-removed window header.

## Goals / Non-Goals

**Goals:**

- One consistent agent-row shape with data points at fixed columns, scannable top-to-bottom.
- Status legible at a glance and without color (glyph + short word in a pinned gutter).
- Sessions read as clear section breaks; the cursor row is easy to track across its width.
- Co-location (two agents in one window) remains discernible via a shared window label.

**Non-Goals:**

- No change to how status/liveness is determined, to hooks, or to the jump/preview behavior.
- No change to which rows are navigable or to the fold action's semantics (only the wording that
  referenced window headers).
- Not introducing a third grouping level or per-window headers — windows become a column.

## Decisions

### Decision: Flat fixed-column row, window demoted to a column

Below the session header there is exactly one row kind: an agent row. Left to right it is
`cursor | status gutter | indent | window | name`. The window is a fixed-width gray column on every
agent row (tmux window name, falling back to `win <index>`), never a header and never an inline
special case. This removes `kindWindow`, the single-agent collapse branch, the `nested` flag, and
the per-nesting indent guides from the render path.

- **Why:** A single row shape is the entire point — the reader learns the columns once. Co-location
  is still visible (two rows under a session with the same gray window label).
- **Alternative — always show a window header (uniform tree):** rejected earlier; it doubles
  vertical cost in the common one-agent-per-window case and dilutes the "shared window" signal into
  "a window exists."
- **Alternative — keep the inline/header conditional:** rejected; that is the inconsistency being
  removed.

### Decision: Status gutter = glyph + 4-char word, pinned at column 0 of content

Status renders as a colored glyph plus a 4-char word: `● attn`, `◐ work`, `○ idle`, `✓ done`,
`· unkn`. It sits in a fixed gutter immediately right of the cursor column, at the same horizontal
position for every agent row regardless of session. Session header rows have a blank gutter.

- **Why:** The gutter is the one place legibility should beat density; the word labels the glyph so
  the view degrades cleanly without color, and a fixed gutter makes status a single vertical stripe.
- **Alternative — glyph only:** denser but leans on color to disambiguate `●`/`○`/`◐`.

### Decision: Cursor column + full-row highlight

A leftmost cursor column (2 cells) holds the cursor glyph on the selected row and is blank
otherwise; it applies to whatever row the cursor is on, including a folded session header. The
selected row additionally gets a full-row background highlight spanning gutter→name.

- **Why:** The glyph marks the row unambiguously; the highlight lets the eye ride from the status
  gutter across to the name. Keeping both was an explicit choice.

### Decision: A single configurable accent for title + cursor

The view title and the cursor glyph share one accent color (default `theme.Accent`, a magenta that
sits off the warm-red family). It is user-configurable via `agents.accent` (a `#rrggbb` hex);
`ResolveAccent` validates it and errors on a malformed value, mirroring how an invalid keymap is
reported. The model carries the resolved accent so both the title and cursor always match.

- **Why:** The cursor was originally coral, which collided with the coral title and the red
  needs-attention status (coral `#ff7a6b` and red `#ff6b6b` are near-twins). Moving the cursor —
  and the title with it — to a distinct magenta keeps the urgency red unique, and unifying the two
  under one knob makes the accent themeable. Needs-attention stays red; only the chrome moved.

### Decision: Session rows as full-width blue bars, left-aligned

Session headers are left-aligned to the start of the list (right after the cursor column), drawn
with a blue-ish full-row background and bold light-blue text, distinct from the cursor row's neutral
highlight. Agent rows are indented past the session name so they read as belonging to the session.
Two background languages coexist: blue = structural (a session), neutral = the cursor row.

- **Why:** Full-width bars make sessions unmistakable section breaks; a distinct color keeps them
  from being confused with the cursor highlight.

### Decision: Drop co-location contiguity; order by urgency within a session

Within a session, agents are ordered solely by status rank then title (`agentLess`). Agents sharing
a window are not forced adjacent. The session-level ordering still floats the most-urgent session to
the top.

- **Why:** With the window reduced to a label, urgency ordering is the more useful invariant for
  triage, and forcing contiguity would re-introduce window-aware ordering we just removed.

## Risks / Trade-offs

- [Co-location is subtler than a shared header — two same-window agents may be non-adjacent] →
  Mitigation: the gray window label is identical on both rows; the original "make co-location
  visible" goal is met by the shared label, and triage-by-urgency is the primary use.
- [The cursor glyph is a Nerd Font codepoint that may render as tofu or double-width on some fonts]
  → Mitigation: verify in a raw terminal; if double-width, drop the trailing space so the gutter
  stays 2 cells. The full-row highlight already marks the selection if the glyph is missing.
- [Two background colors (blue session, neutral cursor) could clash when the cursor sits on the
  first agent under a session bar] → Mitigation: keep the session blue and the cursor neutral
  visually distinct; the cursor row also carries the glyph.
- [Removing indent guides reduces explicit nesting cues] → Mitigation: the flat layout has only one
  level under a session, conveyed by indent + the session bar; guides are no longer needed.

## Open Questions

- Exact column widths (window column, name column) and the precise blue/neutral background values —
  tunable during implementation against a real terminal capture.
- Whether unknown status shows `· unkn` or a blank gutter — leaning `· unkn` for consistency.
