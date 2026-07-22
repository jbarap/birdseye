## Why

The directory provider and the worktree provider overlap: a `zoxide`/roots entry that is itself
a git repository is offered by both - `dir` names it `sessname.For(base, path)`, `worktree`
names it `sessname.Home(gitCommonDir)`. The names differ, so the registry's name-based dedup
does not merge them; both appear in the picker, and selecting the directory-flavored candidate
mints a second, non-home session for the repository (the exact split `sessname.Home` exists to
prevent). The dash then papers over it with an `[in: ...]` locator.

This is the linked follow-up noted in `surface-dir-sessions-in-dash` (see that change's
design.md Open Questions); it was deliberately deferred so the naming extraction could land
first.

## What Changes

- The `dir` provider resolves each candidate path through the same git seam the reconciler uses
  (a `RepoResolver`). When a path is inside a git worktree, it names the candidate
  `sessname.Home(gitCommonDir)` - matching the worktree provider - so the two candidates share a
  name and the registry merges them. Otherwise it keeps `sessname.For(base, path)`.
- Result: a repo reachable via both zoxide and a configured repo root appears once, and opening
  it always lands in the repository's home session rather than a second directory-named one.

## Capabilities

### New Capabilities
<!-- None. -->

### Modified Capabilities
- `session-providers`: the directory / zoxide provider classifies each path as git or non-git
  and names git paths by the repository home, so a directory candidate that resolves to a
  repository deduplicates against the worktree provider's candidate.

## Impact

- `internal/providers/dir`: gains a git-resolution dependency (a `RepoResolver` seam) and a
  per-candidate resolve at enumeration (bounded by the zoxide limit; cacheable).
- `internal/cli`: wires the resolver into `dir.New` when constructing the provider.
- No change to `sessname` (already provides both `For` and `Home`).
