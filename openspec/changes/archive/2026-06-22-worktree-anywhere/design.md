## Context

`be worktree` currently encodes a single managed root: `Clone(root, url)` writes
`<root>/<repo>/main`, `Add(root, repo, name, branch)` writes `<root>/<repo>/<name>`,
and `List(root)` scans `<root>` for any child containing a `main` checkout. The root
comes from `--root` / `[worktree] root` / `~/code`. The picker's worktree provider
(`worktree.NewProvider(root)`) calls `List(root)` to surface candidates.

Two things are wrong with this for bird's-eye's "recognize structure, don't own it"
philosophy: it dictates *where* repos must live, and it hardcodes the primary branch
as `main`. This change makes worktrees creatable anywhere and detects the real
default branch, while keeping a purely *optional* discovery mechanism for the picker.

This is the foundation for the later `agent-orchestration` change, whose anchor
detection keys off the `<repo>/<default-branch>` structure and default-branch
resolution defined here.

## Goals / Non-Goals

**Goals:**
- Worktrees live anywhere; the `<repo>/` container is git-derived, needing no config
  and no marker file.
- `clone` and `add` produce a consistent `<parent>/<repo>/<default-branch>` + siblings
  layout, with the anchor named after the repo's actual default branch.
- `add` is ergonomic from inside the repo, with intuitive branch handling.
- The picker provider still works, via an optional list of discovery roots.

**Non-Goals:**
- No orchestration view, agent spawning, or tmux session/window management (that is
  the `agent-orchestration` change).
- No per-repo registry or frecency-based discovery — discovery stays a simple scan of
  configured roots.
- No automatic migration of existing `[worktree] root` configs beyond a clear error.

## Decisions

### Container is the parent of `git rev-parse --show-toplevel`

`add` infers everything from cwd: `git rev-parse --show-toplevel` gives the current
worktree directory; its parent is the `<repo>/` container; new worktrees are siblings
inside it. No marker file, no config, works from any subdirectory of any worktree.

*Alternative considered:* a sentinel file (`.birdseye`) marking the container —
rejected as ceremony that re-introduces tool ownership of the layout.

### Default branch resolved from `origin/HEAD`

The anchor directory and `add`'s default base branch come from the repository's real
default branch, resolved via the remote's `HEAD` (e.g. `git ls-remote --symref origin
HEAD` at clone time, or `git symbolic-ref refs/remotes/origin/HEAD` afterward). When
it cannot be resolved (no remote), fall back to the checked-out branch of the anchor.

*Alternative considered:* keep hardcoding `main` — rejected; breaks `master`/`trunk`
repos and would mis-detect the anchor in orchestration.

### `clone <url> [parent]`: second arg is the parent

`<parent>` defaults to cwd; the result is always `<parent>/<repo>/<default-branch>`.
This mirrors `git clone <url> [dir]` ergonomics while preserving the two-level
container layout. Re-clone is a no-op that reports the existing layout.

### Provider takes `roots []string`, not a single root

`NewProvider(roots []string)` scans each root for the `<repo>/<default-branch>`
structure. Empty ⇒ no candidates, no error. `List` is generalized to operate over a
set of search paths and to recognize a container by "has a child that is the
default-branch checkout" rather than "has a child named `main`".

*Alternative considered:* drop standalone enumeration and surface only live tmux
worktrees — rejected for now; it removes "attach to a not-yet-open worktree," which
is the provider's whole point.

### `add` branch semantics

- no branch arg, `<name>` is not an existing branch → create new branch `<name>` off
  the default branch.
- `<name>` matches an existing local/remote branch → check it out.
- explicit `branch` arg → use it as given (existing ref or `-b` as git decides).

## Risks / Trade-offs

- **BREAKING config change** (`root` → `roots`). → On encountering the old `[worktree]
  root` key, fail with a clear message naming the new `roots` key rather than silently
  ignoring it.
- **Default-branch resolution can fail** (no remote, detached `origin/HEAD`). → Fall
  back to the anchor's checked-out branch; never block the operation on detection.
- **`add` ambiguity** when `<name>` collides with a real branch the user didn't mean.
  → Document the precedence (existing branch wins) and keep an explicit `branch` arg
  as the escape hatch.
- **Container inference depends on git** — running `add` outside a worktree must fail
  cleanly with guidance, not a confusing git error.

## Open Questions

- Should `clone` also accept a flag to pick a non-default anchor branch? Deferred —
  default-branch detection covers the common case; revisit if asked.
