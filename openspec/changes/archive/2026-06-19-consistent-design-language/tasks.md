## 1. Shared theme tokens

- [x] 1.1 Add a shared selection-glyph token to `internal/theme` (e.g. `CursorGlyph = "\U000f0055"`)
  with a doc comment naming it the tool-wide selection pointer.
- [x] 1.2 Confirm `theme.Accent` is documented as the tool-wide selection/brand accent (not
  agents-only) and `theme.Coral` as the tmuxp candidate-type color.

## 2. Align the surfaces

- [x] 2.1 `internal/agents/view.go`: replace the local `cursorGlyph` const with `theme.CursorGlyph`.
- [x] 2.2 `internal/picker/picker.go`: in `fzfColorSpec`, change `hl+`, `pointer`, `prompt`, and
  `marker` from `theme.Coral` to `theme.Accent`; leave `hl` (blue), `border`, and `info` as-is.
- [x] 2.3 `internal/picker/picker.go`: change `--pointer "▌"` to `theme.CursorGlyph`.
- [x] 2.4 `internal/picker/picker.go`: confirm `colorOf` still returns `theme.Coral` for tmuxp (type
  color unchanged) and that no other coral chrome remains.
- [x] 2.5 `internal/cli/cli.go`: change `infoStyle` from `theme.Coral` to `theme.Accent`.

## 3. Write the design language down

- [x] 3.1 Add a top-level `DESIGN.md`: palette + per-color semantics, the accent, the selection
  glyph, the status glyph+word vocabulary, the layout philosophy (fixed columns, session bars,
  status gutter), degradation rules (no color-only signals; verify nerd-font glyphs in a raw
  terminal), customizability (`agents.accent`), and the rule that all visual tokens come from
  `theme`.
- [x] 3.2 Add a repo `CLAUDE.md` (or section) with a short pointer to `DESIGN.md` so future work
  follows the language.

## 4. Enforce + verify

- [x] 4.1 Add a guard `go test` (e.g. `internal/theme/guard_test.go` or a top-level test) that walks
  non-`theme` Go sources and fails on hardcoded color literals (`#rrggbb`, `lipgloss.Color("#...")`,
  color SGR escapes) and on the raw selection glyph outside `theme`, with a failure message naming
  the file:line and the rule.
- [x] 4.2 `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` all clean.
- [x] 4.3 Manual check: run the picker and `be agents` in a raw terminal and confirm both show the
  same magenta accent and the same `󰁕` selection pointer; confirm tmuxp candidates are still coral.
