## 1. Titled-panel helper (agents view)

- [x] 1.1 Add a `panel(title, body string, accent lipgloss.Color, width int) string` helper in `internal/agents/view.go` that renders `body` with `frameStyle` (at `width`) and replaces the top border line with a reconstructed one: corner/dash runes from `lipgloss.RoundedBorder()` painted in `theme.Border`, a leading dash, then ` <title> ` in the accent (bold), then dashes filling to the right corner.
- [x] 1.2 Truncate an over-long title to the space available before the right corner; when the panel is too narrow for any title, fall back to the plain `frameStyle` top border (no title) instead of overflowing.
- [x] 1.3 Keep all colors sourced from `theme` tokens / the passed accent — no hex, `lipgloss.Color("…")`, SGR, or glyph literals (guard test stays green).

## 2. Apply titled panels in the agents view

- [x] 2.1 In `View()`, render the list inside `panel("agents", m.renderRows(), m.accent, …)` and remove the floating `titleStyle(...).Render("bird's-eye · agents")` line (and its use in `emptyView()` — keep an appropriate empty-state heading).
- [x] 2.2 In `renderPreview`, render the captured output inside `panel("preview", body, m.accent, …)`; delete the `previewTitle` style and the inline title line, and give the body the reclaimed inner row (drop the `avail := inner-1` reservation).
- [x] 2.3 Re-check preview/list sizing math so the embedded titles don't change overall height (titles live in the existing border rows, not new ones); verify the view still fits `m.height` in a narrow popup.

## 3. Picker titled panel (fzf)

- [x] 3.1 In `internal/picker/picker.go`, add `label:<accent>` to `fzfColorSpec` using `theme.Accent.Spec(tc)`.
- [x] 3.2 Add an fzf ≥ 0.35 probe (reuse `fzfVersionAtLeast`) and, when supported, append `--border-label " sessions "` and `--border-label-pos 2` to the fzf args; omit them (untitled border) on older fzf.
- [x] 3.3 Use a bare chevron prompt (`❯`); the picker carries no textual brand (identity rides on the shared chrome).

## 4. Docs

- [x] 4.1 Document the titled-panel convention in `DESIGN.md` (rounded border in `theme.Border`, title embedded top-left in `theme.Accent`, content-descriptive text, bare-chevron prompt / no textual brand) under the layout/selection language.
- [x] 4.2 Add [tsm](https://github.com/adibhanna/tsm) to the README "Prior art / references" list with a one-line description.

## 5. Tests & verification

- [x] 5.1 Add a test for `panel(...)` asserting the title text appears on the first (top-border) line and the body is unchanged below it (force truecolor as existing tests do).
- [x] 5.2 Update `internal/picker` tests to assert `label:` is in the color spec and that the border-label args are present when the fzf version supports it (and absent otherwise).
- [x] 5.3 Run `go build ./...`, `go test ./...` (incl. the theme guard test), and `gofmt -l` on touched files; visually confirm both `be list` and `be agents` panels in a real terminal (glyph rendering judged via raw terminal, not the TUI).
