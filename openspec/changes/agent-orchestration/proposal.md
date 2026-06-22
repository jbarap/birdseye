## Why

`be agents` is a flat, read-only list of *live* agents. It cannot see a repo that
could host an agent but doesn't yet, and it offers no way to spin one up or tear one
down. The common agentic workflow — one base repo, N parallel agents, each in its own
git worktree — has to be driven by hand (create worktree, new tmux window, launch
Claude, later remove all three). bird's-eye already owns the primitives; it should
*recognize* this structure and add the orchestration verbs on top, without becoming a
session manager that owns state. This builds directly on `worktree-anywhere`'s
`<repo>/<default-branch>` layout and default-branch detection.

## What Changes

- **Recognition, not ownership.** Every refresh, the view reconciles three cheap
  inputs into a Workspace snapshot — live tmux structure (sessions/windows/panes with
  `pane_start_path`), the existing agent hook records, and *cached* per-repo git facts
  — and recognizes which sessions are managed repos. Nothing is persisted; the snapshot
  is re-derived each tick, the same way liveness GC already works.
- **Anchor rule.** A session is a managed repo iff at least one of its windows started
  in a `<repo>/<default-branch>` directory; the lowest-index such window is the
  canonical anchor. The main checkout is the gate (no anchor ⇒ not managed); other
  windows started under the same `<repo>/` container are its managed worktrees.
- **Orthogonal rows.** Agent-ness and worktree-ness are independent. A window is shown
  iff it **has an agent**, OR is a **managed worktree**, OR is the **anchor**; a plain
  window with neither stays hidden, so non-managed sessions render exactly as today.
- **New row kinds, same grid.** Managed sessions get a managed indicator (the worktree
  glyph + worktree count) in their session bar, an `⌂ base` anchor row, and a worktree
  row per worktree window — whose pinned status gutter shows the agent's status, or
  `◌ slot` when the worktree has no agent. No new indent level; the worktree is the row.
- **`n` — new agent.** From a managed repo, prompt for a worktree name, create the
  worktree (`worktree.Add`), open a tmux window rooted there, and spawn the configured
  agent command.
- **`d` — delete agent.** Remove the agent; for a managed worktree also `git worktree
  remove` it, guarded by a clean/dirty check surfaced as an in-view confirm (force or
  cancel) when dirty. For an incidental (non-worktree) agent, just kill/forget it — no
  worktree removal.
- **Configurable** refresh interval (`[agents] refresh`, default `1s`, today hardcoded)
  and agent spawn command (`[agents] command`, default `claude`).
- New tmux backend operations: enumerate structure with start paths, create a window
  with a start directory and command, kill a window, kill a session.

## Capabilities

### New Capabilities
<!-- none; this extends the existing agents view and tmux backend -->

### Modified Capabilities
- `agent-view`: recognize managed repos from a per-tick Workspace snapshot; render
  managed-repo rows (anchor, worktree, `◌ slot`) and a managed session-bar indicator;
  add the `n` (new agent) and `d` (delete agent, with worktree dirty-guard) actions;
  pin the anchor first and slots last within a managed session; make the refresh
  interval and agent command configurable.
- `tmux-backend`: enumerate sessions/windows/panes with each pane's start path; create
  a window rooted at a directory running a command; kill a window; kill a session.

## Impact

- `internal/agents/` — new Workspace reconciler beside `ClaudeSource`; new `renderItem`
  kinds (anchor, worktree/slot) and managed session bar; new `Action`s + key handling
  and the modal dirty-confirm; ordering tweak for managed sessions.
- `internal/tmux/tmux.go` — structure enumeration (`pane_start_path`) and lifecycle
  methods (new-window, kill-window, kill-session).
- `internal/worktree/` — a `Remove(dir)` with clean/dirty detection (the guard).
- `internal/config` — `[agents] refresh`, `[agents] command`.
- `internal/theme` — `slot`/`base` glyph tokens (kept in `theme` per the guard test).
- `DESIGN.md` — record the recognition-not-ownership principle, the orthogonal row
  model, and the worktree-as-row layout.
- `README.md` — document managed repos, `n`/`d`, and the new config keys.
- Depends on `worktree-anywhere` (the `<repo>/<default-branch>` structure + default
  branch resolution it introduces).
