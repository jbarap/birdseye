## Why

bird's-eye assumes repos live as `<repo>/<default-branch>` with sibling worktrees in the same
container (the "A" layout). That shape is invasive: it requires restructuring a normal clone before
the tool can touch it, breaks `cd <repo>`, and leaves the container itself not a git repo — at odds
with bird's-eye's first value, coexisting with an existing workflow. Recognition is coupled to that
exact path shape, so it cannot see worktrees placed anywhere else (including by other tools).

## What Changes

- **BREAKING:** Adopt the grouped-sibling layout. A repo is a plain clone at `<path>/<repo>`; its
  worktrees live as grouped siblings under `<path>/<repo>.worktrees/<branch-slug>`. Layout A is
  dropped with no migration.
- Recognition becomes **git-native**: a managed repo and its worktrees are derived from
  `git worktree list` and `git rev-parse --git-common-dir`, not from a path convention — so any
  on-disk layout is recognized, including worktrees a user or another tool created.
- The **anchor** is redefined as a repo's *primary worktree* (git's main worktree), branch-
  independent — no longer "the worktree whose basename equals the default branch." Because
  `git worktree remove` refuses to drop a primary worktree, the anchor stays non-deletable for free.
- The reconciler enumerates worktrees from git, so a worktree with **no open tmux window surfaces as
  a slot** (a spawn target) instead of vanishing.
- **BREAKING:** Remove `be worktree clone` (a pure passthrough to `git clone` under the new layout)
  and never ship `be worktree format` (it existed only to build the A layout).
- `be worktree add` creates worktrees under the grouped-sibling policy, slugifying the branch into
  the directory name.

## Capabilities

### New Capabilities

- (none — this change extends existing capabilities)

### Modified Capabilities

- `worktree-provider`: grouped-sibling layout policy, git-native discovery of a repo's worktrees,
  removal of `be worktree clone`, and `be worktree add` under the sibling policy.
- `agent-view`: anchor redefined as the primary worktree; slots enumerated from git, so windowless
  worktrees are visible.

## Impact

- `internal/worktree`: remove `Clone`; rework `Add`, `Resolve`, `containerOf` onto
  `--git-common-dir`; add worktree listing.
- `internal/agents/workspace.go`: `anchorOf`, `managedRows`, and the reconciler read
  `git worktree list`; add a per-repo cache invalidated on lifecycle actions.
- `internal/cli/worktree.go`: drop `clone`.
- `worktree-provider` scanning: find git repos and enumerate their worktrees rather than matching
  `<repo>/<branch>` shapes.
- Foundation for `agents-headless-cli` (handle addressing) and `dash-teardown-keys` (slots).
