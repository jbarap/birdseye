## Why

`be dash` sorts its one section list by the most-urgent live agent status, then by
name. Status is volatile (work↔idle flips on every pause), so the section you are
watching jumps position the instant its agent changes state - it can fall from the top
to the bottom of the list precisely when you look away. Sections with no agents share
the "idle" rank tier and wedge themselves arbitrarily among active repos. The list
encodes *attention* in *position*, and position is the one thing a scannable list must
hold still.

Separately, a managed repo and a one-off agent in an unmanaged repo render as two
different shapes, and a non-managed repo gives no inline hint of why it is non-managed
or how to fix it - you have to enter the session and guess.

## What Changes

- Split the dash into two always-visible **lenses** over the same agent row set:
  - **Agents** (left, default focus): a flat triage list in **fixed status bands**
    (`NEEDS YOU / WORKING / IDLE / DONE`), ordered by **recency within each band**.
    "Most urgent on top" is preserved (the `NEEDS YOU` band is always first) without
    rows teleporting past neighbors on a flicker - only band membership changes.
  - **Workspaces** (right): repos → worktrees → slots, **sorted by a stable key (name),
    never reordered by status**. Each section bar carries a most-urgent-status summary
    badge (glyph + count, never color alone). The incidental `(no-repo)` bucket - agents
    whose directory is not a recognized git repo - states *why* it offers no worktrees or
    slots inline (e.g. `not a git repo`) instead of an opaque label, so the user
    understands it without entering the session to guess.
- **Linked cursors**: selecting an agent in either lens highlights the same agent in
  the other; the preview follows the focused selection.
- **BREAKING (UX)**: the existing global needs-attention reorder of whole sections is
  removed from the Workspaces lens. Urgency floats only inside the Agents lens.
- Narrow-terminal fallback: when width can't fit both lenses plus preview, tab-toggle
  the two lenses sharing one preview.

## Capabilities

### New Capabilities
- `dash-lenses`: the two always-visible lenses (Workspaces, Agents), their distinct
  ordering contracts (stable-by-name vs. banded-by-recency), the status bands, and the
  linked-cursor behavior across them.

### Modified Capabilities
- `agent-view`: "Needs-attention ordering across groups" is **removed** - the Workspaces
  lens orders by a stable key and never reorders sections by status; cross-group urgency
  now lives in the Agents lens bands (`dash-lenses`). "Managed-repo presentation" is
  modified so the section bar additionally carries a most-urgent-status summary badge.

## Impact

- `internal/agents/view.go`: `groupRows`/`groupRank`/`rowLess` (section ordering), the
  render-item model (`renderItem`, `kindSession`/`kindRow`), the layout split, and
  cursor/nav handling gain a second focusable list.
- `internal/agents/agent.go`: the Agents lens reuses the existing `Row.Updated` for recency
  ordering, and the inline non-repo hint is a derived helper - no new row fields needed.
- `internal/cli/agents.go`: dash construction/layout wiring.
- DESIGN.md: document the two-lens language and the band palette/glyphs.
- No backend (tmux) changes expected beyond what status detection already provides.
