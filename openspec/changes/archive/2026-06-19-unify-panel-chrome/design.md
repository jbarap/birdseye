## Context

bird's-eye renders chrome two different ways. The agents view is a Bubble Tea / lipgloss program:
it draws a top-level title line, a rounded `frameStyle` box around the list, and (when wide) a
second rounded box for the preview whose title is a faint inline word (`previewTitle`) printed as
the first body line. The picker is not a lipgloss program at all — it shells out to `fzf`, piping
pre-colored rows and passing chrome via flags (`--border`, `--color`, `--pointer`, …). So "a panel
with a title" has no single definition; each surface improvises.

We already centralized colors and the selection glyph in `internal/theme` (enforced by a guard
test) and wrote the design language in `DESIGN.md`. This change adds the *panel* to that language:
a minimal rounded border with the title embedded in the top border, accent-colored — the look the
user likes in [tsm](https://github.com/adibhanna/tsm). lipgloss is v1.1.0, which has no built-in
border-title API; fzf is 0.72 and supports `--border-label` / `--border-label-pos` and the `label`
`--color` key (both verified against the installed binary).

## Goals / Non-Goals

**Goals:**
- One titled-panel convention applied to the agents list panel, the agents preview panel, and the
  picker, so the two surfaces read as one tool.
- Titles embedded in the panel border, left-aligned, in the shared accent; content-descriptive
  text (`sessions`, `agents`, `preview`).
- Remove the floating `bird's-eye · agents` title line and the faint inline `preview` label.
- Keep all color/glyph sourcing in `internal/theme` (guard test stays green).

**Non-Goals:**
- No change to selection, navigation, status vocabulary, ordering, folding, providers, or preview
  capture — presentation only.
- No new dependency; no port of the picker to a lipgloss TUI.
- No configurable panel titles or border styles in this change (accent is already configurable).

## Decisions

### One convention, two implementations
The picker can't share lipgloss code because fzf owns its own screen region. So the convention is
realized twice: the agents view draws the panel in lipgloss; the picker asks fzf to draw it.
`DESIGN.md` documents the single convention; each surface implements it with its own primitive. The
*look* is identical (rounded `theme.Border`, accent title); only the rendering code differs.

**Alternative considered — wrap fzf in a lipgloss panel:** draw a lipgloss border, then constrain
fzf inside it so one shared component borders every picker. Rejected. fzf takes over the terminal
and clears/redraws its whole region on each keystroke, so a border printed by our process before
launching fzf is overwritten; `--margin`/`--padding` only reserve space (which fzf then borders
itself, double-bordering). There is no way to make fzf render into a rectangle another process
drew. Achieving a single shared lipgloss panel would mean replacing fzf with an in-process
lipgloss/Bubble Tea picker — reimplementing fzf's matching, keybindings, and performance, and
contradicting the "driven by fzf" design pillar. Also moot today: fzf is the only picker (a hard
requirement, no swap-in abstraction), so shared *panel code* would pay off only once a second picker
exists, whereas the matched fzf border gives the shared *look* now. If a second picker ever lands,
each picker supplies its own border config behind a small interface.

### Agents panels: reconstruct the top border rather than splice
lipgloss v1.1.0 has no API to embed a title in a border, and overwriting characters inside the
already-rendered, ANSI-laden top line by column index is error-prone. Instead, render the box with
the existing `frameStyle`, measure its outer width with `lipgloss.Width`, then **replace the first
line** with a freshly built top border: `TopLeft + Top + " " + title + " " + Top×fill + TopRight`,
where the corner/dash runes come from `lipgloss.RoundedBorder()` (same border `frameStyle` uses),
the dashes/corners are painted in `theme.Border`, and the title in the accent (bold). Building the
line from scratch sidesteps ANSI-width math entirely. A title wider than the panel is truncated to
fit before the right corner. This becomes a small shared helper (e.g. `panel(title, body, accent,
width)`) in the agents package; both the list and preview panels call it.

### Preview panel reclaims a content row
Today the preview reserves an inner line for the inline `preview` label (`avail := inner-1`). With
the title in the border, that line is freed: the preview body uses the full inner height, and
`previewTitle` is deleted. Net: one more row of captured output, and the title no longer collides
visually with terminal text.

### Picker: fzf border-label + label color
Pass `--border-label " sessions "`, `--border-label-pos 2` (near the top-left corner, matching the
lipgloss panels), and add `label:<accent>` to the existing `--color` spec (using
`theme.Accent.Spec(tc)`, same as the other accent keys). The border itself already uses
`theme.Border`. The prompt is a bare chevron (`❯`); the panel title is content-descriptive and the
picker carries no textual brand.

### Version-gate the border label
`--border-label` exists since fzf 0.35; the `label` color key likewise. To honor the spec's
graceful-degradation requirement, reuse the existing `fzfVersionAtLeast` probe to append the
border-label flags (and the `label` color key) only when fzf ≥ 0.35, mirroring how `--gutter` is
gated at 0.72. On older fzf the picker still opens — bordered, accent-colored — just without the
embedded title. (The installed 0.72 always takes the titled path.)

### Titles are content-descriptive
Chosen with the user: `sessions` / `agents` / `preview`. The picker carries no textual brand at all
(its prompt is a bare `❯`) — identity rides on the shared chrome, keeping panels minimal.

## Risks / Trade-offs

- **Reconstructed top line drifts from lipgloss's border rendering** (e.g. a future lipgloss changes
  rounded glyphs) → Mitigation: pull the runes from `lipgloss.RoundedBorder()` rather than literals,
  so the reconstructed line tracks whatever `frameStyle` draws.
- **Title width vs. narrow panels** → Mitigation: truncate the title to the space before the right
  corner; on a panel too narrow for any title, fall back to a plain top border (no title) rather
  than overflowing.
- **fzf `label` color key / `--border-label` unsupported on old fzf** → Mitigation: version-gated;
  verified the flag and color key are accepted on the installed 0.72. Degrades to untitled border.
- **Guard test regressions** → Mitigation: the helper takes colors as parameters / reads
  `theme.Border` and `theme.Accent`; no hex, `lipgloss.Color("…")`, SGR, or glyph literals added
  outside `internal/theme`.
