## Why

The agents-view redesign settled bird's-eye's visual language — a magenta accent for selection and
brand, the `󰁕` selection glyph, a status glyph+word vocabulary, full-width section bars, and
fixed-column rows that degrade without color. But that language only lives in the agents view. The
picker still uses coral for its selection chrome and a `▌` pointer, the CLI uses coral for info
chrome, and nothing writes the rules down — so the next surface will drift again. We want one
documented language and the surfaces aligned to it.

## What Changes

- **Adopt the magenta accent tool-wide.** The picker's selection chrome (`hl+`, `pointer`, `prompt`,
  `marker`) and the CLI's info chrome move from `theme.Coral` to `theme.Accent`. Coral keeps its one
  remaining job: the **tmuxp candidate-type** color. There is now a single "selection/brand" accent
  across every surface.
- **Unify the selection glyph.** Promote the agents cursor glyph to a shared `theme` token and use
  it as the picker's fzf `--pointer`, so "the selected thing" looks identical everywhere.
- **Write the design language down.** Add a top-level `DESIGN.md` capturing the palette and its
  semantics, the accent, the selection glyph, the status vocabulary, the layout philosophy, the
  degradation rules, and the "all visual tokens come from `theme`" rule. Add a pointer to it from a
  repo `CLAUDE.md` so future work follows it.
- **Guard against drift.** Add a `go test` that fails if color literals or selection glyphs are
  hardcoded outside the `theme` package, making the single-source-of-truth rule enforceable rather
  than aspirational.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `session-picker`: add a requirement that the picker's selection chrome uses the shared accent
  color and the shared selection glyph (rather than its own coral/`▌`), so selection looks the same
  as in the agents view. Candidate **type** colors (including coral for tmuxp) are unchanged.

## Impact

- `internal/theme`: add a shared selection-glyph token (`CursorGlyph`); `theme.Accent` becomes the
  tool-wide selection/brand accent.
- `internal/agents/view.go`: use the shared glyph token instead of a local const.
- `internal/picker/picker.go`: selection chrome → `theme.Accent`; `--pointer` → the shared glyph.
- `internal/cli/cli.go`: info chrome → `theme.Accent`.
- New files: `DESIGN.md`, `CLAUDE.md` (pointer), and a guard test.
