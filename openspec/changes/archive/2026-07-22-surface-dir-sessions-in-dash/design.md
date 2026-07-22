## Context

Two shortcomings, sharing a single root: session identity.

1. **Naming.** Session names for directory-rooted create candidates are constructed in two
   places with two schemes. Repos use `dir.HomeSession(gitCommonDir)` = `<repoBase>-<hash6>`
   (`internal/providers/dir/dir.go:121`), deliberately keyed on the git-common-dir so a repo's
   home session is stable across worktrees and matches what `fleet.Spawn`/`openWindow` mint
   (`internal/fleet/fleet.go:258,328`). Plain directories use bare `SessionName(base)`
   (`dir.go:62`). Bare names collide: the registry dedups by `Name` and drops the second
   colliding candidate (`internal/provider/registry.go:59`), so a second `v3` is not merely
   mis-rooted - it is invisible in the picker. The lossy sanitizer (`.`/`:`/space → `_`) adds a
   second collision channel.

2. **Layering.** `HomeSession` lives in the picker provider `providers/dir`, yet the core
   packages `agents`, `fleet`, and `cli` all import that provider solely to call it
   (`workspace.go:3`, `view.go`, `fleet.go:18`, `cli.go:17`) - a leaf provider that core
   depends on.

3. **Visibility.** `Workspace.Rows()` (`internal/agents/workspace.go`) recognizes only
   repositories (from panes/agents resolving to a git-common-dir) and live agents. A tmux
   session rooted at a non-git directory with no agent matches neither and is dropped, so
   `be dash` never shows a directory `be sessions` will happily open.

## Goals / Non-Goals

**Goals:**
- One deterministic, pure source of truth for session names, importable by core without a
  layering inversion.
- Distinct directories never collide into one candidate.
- An open non-git directory session appears in the dash as a jump-only row.

**Non-Goals:**
- Changing repo home-session names or spawn/attach unity (behavior is preserved, only its
  home moves).
- Surfacing dormant (unopened) directories in the dash - that stays the picker's job.
- The directory/worktree provider **overlap** fix (a zoxide repo entry offered under two
  names). Real, but tracked as a linked follow-up; see Open Questions.
- A `SessionSpec`-on-`Candidate` + registry-resolution mechanism. Rejected below.

## Decisions

### D1. A pure `internal/sessname` package, not a registry-resolved `SessionSpec`

Extract naming into `internal/sessname`:

```
Sanitize(base string) string            // current dir.SessionName folding
For(base, identity string) string       // Sanitize(base) + "-" + sha256(identity)[:6]
Home(gitCommonDir string) string        // For(repoBase(gitCommonDir), gitCommonDir)
```

`worktree` and `fleet` call `Home(gitCommonDir)` (unchanged output); `dir` calls
`For(base, cleanPath)`. `dir.HomeSession` becomes a thin wrapper or is removed in favor of
`sessname.Home`.

*Why not the earlier `SessionSpec{Base,Identity,RootDir,Window}` resolved by the registry:*
the consumer that must agree with the picker's names is `fleet`, which **bypasses the
registry** entirely and calls `HomeSession` directly. Resolving a spec in the registry would
centralize `dir`+`worktree` while leaving the one seam that actually matters - picker-open vs
dash-spawn landing in one session - resting on the shared function exactly as today. It also
bifurcates `Candidate` into "spec, resolved later" vs "opaque Action" (and `tmuxp` *cannot* be
spec'd - it discovers its session name post-load, `tmuxp.go:72`), adding a parallel mechanism
without retiring the old one. A pure function is the real shared authority; make it importable
and correct, and both consumers agree by construction.

### D2. Identity is an explicit argument, not the rooting dir

The value hashed differs from the value the session roots at: for a repo, identity =
git-common-dir, root = primary worktree path; for a plain dir they coincide (= the path).
`For(base, identity)` takes identity explicitly, so callers pass the right one. This preserves
the load-bearing invariant that a repo opened via the picker and a repo spawned into from the
dash produce the **same** home session (else spawn's "never inject into a user session" guard,
`representativePane`'s home preference, and `locatorHint`'s home check all split).

### D3. Determinism preserved; no collision-time renaming

The name is a pure function of `(base, identity)` - never of the live session or candidate
set. Rejected: suffix-only-on-collision and Ensure-time "v3 exists elsewhere, use v3-2"; both
make the name depend on live state, breaking the determinism the current `HomeSession` design
relies on and any future name→identity recomputation.

### D4. `RowDir` as a subordinate fallback in `Workspace.Rows()`

Add `RowDir` to `RowKind` (`agent.go`). In `Rows()`, during the existing pane loop, track per
session whether any pane resolved to a repository and whether any pane hosts an agent. After
repo sections and incidental-agent rows are emitted, each session with panes, **no** repo hit,
and **no** agent contributes exactly one `RowDir`:
- `SessionID` = synthetic `"dir:"+session` (stable across refreshes for selection following),
- `Dir` = the representative pane's `StartPath` (lowest window index, then lowest pane -
  deterministic; documented as a heuristic since tmux has no session-level "home path"),
