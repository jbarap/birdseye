## Context

The dashboard body is composed in `model.View` (`internal/agents/view.go`). It previously built the
two lenses side by side and joined the preview beside them, partitioning the terminal width three
ways. A chain of width helpers existed only to protect the preview's horizontal minimum against the
lens columns (`agentsReserve`/`mainWidth` reserved the Agents column; a shrink loop trimmed the
Workspaces columns; `dualLens` dropped to a single lens when both would not fit).

The premise this change acts on: the lenses and the preview do not want the same axis. The lenses
are near-fixed-width lists; the preview wants every column and row it can get. The conventional
answer is a master-detail layout - a fixed-width sidebar of the lists, and one flexible detail pane
that absorbs the slack in both dimensions. Stacking the two lenses vertically makes the sidebar one
lens wide instead of two, which is what frees the width for the preview and removes the dead gap a
top row of two narrow lenses would leave.

## Goals / Non-Goals

**Goals**
- Compose the body as a fixed-width left sidebar (Agents over Workspaces) plus a preview that fills
  the remaining width at full height.
- Give the preview both the leftover width and the whole height; give the lenses only the column
  they need.
- Keep focus movement between the stacked lenses intuitive (vertical keys).
- Keep `agents.split` meaning the sidebar's width share (its original axis), no key churn.
- Degrade gracefully: drop the preview when too narrow, collapse to the focused lens when too short.

**Non-Goals**
- No change to lens content, sourcing, ordering, folding, or linked selection.
- No configurable placement and no configurable Agents/Workspaces height ratio.
- No change to capture, the windowless-slot placeholder, or the notice/help chrome.

## Decisions

### `View` places a sidebar beside a full-height preview

`View` builds `sidebar := renderLenses()` and, when `showPreview()` holds,
`body := JoinHorizontal(Top, sidebar, "  ", renderPreview())`. `renderLenses` stacks the two lens
panels vertically:

```
ag := sidebarPanel(lensAgents,     "agents",     renderAgents(), agentsPaneRows())
ws := sidebarPanel(lensWorkspaces, "workspaces", renderRows(),   workspacePaneRows())
return JoinVertical(Left, ag, ws)
```

`sidebarPanel` forces each panel's inner height to its pane-rows share so the two panels tile the
sidebar exactly and the column stands as tall as the preview beside it.

### `agents.split` is the sidebar's width share

The config field, default (`DefaultSplit = 0.4`), and 0<x<1 validation are unchanged. `sidebarWidth`
is `round(split * width)`, clamped so the preview keeps `minPreviewCols` beside it and so a row stays
legible (`minSidebarCols`). Both lens panels render at this width; the Workspaces columns shrink to
`sidebarContentWidth` via the existing colWidths loop, now capped by the sidebar rather than by a
preview reservation. The preview takes `width - sidebarWidth - gap`.

### Height is divided between the two stacked lenses; the preview is full height

`contentRows()` is one panel's inner height (`m.height - chrome - 2`). The preview panel fills the
body: its inner height is `contentRows()`. The sidebar holds two stacked panels, so their inner
heights share `contentRows() - 2` (the second panel's border pair) - `sidebarBodyRows()`.
`agentsPaneRows()` is the triage list's visible item count, clamped so the Workspaces pane keeps
`minLensRows`; `workspacePaneRows()` takes the remainder. Because Agents is sized to content, a short
triage list leaves the tree the bulk of the height, and `agentsInner + wsInner + 4 == contentRows()
+ 2 == body`, so the sidebar never overflows and lines up with the preview.

### Focus moves on a vertical key pair

The lenses stack, so the focus-switch is vertical. `ActionFocusLeft` (focus Agents, on top) and
`ActionFocusRight` (focus Workspaces, below) keep their identities but bind `ctrl+k` and `ctrl+j`
as their primary keys, with `h`/`l`/arrows retained as aliases. This mirrors `ctrl+u`/`ctrl+d`
(half-page): plain `j`/`k` move within a lens, the ctrl pair jumps between lenses. `ctrl+j` is LF and
stays distinct from `enter` (CR) on standard terminals; `prettyKey` renders the pair as `^k`/`^j`.

### Responsive fallbacks are per-axis

- **Too narrow**: `showPreview()` is width-gated - it holds only when the minimum sidebar plus a
  usable preview fit side by side. Below that the preview is dropped and `sidebarWidth` returns the
  full width, so the stacked lenses use the whole terminal.
- **Too short**: `stackLenses()` reports whether the body can hold two minimum lens panels. When it
  cannot, `renderLenses` shows only the focused lens at full body height (the height analogue of the
  old too-narrow single-lens fallback), still switchable with the focus keys.

Because stacking removes the horizontal competition between the two lenses, there is no longer a
width-based single-lens mode: both lenses stack whenever the height allows, at whatever width.

## Risks / Trade-offs

- **`split` changes axis vs. the interim bottom-preview attempt, but restores its original width
  meaning.** A user's existing `agents.split` again controls a width share; the key and default are
  unchanged, so nothing breaks.
- **Sidebar internal whitespace.** At a fixed split share the lens rows do not fill the column
  (the Agents rows especially), leaving trailing space inside the framed panel. This reads as an
  intentional fixed-width sidebar, not the dead gap the old top-row layout left, and shrinks if the
  user lowers `split`. Sizing the sidebar to content instead was considered but rejected as more
  machinery for a marginal gain.
- **Two stacked panels need more height than one row of panels did.** The sidebar requires height
  for two bordered panels; a very short terminal collapses to the focused lens. `stackLenses` keeps
  the threshold low so only genuinely short popups lose the second lens.
- **Height division must be exact.** `agentsInner + wsInner` must equal `sidebarBodyRows` so the
  sidebar matches the preview height and neither overflows. Unit tests pin the arithmetic at several
  sizes, and the use-tty pass confirms the rendered frame at wide, narrow, and short terminals.
