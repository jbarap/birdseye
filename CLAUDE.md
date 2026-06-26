# birdseye — working notes

## Design language

birdseye has a single visual language documented in [DESIGN.md](DESIGN.md). Read it before
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

## Drive the TUI — don't just reason about it

You can run birdseye's TUI in a real terminal yourself via the **use-tty** skill: build
`./bin/be`, launch it in a dedicated detached tmux session (`_birdseye_dev`), and read the
screen with `tmux capture-pane -p`. Send keys with `tmux send-keys` to exercise navigation,
folds, modals, and command actions. Use it liberally — after any change to a UI surface or an
interactive behavior, and whenever verifying a feature end to end.

Layout and wiring bugs surface here that pass every unit test: the alternate-screen layout
against the real pty size (dead gaps, clipped columns, the lens/preview split), and seams that
only connect through the production wiring (e.g. a `Muter`/orchestration interface a fake source
satisfies directly but the real `Workspace` reconciler must delegate). A green suite is not a
substitute for looking at the rendered frame. See `.claude/skills/use-tty` for the workflow, and
never touch sessions other than `_birdseye_dev`.
