# bird's-eye — working notes

## Design language

bird's-eye has a single visual language documented in [DESIGN.md](DESIGN.md). Read it before
touching any UI surface (the picker, the agents view, CLI chrome).

Key rules:

- All colors and selection glyphs come from `internal/theme` — never hardcode a hex, a
  `lipgloss.Color("#...")`, a color SGR escape, or the selection glyph in another package. A guard
  test enforces this; add new tokens to `theme` and reference them.
- `theme.Accent` (magenta) is the one tool-wide selection/brand accent. `theme.Coral` is only the
  tmuxp candidate-type color. Status and type palettes are context-scoped; the accent is not.
- `theme.CursorGlyph` (`󰁕`) is the shared selection pointer (agents cursor + picker `--pointer`).
- Never rely on color alone — pair it with a glyph, word, or shape.
- Judge nerd-font glyph rendering by `cat`-ing a sample in a raw terminal, not inside the TUI.
