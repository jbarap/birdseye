## Why

bird's-eye's two surfaces don't look like the same tool. `be list` is unstyled fzf — a bare
border with no title. `be agents` shows a floating `bird's-eye · agents` text line *above* an
untitled list panel, and a preview panel whose only title is a faint inline word that blends into
the captured terminal output. There is no shared notion of "a panel" or "a panel's title", so the
chrome reads as three different conventions glued together. We want one minimal, modern panel
language — titles embedded in the panel border, à la [tsm](https://github.com/adibhanna/tsm) —
applied consistently across both surfaces.

## What Changes

- Introduce a single **titled-panel** convention: a minimal rounded border in the shared border
  color with the panel's title embedded in the **top border**, left-aligned and rendered in the
  tool-wide accent color. This is the one way a panel announces what it holds.
- **`be agents`**: give the list panel an embedded `agents` title in its border; replace the faint
  inline `preview` label with an embedded `preview` title in the preview panel's border. Remove the
  floating `bird's-eye · agents` title line that sat above the frame.
- **`be list`**: render fzf inside a titled panel — an embedded `sessions` title on the fzf border
  (via fzf's border-label), colored with the shared accent — so the picker matches the agents
  panels instead of looking like raw fzf.
- Panel titles are **content-descriptive** (`sessions`, `agents`, `preview`); the picker carries no
  textual brand (the prompt is a bare `❯`) — identity comes from the shared chrome.
- Add a shared helper for rendering a titled lipgloss panel so the agents list and preview panels
  build their chrome the same way (and a future panel inherits it for free).
- Add [tsm](https://github.com/adibhanna/tsm) to the README's prior-art / references list.

## Capabilities

### New Capabilities
<!-- none: this is a presentation change to two existing surfaces -->

### Modified Capabilities

- `agent-view`: the "Refined visual presentation" requirement changes from "a titled frame" (an
  unspecified title) to **titled panels** — the list panel and the preview panel each carry their
  title embedded in the panel border in the shared accent, and the floating title line and the
  faint inline preview label are removed.
- `session-picker`: the "Consistent selection chrome" requirement extends to **panel chrome** — the
  picker SHALL present fzf inside a titled panel whose title is embedded in the border and colored
  with the shared accent, matching the agents panels.

## Impact

- Code: `internal/agents/view.go` (panel rendering for list + preview; drop floating title and
  inline preview label), `internal/picker/picker.go` (fzf `--border-label`/`--border-label-pos`
  and the `label` color key), a new shared titled-panel helper (in `internal/theme` or a small
  `internal/ui`), and `internal/theme` if a token is needed for the label color (the accent already
  exists).
- Docs: `DESIGN.md` gains the titled-panel convention under its layout/selection language;
  `README.md` adds tsm to prior art.
- Tests: the `internal/theme` guard test still applies (no hardcoded tokens); panel-rendering and
  fzf-arg tests assert the title/label wiring.
- Dependencies: no new dependencies. fzf border-label is a long-standing fzf flag; gate it the same
  way the existing `--gutter` probe gates version-specific flags if needed.
- No behavior change to selection, navigation, status, ordering, or providers — presentation only.
