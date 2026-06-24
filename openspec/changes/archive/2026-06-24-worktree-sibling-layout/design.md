## Context

bird's-eye recognizes a "managed repo" by a path convention: a repo lives at
`<repo>/<default-branch>` and its worktrees are siblings under the same `<repo>/`
container. Two pieces of code encode that convention:

- `internal/worktree/worktree.go` — `containerOf` derives the container as
  `filepath.Dir(top)`, where `top` is `git rev-parse --show-toplevel`. So the container
  is assumed to be the *parent* of the checkout, and `add` places new worktrees there.
- `internal/agents/workspace.go` — `anchorOf` (workspace.go:132) classifies a window as
  the anchor only when `info.Worktree == info.DefaultBranch` (the checkout's basename
  equals the default branch). Everything downstream — managed/worktree/anchor row
  classification — flows from that string comparison.

The reconciler is **pane-derived only**: it builds rows from live tmux panes
(`ListPanes`) and never reads a repo's worktrees from disk. A worktree exists *to
bird's-eye* only while a tmux window is open in it; close the window and the worktree
vanishes from the view.

This shape is invasive. It requires restructuring a normal `git clone` (which produces
`<repo>/` as the checkout, not as a container) before bird's-eye will recognize it, it
breaks `cd <repo>`, and the container directory is not itself a git repo. It also blinds
recognition to any worktree placed off-convention — including worktrees other tools
create.

## Goals / Non-Goals

**Goals:**

- Recognize a managed repo and enumerate its worktrees from **git facts**, not from a
  path convention, so any on-disk layout (including third-party worktrees) is seen.
- Adopt one **grouped-sibling** creation policy: plain clone at `<path>/<repo>`,
  worktrees at `<path>/<repo>.worktrees/<branch-slug>`.
- Redefine the **anchor** as a repo's git *primary worktree*, branch-independent, and
  get "anchor is non-deletable" for free from git's own refusal to remove a primary
  worktree.
- Make the reconciler enumerate worktrees from git so a worktree with **no open tmux
  window** surfaces as a slot (a spawn target) instead of disappearing.
- Remove `be worktree clone` and never ship `be worktree format` — both existed only to
  build or passthrough the old layout.

**Non-Goals:**

- Migrating existing layout-A checkouts. Layout A is dropped with no migration path; a
  user re-clones or moves their tree by hand.
- The `dd`/`dD` teardown key semantics — those build on the slot behavior introduced
  here but are specified in `dash-teardown-keys`.
- Headless `be agents` verbs and `repo/worktree` handle addressing — specified in
  `agents-headless-cli`, which depends on this change's git-native recognition.

## Decisions

### Recognition derives from git, not from path shape

A managed repo and its worktrees are derived from `git worktree list --porcelain` and
`git rev-parse --git-common-dir` (the shared git dir, identical across every worktree of
one repo), rather than from matching `<repo>/<default-branch>`.

- *Why:* git already maintains the authoritative worktree set; reading it is layout-
  independent and recognizes worktrees bird's-eye did not create. It also makes the
  `git-common-dir` the stable repo identity, which `agents-headless-cli` reuses for
  `repo/worktree` handles.
- *Alternative considered:* keep the path convention but add more recognized shapes.
  Rejected — it multiplies special cases and still can't see arbitrary worktrees.

### Anchor = git primary worktree

The anchor is the repo's primary worktree (the one git reports first / refuses to
remove), independent of which branch is checked out there.

- *Why:* "basename equals default branch" is a brittle proxy; a primary worktree on a
  non-default branch is common and legitimate. Defining the anchor as git's primary
  worktree is exact, and because `git worktree remove` refuses to drop a primary
  worktree, "the anchor is not deletable" stops being a hand-coded special case and
  becomes a property of the substrate.
- *Alternative considered:* lowest-index tmux window as anchor. Rejected — couples a
  durable concept (which checkout is primary) to an ephemeral one (tmux window order).

