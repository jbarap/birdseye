## 1. Grouping & ordering core

- [x] 1.1 Add a pure grouping helper in `internal/agents/view.go` that takes `[]Agent` and
  returns the agents in render order plus a `[]renderItem` describing the interleaved
  session headers, window headers, and agent rows (each agent item carrying its index into
  the ordered slice).
- [x] 1.2 Partition agents by `TmuxSession` (empty → a single "ungrouped" bucket) and, within
  each session, by `TmuxWindow`.
- [x] 1.3 Compute group urgency as the minimum `Status.rank()` over members; order sessions
  by urgency then session name, windows within a session by urgency then window, and the
  ungrouped bucket among sessions by its own urgency.
- [x] 1.4 Order agents within a window by status rank then title (today's order, per group).
- [x] 1.5 Apply the collapse rule: session header for every non-empty session; window header
  only for windows holding ≥2 agents; a single-agent window emits its agent inline carrying
  the window label (no window header).
- [x] 1.6 Unit-test the helper: co-located agents grouped; single-agent window collapses
  inline even when its session spans multiple windows; most-urgent session/window floats to
  top; ungrouped bucket placement; ordering within a group.

## 2. Wire grouping into the model

- [x] 2.1 Make `m.agents` hold agents in render order: run the grouping helper in `newModel`
  and in `reload`, storing the ordered slice and caching the `[]renderItem`.
- [x] 2.2 Confirm `relocate` still preserves selection by `SessionID` against the re-ordered
  slice, and that `refreshPreview` keys off `m.agents[cursor]` unchanged.
- [x] 2.3 Verify navigation (`j/k`, arrows, `gg/G`, half-page) operates over agent leaves
  only — since headers are not in `m.agents`, the cursor cannot land on a header; add/adjust
  tests asserting motions move between adjacent agents across group boundaries.

## 3. Rendering

- [x] 3.1 Replace flat `renderRows` with a walk over `[]renderItem`: emit styled session
  headers (disclosure glyph + name), indented window headers (fainter, with agent count),
  and indented agent rows.
- [x] 3.2 Render collapsed single-agent windows inline under the session header, showing the
  window label via the existing faint location column.
- [x] 3.3 Keep the selected-row marker and `selectedStyle` on agent rows only; ensure headers
  are never marked selected.
- [x] 3.4 Convey hierarchy via indentation + glyphs (not color alone) so it degrades on
  limited-color terminals; reuse the shared theme for header styles.

## 4. Adapt existing behavior & tests

- [x] 4.1 Update `TestViewWideShowsPreviewNarrowHides`, height-fit, and selection-preservation
  tests as needed so they account for header lines while still asserting the same invariants
  (preview shown/hidden, total height ≤ terminal height, selection follows its agent).
- [x] 4.2 Confirm the empty state ("No agents tracked yet") and help line are unchanged.
- [x] 4.3 Run `go test ./...` and `go vet ./...`; fix any regressions.

## 5. Indent guides, title de-duplication, and folding

- [x] 5.1 Add faint vertical indent guides (`│ ` per depth) and route agent titles through a
  `displayTitle` helper that strips a redundant `"<session>:"` prefix.
- [x] 5.2 Add an `ActionFold` keymap action (default `tab`), a `folded` session-key set, and a
  derived `nav` slice of navigable rows; make a session header navigable only when folded and
  agents navigable only when expanded.
- [x] 5.3 Drive all motions and selection off `nav`; resolve preview/jump for a folded header
  to the section's first agent; preserve selection by row identity (agent id or folded header)
  across refresh and prune fold state for vanished sessions.
- [x] 5.4 Render folded sections as a single `▸ name (N)` header; show `tab fold` in the help
  line; unit-test fold/unfold, fold persistence across refresh, and `displayTitle`.
- [x] 5.5 Re-run `go test ./...` and `go vet ./...`; visually verify guides, de-duplicated
  titles, and folding via a raw-terminal snapshot.

## 6. Window names, no agent-count, and section jumps

- [x] 6.1 Capture `#{window_name}` in `resolveTmux`; persist it as `tmux_window_name` on the
  record and `TmuxWindowName` on `Agent` (additive, display-only — targeting keeps the index).
- [x] 6.2 Add `windowLabel(index, name)` (name → `win <index>` → empty) and use it for window
  headers and inline collapsed-window labels; drop the `(n agents)` count from window labels.
- [x] 6.3 Add `ActionPrevSection`/`ActionNextSection` (default `{`/`}`) with vim-paragraph
  semantics over session sections; surface them in the help line.
- [x] 6.4 Unit-test window-name labelling (name vs index fallback) and the section-jump motions;
  re-run `go test ./...` / `go vet ./...` and verify the rendering via a raw-terminal snapshot.

## 7. Manual verification

- [ ] 7.1 With multiple Claude sessions (including two panes in one window and two windows in
  one session), run `be agents` in a tmux popup and confirm the tree renders with indent
  guides, windows show their tmux names, titles aren't session-prefixed, `tab` folds/expands a
  section, `{`/`}` jump between sessions, single-agent windows collapse inline, needs-attention
  groups float to top, and jump-to-pane still lands on the correct pane.
