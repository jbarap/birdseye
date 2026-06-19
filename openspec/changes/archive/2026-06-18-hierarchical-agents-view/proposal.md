## Why

The `be agents` view is a flat list sorted by status then title, so two Claude
instances running in the same tmux session (or even the same window) get scattered
to wherever their status sorts them, and are only told apart by an identical
`session:window` location string. When several agents are active at once, the user
cannot see at a glance which ones share a session or window — exactly the relationship
that matters for "go deal with that worktree" triage.

## What Changes

- Group the agents list into a two-level hierarchy: tmux **session** at the top level,
  **window** beneath it, with agent rows as leaves — so co-located instances are
  visually bracketed instead of scattered.
- Collapse single-occupant levels: a window sub-header appears only when a session has
  agents spread across multiple windows or more than one agent in a single window;
  a lone agent renders flat under its session with its window shown inline. This keeps
  the common single-agent case quiet.
- Order groups by their most-urgent member: the session (and within it, the window)
  containing the highest-priority agent floats to the top, preserving today's
  "needs-attention surfaced first" triage behavior at the group level. Within a group,
  agents keep status-rank ordering.
- Make navigation hierarchy-aware: with nothing folded the cursor lands only on agent rows
  (leaves), skipping headers, and existing vim motions (`j/k`, `gg/G`, half-page, arrows)
  continue to move between agents. Selection-by-identity across refreshes is preserved.
- Draw vertical indent guides so the tree structure reads clearly, and drop the redundant
  `session:` prefix from an agent's title when it duplicates its session header.
- Identify windows by their tmux window name (e.g. `nvim`) instead of a bare index, falling
  back to `win <index>` when unnamed, and drop the redundant agent-count from window labels.
  This requires the hook to also capture `#{window_name}` (display-only; targeting still uses
  the stable window index).
- Let the user fold/expand a session's section with a rebindable key (default `tab`): a
  folded section collapses to one header showing the hidden count, and its header stays
  reachable so it can be expanded again. Fold state survives live refreshes.
- Add rebindable section-jump motions (default `{`/`}`, like vim's paragraph motions) to move
  the cursor to the previous/next session.
- Agents with no tmux location (session-id only) collect under a stable "ungrouped"
  area rather than being forced into a session group.

## Capabilities

### New Capabilities
<!-- None: this changes the behavior of an existing capability. -->

### Modified Capabilities
- `agent-view`: The "Refined visual presentation" requirement gains hierarchical
  session→window grouping with single-occupant collapse, most-urgent-first group ordering,
  vertical indent guides, de-duplicated titles, and window-name labels; a new "Foldable
  sections" requirement adds collapse/expand with hidden-count headers; and the
  "Configurable, vim-native navigation" requirement is updated so motions move between agents
  and folded-section headers while skipping non-selectable headers, and gains section-jump
  motions. Default bindings and selection-preservation behavior are unchanged.

## Impact

- Code: `internal/agents/view.go` (row rendering, navigation/fold/section-jump over the
  tree, help/empty states) and its tests in `internal/agents/view_test.go`. Grouping,
  ordering, folding, and section jumps are presentation-only and live in the view; hook-written
  status, dedup, and preview capture are unaffected.
- Hook/data: the hook capture (`resolveTmux` in `internal/agents/claude.go`) additionally reads
  `#{window_name}`, persisted as a new `tmux_window_name` field on the state record and a
  `TmuxWindowName` field on `Agent`. This is additive and display-only — jump-to-pane still
  targets the stable window index; older records without the field fall back to `win <index>`.
- No new dependencies. No CLI surface or hook-config (settings.json) changes. The keymap gains
  three additive default bindings (`tab` fold, `{`/`}` section jumps); existing bindings are
  unchanged.
