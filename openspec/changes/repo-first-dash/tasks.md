## 1. Persist agent working directory

- [x] 1.1 Add a working-directory field to the `Agent` record (`internal/agents/agent.go`) and stop
  discarding the hook's `cwd` (`internal/agents/claude.go`) - persist it via the store
  (`internal/agents/store.go`)
- [x] 1.2 Surface the agent working directory in the headless JSON contract (`be agents list/status`)
  as an additive field
- [x] 1.3 Tests: hook ingestion persists `cwd`; JSON record carries the working directory

## 2. Repo-first recognition in the reconciler

- [x] 2.1 Replace `managedRepoOf` / the `managed` whole-session gate with per-repo recognition keyed
  on `git-common-dir` (`internal/agents/workspace.go`)
- [x] 2.2 Build recognition from the union of pane `start_path` and active-agent working directories;
  a repo is recognized when any one of them resolves into it
- [x] 2.3 Drop the `Managed` boolean from the row model; row identity becomes "one row per agent (by
  pane)" plus "one row per agentless worktree (base/slot)"
- [x] 2.4 Tests: stray non-git pane no longer suppresses a repo; agent-cwd anchors a repo whose pane
  started elsewhere; panes spanning two repos yield two sections; a repo with no live presence is not
  shown

## 3. Group the view by repository

- [x] 3.1 Re-key `groupRows` from `r.TmuxSession` to repository identity (`internal/agents/view.go`)
- [x] 3.2 Render one row per agent flat (worktree label repeats); keep the two-level tree
- [x] 3.3 Add the `[in: <session>]` locator hint for rows outside the repo's `be-` home; deterministic
  pane resolution (prefer agent-bearing pane, then `be-` home pane) when a worktree spans sessions
- [x] 3.4 Tests: two agents in one worktree render as two rows; out-of-home row shows the hint

## 4. Collision-safe per-repo session naming

- [x] 4.1 Derive the `be-<repo>` home session name from `git-common-dir` with collision
  disambiguation (`internal/providers/dir/dir.go`, `internal/tmux/tmux.go`,
  `internal/worktree/provider.go`)
- [x] 4.2 Tests: two repos sharing a basename resolve to distinct stable names; ensuring the same
  repo twice resolves to one session

## 5. Write-domain spawn routing

- [x] 5.1 Route new-agent (view) and `be agents spawn` to the repo's deterministic `be-` home,
  ensuring-or-reusing it; never inject windows into a user-made session (`internal/fleet/fleet.go`)
- [x] 5.2 Tests: spawn creates/reuses the `be-` home; spawn from a repo whose presence is only a user
  session still routes to the `be-` home

## 6. Sole-occupant delete guard

- [x] 6.1 Guard `dD` / `be agents delete` to refuse when the worktree has more than one row, with a
  notice; preserve the clean/dirty/force and anchor-refusal behavior for sole occupants
- [x] 6.2 Tests: delete refused with a co-tenant present; delete proceeds for a sole occupant

## 7. Documentation

- [x] 7.1 Amend DESIGN.md: replace whole-session managed-repo recognition with repo-first grouping;
  soften "the worktree is the row" to "an agent is a row; an agentless worktree (base/slot) is a row"
  while keeping the two-level tree; document the `be-` write-domain rule and `[in: <session>]` hint
- [x] 7.2 Update README if it describes the managed/non-managed session distinction
