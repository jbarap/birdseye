## 1. tmux backend (internal/tmux/tmux.go)

- [x] 1.1 Add structure enumeration returning, per pane, session/window-index/window-name/
  pane-id/start-path (via `list-panes`/`display-message` with `#{pane_start_path}`);
  empty result when no server is running.
- [x] 1.2 Add `NewWindow(session, dir, command)` (new-window `-c dir` running command),
  `KillWindow(target)`, and `KillSession(name)`, routed through the injectable runner.
- [x] 1.3 Tests for enumeration parsing and lifecycle ops via `NewWithRunner`.

## 2. Worktree removal (internal/worktree)

- [x] 2.1 Add `Remove(dir, force)` (runs from the repo's main worktree, resolved via
  `--git-common-dir`) plus `IsDirty(dir)`; git refuses a dirty removal without force, so
  the caller can prompt force/cancel.
- [x] 2.2 Tests for clean removal and the dirty-needs-force signal.

## 3. Theme tokens (internal/theme)

- [x] 3.1 Add `◌ slot` / `⌂ base` gutter tokens and the `󰘬` managed indicator beside the
  existing status glyphs in `view.go` (the guard test polices colors + the cursor glyph,
  not status glyphs); their color comes from `theme.Gray`, so no new `theme` color is
  needed and the guard stays green.

## 4. Workspace reconciler (internal/agents)

- [x] 4.1 Add a per-repo git-facts cache keyed by git common-dir (default branch,
  worktree set), resolved on miss, dirtiness computed on demand (not per tick).
- [x] 4.2 Add the reconciler that builds a `Workspace` snapshot from tmux enumeration +
  agent records + the cache: classify each window by start path (anchor / managed
  worktree / plain), apply the anchor rule (≥1 at `<repo>/<default-branch>`,
  lowest-index canonical, main as gate), and the visibility predicate
  (has-agent OR managed-worktree OR anchor).
- [x] 4.3 Unit tests: recognition (anchor present/absent, second main, cd inside),
  orthogonal classification, visibility predicate, and re-derivation (no persistence).

## 5. View model & rendering (internal/agents/view.go)

- [x] 5.1 Extend `renderItem` with anchor (`⌂ base`) and worktree/slot row kinds; render
  them in the existing fixed-column grid (no new indent level); status gutter shows
  agent status or `◌ slot`.
- [x] 5.2 Render the managed indicator (worktree glyph + count) in managed session bars;
  show the bar even when a managed repo has no live agents.
- [x] 5.3 Ordering for managed sessions: anchor pinned first, worktrees by status rank,
  `◌ slot` rows last; plain sessions unchanged.
- [x] 5.4 Drive the view from the Workspace snapshot instead of a bare `[]Agent`,
  preserving selection-across-refresh and fold behavior.

## 6. Orchestration actions (internal/agents)

- [x] 6.1 Add `ActionNewAgent` (default `n`) and `ActionDeleteAgent` (default `d`) to the
  keymap/action infra and the help line.
- [x] 6.2 New-agent: a worktree-name text-input mode; on submit, `worktree.Add` →
  `tmux.NewWindow` rooted there → spawn `[agents] command`; available only in managed
  repos.
- [x] 6.3 Delete-agent: clean worktree removed immediately; dirty worktree enters a
  modal force/cancel confirm (default cancel); incidental agent only killed/forgotten;
  anchor not removable.
- [x] 6.4 Tests for the action gating (managed vs plain, anchor), and the modal confirm
  state transitions.

## 7. Config (internal/config)

- [x] 7.1 Add `[agents] refresh` (duration, default 1s; replaces the hardcoded
  `refreshInterval`) and `[agents] command` (default `claude`), with clear errors on
  malformed values. (`Agents.RefreshInterval()` / `Agents.AgentCommand()`; view wiring
  lands with group 5.)

## 8. Docs & verification

- [x] 8.1 DESIGN.md: the recognition-not-ownership principle, the layer split, the
  orthogonal row model, and worktree-as-row are already documented; finalize the
  concrete `◌ slot` / `⌂ base` gutter tokens in the status vocabulary once they exist in
  `theme`.
- [x] 8.2 README.md: document managed repos, the managed indicator, `n`/`d`, and the new
  `[agents]` config keys.
- [x] 8.3 `just check` green (go vet + go test ./...).
