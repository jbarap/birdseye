## 1. Reshape the grouping model

- [x] 1.1 Remove `kindWindow` and the `windowGroup` header level from the render model in
  `internal/agents/view.go`; collapse `renderItem` to two kinds (session header, agent row) and
  drop the `nested` field.
- [x] 1.2 Rewrite `groupAgents` to build session→agent groups only: one session header per session,
  then its agents ordered by `agentLess` (status rank, then title) with no window grouping and no
  single-agent inline-collapse branch.
- [x] 1.3 Carry each agent's window label (`windowLabel(window, windowName)`) onto its agent
  `renderItem` so it can render as a column, and delete the window-header logic that no longer
  applies (`sessionRank`, `sessionCount`, `windowLess`).
- [x] 1.4 Drop the per-nesting `guides`/`itemDepth` machinery (single fixed indent for agent rows
  now) and remove the `windowLess` window ordering if no longer referenced.

## 2. Render the fixed-column layout

- [x] 2.1 Add a leftmost cursor column: a coral cursor glyph on the selected row, two blank cells
  otherwise, applied to whatever row the cursor is on (including a folded session header).
- [x] 2.2 Render the status gutter as a per-status glyph + 4-char word (`● attn`, `◐ work`,
  `○ idle`, `✓ done`, `· unkn`), color-coded, at a fixed position right of the cursor column;
  session header rows get a blank gutter.
- [x] 2.3 Render session headers as full-width bars left-aligned after the cursor column, with a
  blue-ish full-row background and bold light-blue text, distinct from the cursor highlight.
- [x] 2.4 Render agent rows as `cursor | status gutter | indent | gray window column | white name`,
  with fixed widths and `truncate` so later columns stay aligned; keep `displayTitle` so the name
  drops a redundant `<session>:` prefix.
- [x] 2.5 Apply the full-row neutral highlight to the selected agent row (and folded header) spanning
  gutter→name, in addition to the cursor glyph.
- [x] 2.6 Update styles/constants (column widths, new session-bar and cursor colors via the `theme`
  package) and remove now-unused styles (window header, indent guides).
- [x] 2.7 Give the title and cursor a single accent color (default `theme.Accent` magenta, distinct
  from coral chrome and the red needs-attention status), make it user-configurable via
  `agents.accent` (validated `#rrggbb` hex through `ResolveAccent`, error on malformed), and thread
  it through the model and `Run`.

## 3. Tests and verification

- [x] 3.1 Update `internal/agents/view_test.go`: grouping tests assert session→agent order with no
  window headers, urgency ordering within a session, and that two same-window agents share a window
  label without being forced adjacent.
- [x] 3.2 Update/extend render-string assertions for the new row shape (cursor column, status gutter
  word, gray window column, white name) and session-bar rows.
- [x] 3.3 `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` all clean.
- [x] 3.4 Manual check: run `be agents` (or a captured render) in a raw terminal and confirm the
  cursor glyph, status gutter, session bars, and column alignment render as designed; confirm the
  glyph is single-width (drop the trailing space if it is double-width).
