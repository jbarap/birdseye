## Context

`theme` is already the single source of truth for color — `cli.go`, `picker.go`, and `view.go` are
its only consumers, and there are no hardcoded hex values outside it. What diverges is not *where*
colors come from but *which* tokens each surface picks for the same role:

- Selection/brand accent: agents uses `theme.Accent` (magenta); picker and CLI use `theme.Coral`.
- Selection glyph: agents uses `󰁕`; picker's fzf `--pointer` is `▌`.

And the design language itself is unwritten, so consistency depends on memory.

## Goals / Non-Goals

**Goals:**

- One selection/brand accent and one selection glyph across every surface.
- A written `DESIGN.md` that is the human-readable contract for the visual language.
- A mechanical guard so the "tokens come from `theme`" rule cannot silently rot.

**Non-Goals:**

- No change to candidate **type** colors (tmux=blue, tmuxp=coral, dir=gold, worktree=green) or to
  agent **status** colors — those are context-scoped palettes, not the cross-cutting accent.
- No new interaction behavior; this is presentation alignment plus documentation.
- Not turning the design language into an OpenSpec capability — `DESIGN.md` is its home. The only
  spec touched is `session-picker`, to make the picker's accent/glyph alignment verifiable.

## Decisions

### Decision: Magenta `theme.Accent` is the tool-wide selection/brand accent; coral recedes

Every surface's "this is selected / this is chrome" color becomes `theme.Accent`. `theme.Coral`
stops being the primary accent and is used only as the tmuxp candidate-type color.

- **Why:** We converged on magenta in the agents view precisely because coral collided with the red
  needs-attention status. Making it tool-wide removes the split-brain where "selected" means two
  different colors depending on the surface. Coral still has a real, non-conflicting job (a type
  hue), so it stays in the palette.
- **Alternative — keep coral primary, magenta agents-only:** rejected; that *is* the inconsistency.

### Decision: One shared selection-glyph token in `theme`

Move the agents cursor glyph out of `view.go` into `theme` (e.g. `theme.CursorGlyph`) and feed it to
fzf's `--pointer`. Both surfaces reference the one token.

- **Why:** A shared glyph is the visual half of "selection looks the same everywhere," and a single
  token is what the guard test can point at. fzf renders an arbitrary `--pointer` string, so the
  same nerd-font glyph works there.
- **Risk:** the glyph must be single-width for both lipgloss and fzf gutters (already verified for
  the agents view).

### Decision: `DESIGN.md` is the written contract; `CLAUDE.md` points to it

The language lives in a top-level `DESIGN.md` (palette + semantics, accent, selection glyph, status
vocabulary, layout philosophy, degradation rules, the theme-token rule, customizability). A repo
`CLAUDE.md` gains a short pointer so future agent/work sessions are routed to it.

- **Why:** A dedicated doc is the most discoverable form, and the `CLAUDE.md` pointer makes it part
  of the working context rather than a doc that rots unread.

### Decision: A source-scanning guard test enforces the token rule

Add a `go test` that walks the non-`theme` Go sources and fails on hardcoded color literals (a
`#rrggbb` hex, a `lipgloss.Color("#...")`, or an SGR color escape) and on the raw selection glyph
appearing outside `theme`. `/opsx:verify` confirms *this* change matches its artifacts; the guard
test is what keeps the rule true for *future* changes, since nothing re-runs verify on its own.

- **Why:** Verify is point-in-time and agent-driven; only a test in the normal `go test` path gives
  durable, automatic enforcement.
- **Trade-off:** the scan is a heuristic (string matching). It allows `theme` itself and tolerates
  non-color escapes (reset/bold/dim) by matching only color SGR forms, accepting rare false
  negatives over noisy false positives.

## Risks / Trade-offs

- [Changing the picker's long-standing coral chrome to magenta is a visible shift] → Mitigation: it
  is exactly the intended consistency; coral remains visible as the tmuxp type color, so the palette
  still feels familiar.
- [The guard test could be brittle or annoying] → Mitigation: scope it to color SGR + hex + the one
  glyph token, with `theme` exempt; keep the failure message pointing at the offending file:line and
  the rule.