### Grouped-sibling creation layout

`be worktree add <branch>` creates `<repo>.worktrees/<branch-slug>` beside the clone,
slugifying the branch into the directory name (`feature/login` → `feature-login`).

- *Why:* keeps `<repo>` a normal clone (so `cd <repo>` and existing tooling work
  untouched), groups all worktrees under one predictable sibling directory, and avoids
  nesting worktrees inside the repo (which pollutes status and ignore handling). It
  satisfies bird's-eye's first value: coexist with an existing workflow.
- *Alternatives considered:* (a) worktrees inside the repo under `.worktrees/` or
  `.claude/worktrees/` — rejected: nesting a worktree inside its own repo confuses git
  status, `.gitignore`, and editors. (b) keep layout A — rejected: it is the invasive
  shape this change exists to remove.

### Reconciler enumerates worktrees from git, with a per-repo cache

`workspace.go` joins the live tmux window set with the git worktree set per repo. A
worktree with a window renders as today; a worktree with no window renders as a slot.
Git facts (worktree set, default branch, dirtiness) are read through a per-repo cache
invalidated on lifecycle actions, so the per-tick path stays cheap.

- *Why:* this is what makes windowless worktrees visible and gives `dash-teardown-keys`
  its `dd`-leaves-a-slot behavior for free and correctly motivated. Caching keeps the
  refresh loop from shelling out to git for every window every tick.
- *Alternative considered:* enumerate worktrees only on demand (on a keypress).
  Rejected — slots must be visible passively for the view to be a spawn surface.

### Remove `clone`, never add `format`

`be worktree clone` is deleted; `be worktree format` is never shipped.

- *Why:* under the new layout `clone` is a pure passthrough to `git clone` (it earns no
  command — "only wrap what you improve"), and `format` existed solely to reshape a
  clone into layout A, which no longer exists.

## Risks / Trade-offs

- **Breaking change with no migration.** [Existing layout-A users lose recognition] →
  documented as breaking; the new layout is a plain clone plus a sibling directory, so
  re-creating it is `git clone` + `be worktree add`, no special tooling.
- **More git invocations.** [Reading `git worktree list` per repo per refresh could be
  slow] → mitigated by the per-repo cache keyed on `git-common-dir`, invalidated only on
  lifecycle actions; the per-tick classification is a cache lookup, never a git call.
- **Third-party worktrees now appear.** [A worktree another tool made shows up as a
  slot/row, which may surprise] → this is the intended "recognition over ownership"
  behavior; bird's-eye owns nothing and never mutates a worktree it didn't act on
  without an explicit user action.
- **Anchor on a feature branch.** [Defining anchor as primary worktree means the anchor
  row may not be on the default branch] → acceptable and more honest; the `⌂ base` row
  reflects the actual primary checkout rather than assuming `main`.

## Migration Plan

No data migration. This is a behavior change shipped in one release:

1. Rework `internal/worktree` onto `--git-common-dir`; add git worktree enumeration;
   remove `Clone`.
2. Rework `internal/agents/workspace.go` recognition (`anchorOf`, `managedRows`,
   reconciler) onto the enumerated worktree set with the per-repo cache.
3. Drop `be worktree clone` from `internal/cli/worktree.go`.
4. Update `worktree-provider` scanning to find git repos and enumerate their worktrees.
5. Update README/DESIGN references from the `<repo>/<default-branch>` shape to the
   grouped-sibling shape.

Rollback is reverting the release; there is no persisted state to undo.

## Open Questions

- Branch-slug collisions: if two distinct branches slugify to the same directory name
  (`feature/login` and `feature-login`), `add` must detect the existing directory and
  report a conflict rather than overwrite — confirm this reuses the existing duplicate-
  name guard.
- Whether `worktree-provider` discovery should treat the primary worktree and its
  grouped siblings as one logical repo entry in the picker or as separate candidates.
