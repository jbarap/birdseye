## Context

`be agents` today is a Bubble Tea TUI over `Source.Agents() []Agent`, where the only
Source is `ClaudeSource` reading hook-written records and GC-ing by process liveness.
The view groups agents into a two-level tree (session bar → agent rows), keeps the
status gutter pinned, and re-reads + re-renders every `refreshInterval` (hardcoded 1s).
It has no notion of a repo that could host an agent, and no write actions.

This change layers orchestration on top of those primitives, keying off the
`<repo>/<default-branch>` worktree structure from `worktree-anywhere`. The guiding
constraint is **recognition, not ownership**: bird's-eye never stores orchestration
state; it re-derives everything each tick from tmux + filesystem + hooks, exactly as
liveness GC already does.

## Goals / Non-Goals

**Goals:**
- Recognize managed repos from a cheap per-tick snapshot; render anchor / worktree /
  slot rows in the existing grid with no new indent level.
- `n` (new agent) and `d` (delete agent, dirty-guarded) as the orchestration verbs.
- Keep agent-ness and worktree-ness orthogonal so plain sessions are byte-identical to
  today.
- Configurable refresh interval and agent command.

**Non-Goals:**
- No persisted orchestration state, registry, or daemon.
- No multi-agent-per-worktree UI beyond reusing today's co-located-agent rows.
- No change to how status is derived (still hooks; preview still display-only).
- The `worktree-anywhere` rework itself (prerequisite, separate change).

## Decisions

### A Workspace reconciler beside ClaudeSource, not inside it

Introduce a reconciler that produces a per-tick `Workspace` snapshot from three inputs:
(1) tmux structure enumeration (new backend op, with `pane_start_path`), (2) the
existing agent records, (3) cached per-repo git facts. `Source.Agents()` stays as-is
for plain agents; the reconciler joins it with tmux + git to add anchor/worktree/slot
rows. The view consumes the snapshot instead of a bare `[]Agent`.

*Alternative considered:* widen `Source` to return a richer type — rejected; it would
force every Source (and the abstraction's "list agents" contract) to know about
worktrees. Keeping `Agents()` narrow and reconciling above it preserves the extension
point.

### Cheap tick, lazy git cache

Per tick: enumerate tmux (one call), read agent records (existing), then classify each
pane's start path by **map lookup** into a per-repo cache keyed by the git common-dir.
On a cache miss, resolve git facts once (toplevel, common-dir, default branch, worktree
list) and memoize; refresh lazily, and compute dirtiness only at delete time. No
`git status` in the hot path — that would make the 1s tick sluggish across many
worktrees.

### Anchor rule: ≥1, lowest-index canonical, main is the gate

A session is managed iff ≥1 window started in `<repo>/<default-branch>`; the
lowest-index such window is the anchor. "≥1, lowest-index" (rather than "exactly one")
keeps recognition stable when a second `main` window appears. Requiring an anchor at
all is the deliberate gate that stops an incidental `cd` into a worktree from
auto-managing an unrelated session. Classification keys on `pane_start_path`, so
`cd`-ing inside a window never changes recognition. Closing the anchor de-manages the
session gracefully — its agents keep their status as ordinary rows.

### Orthogonal rows + one visibility predicate

Two independent bits per window: *has-agent* (a live hook record) and
*is-managed-worktree* (start path under the repo container). Render a window iff
`has-agent OR is-managed-worktree OR is-anchor`; everything else (a plain shell, nvim)
stays hidden, which is why non-managed sessions look unchanged. The worktree **is** the
row: with one agent per worktree (the common case) worktree-row and agent-row collapse,
so no third indent level is added. Gutter shows agent status, or `◌ slot` when the
worktree has no agent; the anchor is `⌂ base`. New glyph tokens live in `theme`.

### `d` dirty-guard is modal (the view is a TUI, not the picker)

Unlike the fzf picker, the agents view is Bubble Tea, so the dirty confirmation is a
modal state in the model: `d` on a clean worktree removes immediately; on a dirty one
it enters a confirm state (force / cancel, default cancel) handled in `Update`. `d` on
an incidental agent only kills/forgets; the anchor is never removable. `git worktree
remove` already refuses a dirty/locked tree without `--force`, so the guard is the UI
surfacing git's own safety, not a reimplementation.

### `n` prompts for a worktree name only

`n` opens a small text-input state for the worktree name, then composes primitives:
`worktree.Add(cwd-of-anchor, name, "")` → new window rooted at the returned dir →
spawn `[agents] command`. Branch selection is deferred to `worktree.Add`'s defaults
(new branch named after the worktree); a richer branch prompt can come later.

## Risks / Trade-offs

- **Per-tick tmux enumeration cost.** → One `list-panes`-style call per tick; git is
  cached/deferred. Matches today's once-per-second cadence.
- **Cache staleness** (a worktree added/removed outside bird's-eye). → Cache keyed by
  git common-dir with lazy refresh; actions always re-resolve before mutating.
- **Modal state complicates the model.** → Confined to an explicit confirm/input mode
  in `Update`; navigation keys are inert while a prompt is open.
- **Spawn command portability** (the configured command must exist). → Surface the
  spawn failure in-view; default `claude`, overridable.
- **Orthogonality bugs** (an incidental agent mistaken for a worktree → wrong `d`). →
  The is-managed-worktree bit is path-derived and drives `d`'s behavior explicitly;
  covered by scenarios.

## Resolved (was Open Questions)

- **`n` branch selection:** v1 does *not* offer a base-branch picker — it relies on
  `worktree.Add`'s default (a new branch named after the worktree). A richer prompt can
  come later if asked.
- **`◌ slot` source:** v1 scopes rows to **live tmux windows** only (one enumeration,
  simplest). On-disk worktrees with no open window are *not* shown as slots; they are
  reached via the picker / `worktree` provider.
