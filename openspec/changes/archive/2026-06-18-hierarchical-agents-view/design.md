## Context

`be agents` (`internal/agents/view.go`) is a Bubble Tea model that renders a flat list of
`Agent`s. The source (`internal/agents/claude.go`) returns agents already deduped to one row
per tmux location and sorted by status rank then title. The view keeps a single `cursor int`
that indexes directly into `m.agents`; navigation, selection-by-identity across live
refreshes (`relocate`), and the preview pane all key off that index.

Each `Agent` already carries `TmuxSession`, `TmuxWindow`, and `TmuxPane`. So the grouping
information is present today — it is simply flattened away at render time. Two Claude
instances in one session (or one window) are only distinguishable by an identical
`session:window` location string, and status sorting can place them far apart.

This change is presentation-only: it re-orders and re-renders the same `Agent` slice into a
session→window tree. It touches `view.go` and `view_test.go`; it does not change the `Agent`
abstraction, hook-written status, dedup, or preview capture.

## Goals / Non-Goals

**Goals:**
- Render agents as a session→window tree so co-located instances are visually bracketed.
- Collapse to the minimum: session header always; window header only for windows with ≥2
  agents; single-agent windows render inline with their window label.
- Order groups by their most-urgent member, preserving needs-attention-first triage.
- Navigation lands on selectable rows only — every agent of an expanded section plus the
  header of each folded section — skipping non-selectable headers. With nothing folded this
  is exactly today's "cursor moves between agents." Preserve selection-by-identity across
  refreshes.
- Draw vertical indent guides for readable nesting, and strip a redundant `session:` prefix
  from agent titles.
- Label windows by their tmux window name (fallback `win <index>`), without redundant counts.
- Let the user fold/expand a section (default `tab`) through the existing rebindable keymap;
  fold state survives refreshes.
- Add rebindable section-jump motions (default `{`/`}`) to move between sessions.
- Keep the preview, help line, and empty state working; extend the keymap with one new action.

**Non-Goals:**
- No third grouping level (e.g. worktree/cwd) and no per-pane sub-rows beyond today's dedup.
- No window-level folding (folding is per session section).
- No change to how status is determined, to the source's dedup, or to the on-disk schema.
- No change to existing default keybindings (only the additive `tab` → fold).

## Decisions

### Cursor indexes a derived "navigable rows" slice, not the agents directly

`groupAgents` produces the full tree as `m.items []renderItem` (session headers, window
headers, agent leaves) plus `m.agents []Agent` in render order. From `items` and the fold
state the model derives `m.nav []int` — the item indices the cursor may land on. The rule:
**a session header is navigable only when its section is folded; an agent is navigable only
when its section is expanded; window headers are never navigable.** `cursor` indexes `nav`,
and all motions clamp to `len(nav)`.

This rule is what makes folding cheap. With nothing folded, `nav` is exactly the agents in
order, so `cursor` ↔ agent is identical to the pre-fold behavior and every existing
navigation test passes unchanged. A folded section contributes a single navigable stand-in
(its header), so the user can always reach a folded section to expand it — without that, a
folded section's agents would vanish from `nav` and become unreachable. `tab` toggles the
fold of the section under the cursor and then re-seats the cursor: onto the collapsed header
when folding, onto the section's first agent when expanding.

Selection-by-identity uses a row key — `"a:"+SessionID` for an agent, `"s:"+sessionKey` for
a folded header — so `relocate` follows whichever row the cursor was on across a refresh,
including a folded header. Preview and jump resolve the current row to an agent: the selected
leaf, or (for a folded header) the section's first agent, so both still target something
useful while collapsed.

Alternative considered: keeping `cursor` as a direct index into `m.agents` and never letting
it touch a header. Rejected once folding entered scope — a folded section has no visible
agent to land on, so expansion would be impossible. Making headers navigable *only when
folded* keeps the simple model for the common case while making folds reachable.

### Grouping/ordering lives in the view, computed each render (or on reload)

A pure helper takes `[]Agent` and returns both the render-ordered agent slice and the list
of "render items" (header markers + agent indices) the View iterates. It runs when the agent
list changes (on `reload` and at construction), producing `m.agents` in grouped order and a
cached `[]renderItem`. This supersedes the source's status sort for display; the source sort
becomes a harmless upstream stable order (kept, since it gives a deterministic tiebreak input
and the source is also unit-tested on it).

The grouping algorithm:
1. Partition agents by `TmuxSession`. Empty session → a single "ungrouped" bucket.
2. Within each session, partition by `TmuxWindow`.
3. Group urgency = the minimum status rank (`Status.rank()`, lower = more urgent) over its
   members. Sort sessions by urgency, tiebreak by session name; sort windows within a
   session by urgency, tiebreak by window. The ungrouped bucket sorts among sessions by its
   own urgency.
