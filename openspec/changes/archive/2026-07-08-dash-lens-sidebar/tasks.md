## 1. Sidebar width model

- [x] 1.1 Add `minSidebarCols` and `minPreviewCols`; drop the side-by-side lens-fit constants
  (`dualLensMinWidth`, `agentsNameMin/Max`) and the interim bottom-preview `minPreviewRows`.
- [x] 1.2 Add `sidebarWidth()` (`split` share of the width, clamped so the preview keeps its minimum
  and a row stays legible; full width when no preview shows) and `sidebarContentWidth()`.
- [x] 1.3 Cap the `colWidths` shrink loop by `sidebarContentWidth()`, and make `rowContentWidth()`
  the sidebar content width so both lens panels fill the column uniformly.
- [x] 1.4 Remove the retired width helpers: `agentsReserve`/`mainWidth` (as preview reservation),
  `listCapWidth`, `agentNameWidth`/`agentsContentWidth`/`agentsPaneOuter`, `dualLens`.

## 2. Sidebar height division

- [x] 2.1 Add `sidebarBodyRows()` (= `contentRows() - 2`), `agentsPaneRows()` (triage item count
  clamped so Workspaces keeps `minLensRows`), and `workspacePaneRows()` (the remainder). Preserve
  the "not windowed before the first size message" behavior.
- [x] 2.2 Add `stackLenses()` - whether the body holds two minimum lens panels - and use the pane-row
  helpers in `renderRows`, `renderAgents`, `syncViewport`, and `halfPage` in place of `listCap`.

## 3. Recompose the body

- [x] 3.1 `renderLenses` stacks the Agents panel over the Workspaces panel via `JoinVertical`, each
  forced to its pane-rows height by a new `sidebarPanel` helper; when `!stackLenses()` it renders
  only the focused lens at full height.
- [x] 3.2 `View` joins the sidebar beside the preview with `JoinHorizontal` when `showPreview()`,
  else the sidebar alone. Notice/help and modal `overlayCenter` unchanged.
- [x] 3.3 `renderPreview` sizes its width to `m.width - sidebarWidth() - gap - frame` and its height
  to `contentRows()` (full body), clipped by `MaxHeight(inner+2)`.

## 4. Gate the preview on width

- [x] 4.1 Rewrite `showPreview()` to hold when the minimum sidebar plus a usable preview fit side by
  side (`m.width - (minSidebarCols+cursor+frame) - gap >= minPreviewCols + frame`).

## 5. Vertical lens focus keys

- [x] 5.1 Bind `ctrl+k` (focus Agents/up) and `ctrl+j` (focus Workspaces/down) as the primary keys
  for `ActionFocusLeft`/`ActionFocusRight`, keeping `h`/`l`/arrows as aliases.
- [x] 5.2 Render the pair as `^k`/`^j` in `prettyKey` so the help hint reads cleanly.

## 6. Config docs

- [x] 6.1 Re-document `agents.split` in `config.go` (field, `DefaultSplit`, `SplitRatio`) and
  `default.toml` as the lens sidebar's share of the terminal width. No key/default/validation change.

## 7. Tests

- [x] 7.1 `TestSidebarHeightDivision`: the two panes sum to `sidebarBodyRows`, both keep the minimum,
  the stacked panels equal the preview height, and a short terminal collapses to the focused lens.
- [x] 7.2 `TestViewSidebarPreviewRight`: the sidebar stacks agents above workspaces and the preview
  title shares the top row (to the right).
- [x] 7.3 `TestViewWideShowsPreviewNarrowHides`: the preview shows when wide, drops when narrow.
- [x] 7.4 `TestSidebarStacksBothLensesShortFallback`: both lenses stack when tall; a short terminal
  shows only the focused lens, switchable with `ctrl+k`/`ctrl+j`.
- [x] 7.5 Update tests that assumed a narrow-width single-lens mode to assert on the specific lens
  render (`renderRows`/`renderAgents`), since both lenses now always stack.

## 8. Verify

- [x] 8.1 `go build ./...`, `go vet ./...`, `go test ./...` pass.
- [x] 8.2 Drive it via the use-tty workflow: wide 200x45 (sidebar + full-height preview, no dead
  gap), narrow 64x40 (no preview, full-width sidebar, columns truncate), short 200x11 (single focused
  lens + preview), and confirm `ctrl+j`/`ctrl+k` move focus down/up between the lenses.
