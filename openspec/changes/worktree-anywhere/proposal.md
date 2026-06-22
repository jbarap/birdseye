## Why

`be worktree` forces every managed repo to live under a single configured root
(`worktree.root`, default `~/code`), and hardcodes the primary checkout as `main`.
That makes the tool dictate your filesystem layout — the opposite of bird's-eye's
"adopt the primitives without reshaping your workflow" philosophy — and breaks on
repos whose default branch is `master`/`trunk`. Worktrees should be creatable
anywhere, with the tool *recognizing* the `<repo>/<default-branch>` structure
rather than *owning* a root.

## What Changes

- **BREAKING**: drop the mandatory common root. A managed repo is a `<repo>/`
  container directory **anywhere** on disk that holds sibling git worktrees, one
  of which is the default-branch checkout (the anchor). The container is derived
  from git (`<repo>/` is the parent of `git rev-parse --show-toplevel`), needing
  no marker file and no config.
- `be worktree clone <url> [parent]` clones into `<parent-or-cwd>/<repo>/<default-branch>`.
  The second argument is the **parent** the `<repo>/` container is created under
  (default: cwd). The anchor directory is named after the repository's **actual**
  default branch resolved from `origin/HEAD` (`main`, `master`, `trunk`, …), not a
  hardcoded `main`.
- `be worktree add <name> [branch]` is run **from inside a managed repo** and infers
  the container from cwd. It creates `<container>/<name>` as a sibling worktree.
  With no `branch`, it creates a **new branch** named `<name>` off the default
  branch; if `<name>` already names an existing (local or remote) branch, it checks
  that branch out instead of erroring; an explicit `branch` argument checks out /
  bases on that ref.
- **BREAKING**: the picker's worktree provider stops scanning a single
  `worktree.root`. It enumerates an **optional** list of discovery paths,
  `[worktree] roots = [...]`. When empty, the provider yields no candidates;
  worktrees remain fully usable via the CLI (and, later, the orchestration view).
- `git` remains required for all worktree operations, reported clearly when absent
  (unchanged).

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `worktree-provider`: clone/add no longer require a common root and the layout is
  `<parent>/<repo>/<default-branch>` with real default-branch detection; `add`
  infers its container from the current directory and gains intuitive branch
  semantics; the provider enumerates configurable `roots` instead of a single root.

## Impact

- `internal/worktree/worktree.go` — `Clone`/`Add`/`List` signatures and layout
  logic; new default-branch resolution; container-from-cwd inference.
- `internal/worktree/provider.go` — `NewProvider` takes `roots []string`;
  `Candidates` scans each root (or yields nothing when empty).
- `internal/cli/worktree.go` — `clone`/`add` argument shapes and `--root` flag.
- `internal/config` — `Worktree.Root string` → `Worktree.Roots []string`.
- `internal/cli/cli.go` — provider registration wiring.
- Config files using `[worktree] root` must migrate to `roots` (**BREAKING**).
- Lays the foundation the later `agent-orchestration` change keys off (anchor
  detection depends on this structure + default-branch resolution).