4. Within a window, sort agents by status rank, tiebreak by title — i.e. today's order,
   applied per group.
5. Emit, per session: a session header item, then for each window in order — if the window
   has ≥2 agents emit a window header item followed by its agent items; if it has exactly 1
   agent emit that agent item inline (no window header), carrying the window label so the row
   can show it. The ungrouped bucket emits its agents under an "ungrouped" header.

### Rendering: indent guides, de-emphasized headers, color-independent

Each row is `marker + indent-guides + content`. The marker (2 cols) is the cursor bar `▌`
when the row is selected (an agent or a folded header), else blank. Indent guides are faint
vertical bars `│ ` repeated per depth (session 0, window / collapsed-agent 1, windowed-agent
2), so consecutive rows at the same depth line up into continuous vertical guides. An expanded
session header shows `▾ name`; a folded one shows `▸ name  (N)` with the hidden-agent count.
Window headers sit at depth 1 with a fainter style and an agent count. Because depth and the
disclosure glyph carry the structure, the tree still reads on a monochrome terminal — the
guides are an aid, not the sole signal.

Agent titles pass through `displayTitle`, which strips a leading `"<session>:"` so a title
like `arewa:api` shows as `api` under session `arewa` (it never blanks a title that equals
the session). The inline window label for collapsed single-agent windows reuses today's faint
location column. `halfPage` continues to use terminal height and moves the cursor by `page/2`
*navigable rows*, matching the existing clamp behavior.

### Folding model

Fold state is a `map[string]bool` keyed by session key, toggled by the `tab`-bound
`ActionFold`. `recomputeNav` runs after any fold toggle and after each reload; `pruneFolded`
drops keys for sessions no longer present so the map can't grow unbounded. Folding is per
session section (not per window) — the natural "collapse this project" unit, and it keeps the
navigable-stand-in rule simple (one header per folded section).

### Window name display (new captured data)

Window labels use the tmux window name (e.g. `nvim`) instead of a bare index, since the index
carries no meaning to the user. That name was not previously captured, so `resolveTmux` now
also reads `#{window_name}`, stored as `tmux_window_name` on the record and `TmuxWindowName`
on `Agent`. The change is deliberately additive and display-only: `TmuxWindow` (the index)
still drives dedup, preview targeting, and jump-to-pane, which need a stable identifier (window
names aren't unique and change as the foreground command changes). `windowLabel(index, name)`
centralizes the fallback — name, else `win <index>`, else empty (an agent outside tmux shows no
window label). Counts are dropped from window headers (the agents are listed right below); the
folded-session header keeps its `(N)` because those agents are hidden.

### Section jumps

`{`/`}` (rebindable `ActionPrevSection`/`ActionNextSection`) move between session sections.
`sectionStarts` derives the nav indices where each session begins (contiguous runs of one
session key in `nav`). `}` goes to the next section's first row (clamping to the last row at
the end); `{` mirrors vim — to the current section's top first, then the previous section's
top — so repeated presses walk up section by section.

### Empty state and ungrouped bucket unchanged in spirit

The "No agents tracked yet" empty state is untouched. Agents with no tmux session (id-only)
were already possible and previously rendered as plain rows; now they collect under a stable
"ungrouped" heading so they don't masquerade as a session.

## Risks / Trade-offs

- [Headers consume vertical space in a short tmux popup] → Collapse already minimizes header
  count, and folding lets the user reclaim space by collapsing sections they aren't watching.
  The list still scrolls/clamps as today; preview-hiding on narrow/short terminals is unchanged.
- [Selection must follow the cursor's row across refresh, including a folded header] →
  `relocate` keys on a row identity (`"a:"+SessionID` or `"s:"+sessionKey`) found in the
  rebuilt `nav`, so the cursor follows its agent — or its folded section — across reorders.
- [Half-page distance is in navigable rows, not visual lines] → Accepted; motions are defined
  over selectable rows and this avoids coupling navigation to header layout.
- [Indent guides could be too dim or too loud] → Drawn with the shared theme's border color
  (faint) and never the sole structural signal; depth + disclosure glyphs carry the tree.
- [Source sort is now redundant for display] → Left in place; it is cheap, separately tested,
  and provides a stable input ordering. Removing it is out of scope.

## Open Questions

- None outstanding. Glyphs/styles (session `▾`/`▸`, faint `│` guides, theme border color)
  were settled during implementation; the spec only requires they be distinguishable without
  color.