- `TmuxSession`/`TmuxPane` set so jump and preview work.

The "no repo pane / no agent" gate guarantees zero overlap with existing rows. Verb gating
keys on the already-existing predicates: `isManagedWorktree()` is already false when
`Worktree == ""` (`agent.go:162`), so delete/worktree ops no-op naturally; spawn (`n`) gets an
explicit "not a git repository" notice rather than a silent no-op.

### D5. Rendering

`RowDir` renders in the Projects lens as a lightweight one-row entry with a distinct glyph and
the directory basename (path in the detail column). Per DESIGN.md, the glyph and any color come
from `internal/theme` (a new token), never hardcoded; it must differ from the anchor `⌂` and
slot `◌` glyphs, which carry git-specific meaning. Judge the glyph by `cat`-ing a sample in a
raw terminal, and verify the rendered frame with the use-tty skill, not unit tests alone.

## Risks / Trade-offs

- **Session name gains a hash suffix** (`v3` → `v3-9f2a1c`) in the tmux status line. → Accepted
  already for repos; the picker Label still shows the friendly path. Uniformity and collision
  safety outweigh the cosmetic cost.
- **Representative-dir heuristic** (window-0/pane-0 start path) mislabels a hand-rolled session
  whose first window was later `cd`-ed elsewhere. → Acceptable first cut; document the
  heuristic in a comment. It is correct for `be sessions`-created sessions, which root window 0
  at the picked directory.
- **"Any open non-git session" includes scratch/tmuxp/template sessions.** → Accepted as the
  chosen default (liveness is the scoping gate; users keep few sessions open). If it proves
  noisy, the deterministic `sessname.For` name makes a "be-made dir session" recognizer
  available later at zero storage cost, without recoupling the two features.
- **Behavior-preserving refactor touches four packages' imports.** → Output of `Home` is
  identical to `HomeSession`; a test asserting `sessname.Home(x) == old HomeSession(x)` for
  representative inputs guards the move.

## Migration Plan

Behavior-preserving for repos; additive for directories. No data migration. Existing plain-dir
sessions named `v3` are not renamed retroactively - a newly opened `v3` gets the hashed name,
and the old one attaches by its existing name until closed. Rollback is a pure code revert; no
persisted state changes.

## Open Questions

- **Directory/worktree overlap (linked follow-up).** A zoxide entry that is itself a git repo
  is offered by `dir` (as `For(base,path)`) and by `worktree` (as `Home(gitCommonDir)`) under
  different names, so registry dedup does not merge them and opening the dir-flavored candidate
  mints a second, non-home session for the repo. Fix: resolve the `dir` provider's paths
  through the same git seam the reconciler uses and name repo paths with `Home(gitCommonDir)`.
  Deferred: it adds a git resolve per candidate dir (capped by the zoxide limit, cacheable) and
  a dependency; worth its own change once `sessname` exists. **Captured as the change
  `dedup-dir-repo-overlap`.**
- **`dd` on a directory row:** close the session (symmetric with closing an incidental agent's
  window, with confirmation) vs. disabled. Leaning close-with-confirm; settle during apply.
