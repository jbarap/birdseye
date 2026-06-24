## 1. Git-native worktree primitives (`internal/worktree`)

- [x] 1.1 Add a repository resolver that returns the shared git dir via `git rev-parse --git-common-dir` and treats it as repo identity; replace `containerOf`'s `filepath.Dir(top)` derivation.
- [x] 1.2 Add worktree enumeration via `git worktree list --porcelain`, returning each worktree's path, branch, HEAD, and primary/linked flag.
- [x] 1.3 Add a branch-slug helper (path separators and unsafe characters → hyphens) and the grouped-sibling path policy `<path>/<repo>.worktrees/<branch-slug>`.
- [x] 1.4 Rework `Add` to create worktrees under the grouped-sibling policy, resolving the container from the shared git dir; preserve the no-branch / existing-branch / explicit-branch handling.
- [x] 1.5 Make `Add` refuse when the target slug directory already exists (slug-collision conflict) instead of overwriting.
- [x] 1.6 Remove `Clone` from `internal/worktree`.

## 2. CLI surface (`internal/cli/worktree.go`)

- [x] 2.1 Drop the `be worktree clone` subcommand and its wiring.
- [x] 2.2 Confirm `be worktree add` reports the new grouped-sibling path it created; update help text away from the `<repo>/<default-branch>` framing.

## 3. Recognition & reconciler (`internal/agents/workspace.go`)

- [x] 3.1 Replace `anchorOf`'s `info.Worktree == info.DefaultBranch` test: classify a window by resolving its start path to a git worktree + repo (shared git dir).
- [x] 3.2 Define the anchor as the repo's git primary worktree (the worktree git refuses to remove), branch-independent; pick the window started in that primary worktree as the anchor window.
- [x] 3.3 Group windows of the same repo by shared git dir into managed worktrees; recognize a session as managed iff it has a window started in any worktree of a repo.
- [x] 3.4 Join the live tmux window set with the git worktree set so windowless worktrees are emitted as slot rows.
- [x] 3.5 Add a per-repo git-facts cache (worktree set, default branch, dirtiness) keyed on shared git dir, invalidated on lifecycle actions; ensure the per-tick path is a cache lookup, not a git call.

## 4. Presentation (`internal/agents/view.go`, `internal/agents/agent.go`)

- [x] 4.1 Render the `⌂ base` anchor row against the primary worktree (not the default-branch basename).
- [x] 4.2 Render a worktree row per git-enumerated worktree, including windowless worktrees as `◌ slot` spawn targets.
- [x] 4.3 Ensure incidental (non-worktree) agent rows and plain sessions are unchanged.

## 5. Worktree provider discovery (`internal/worktree` / provider)

- [x] 5.1 Change root scanning to find git repositories under `[worktree] roots` and enumerate each repo's worktrees from git, rather than matching `<repo>/<default-branch>` shapes.
- [x] 5.2 Verify the no-roots-configured path still yields zero candidates without error.

## 6. Docs & guard

- [x] 6.1 Update README: remove `be worktree clone` usage, describe the grouped-sibling layout (`<repo>` + `<repo>.worktrees/<branch-slug>`), and revise the managed-repo recognition section (anchor = primary worktree; windowless worktrees as slots).
- [x] 6.2 Update DESIGN.md's implemented-state references (anchor = default-branch checkout; row model) to the git-native, primary-worktree definition.
- [x] 6.3 Run `just check` (go vet + go test) and fix fallout; confirm the theme guard test still passes.
