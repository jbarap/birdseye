## Context

Today the dash groups by tmux **session** and computes a single `managed` boolean per session:
a session is a managed repo only when *every* pane resolves to one and the same repository
(`Workspace.managedRepoOf`, `internal/agents/workspace.go`). Rows are then grouped by
`r.TmuxSession` (`groupRows`, `internal/agents/view.go`). This whole-session gate is brittle (one
stray non-git pane drops the session to plain), opaque (a non-managed session gives no reason why),
and asymmetric (a managed session shows its full worktree set even with no agents, while a non-git
session shows only a bare agent row even if it is sitting in a repo with worktrees).

The grouping axis is the *ephemeral* unit (the tmux session) while the meaningful unit is the
*durable* one (the repository and its worktree set). DESIGN.md already states the orthogonal row
model - agent-ness and worktree-ness are independent per row - but the session-level gate violates
it: a row's worktree-ness is hostage to its session-mates.

Recognition reads `pane_start_path` (`internal/tmux/tmux.go`), the agent hook reports `cwd` but the
persisted `Agent` record drops it (`internal/agents/claude.go` uses it only to build a title), and
per-repo session names are derived from a basename with no collision handling
(`internal/providers/dir/dir.go` `SessionName`).

## Goals / Non-Goals

**Goals:**
- Group the dash by repository; make tmux sessions an implementation detail.
- Make recognition a per-pane / per-agent union rather than a whole-session gate, so a repo lights
  up whenever it has any live presence and stray panes never poison it.
- Keep actions unambiguous by routing every spawn to a deterministic per-repo `be-<repo>` home.
- Confine be's mutations to sessions it named; observe everything else read-only.
- Remove the `managed` boolean and the notion of "managed sessions" as tracked state. Recognition
  stays fully derived each tick.

**Non-Goals:**
- Discovering repos on disk that have no live presence (no filesystem scan; recognition, not
  discovery). A repo with zero panes and zero agents stays invisible.
- Owning or persisting orchestration state. The only durable signal is the `be-` session name.
- A dual-pane / master-detail layout. The single grouped list is retained; only its grouping axis
  and recognition change.
- Adopting or migrating a user's hand-made session into a `be-` session.

## Decisions

### D1: Group by repository, not tmux session
The render unit becomes the repository (keyed by `git-common-dir`). A section aggregates every row
whose presence resolves to that repo, regardless of which tmux session the row's pane lives in.

*Alternatives considered.* (a) Keep session grouping but recognize per-window instead of
per-session - rejected because under the user's "one session per repo" convention session and repo
coincide, so per-window-within-session adds the same complexity without the durable-unit payoff, and
still can't unify a repo whose worktrees span sessions. (b) Per-row interface with no grouping
(every worktree/agent a flat row, repo shown as a column) - rejected as higher cognitive overhead;
information for one repo would scatter across the list.

### D2: Recognition = start_path UNION active-agent cwd
A repo is recognized when any pane's `start_path` **or** any active agent's `cwd` resolves into it
(via git-common-dir). `start_path` is kept as the spine because it is stable and correct for windows
be creates; an agent's `cwd` covers auto-`cd` workflows (e.g. tmuxp) where a pane is born outside
the repo and then moved in. An agent's `cwd` is durable (its worktree), so it does not flicker the
way a wandering shell's current path would.

*Alternatives considered.* Switching wholesale to `pane_current_path` - rejected: the dash would
churn as the user `cd`s around, and an agent that momentarily `cd`s out would misfile itself. The
union gets the stability of start paths with coverage of the auto-cd case.

*Consequence.* The persisted `Agent` record must carry `cwd` (it is dropped today).

### D3: `be-<repo>` sessions are be's write domain
be authors windows and panes only in sessions it named. Every other session is read-only: be
recognizes repos in it and monitors agents, but never injects or removes windows there. Spawning
from a section whose only presence is a user-made session creates the repo's `be-` home instead of
touching the user's session. This is the relocation of "managed": not a tracked flag, but a rule
about *where be is allowed to write*. It is the cleanest expression of recognition-over-ownership -
be owns exactly what it created.

### D4: Per-repo session name keyed on repo identity, collision-disambiguated
The spawn home is a deterministic function of the repository's `git-common-dir`: `be-<basename>`
normally, disambiguated on collision (e.g. a short hash of the git-common-dir appended) so two repos
sharing a basename get distinct homes. Determinism is load-bearing: pressing the new-agent key on a
repo must always reach the same session, and `Ensure` reuses it if present.

*Alternative considered.* `be-<basename>` alone - rejected: two repos named `api-proxy` would silently
share a session and break the model. DESIGN.md already refuses ambiguous basename handles.

### D5: One row per agent, rendered flat
A worktree is no longer 1:1 with a row. Two agents in one worktree render as two rows with the
worktree label repeating; a worktree with no agent renders as a single base/slot row. The two-level
tree (section -> row) is preserved - no third indent level. Row identity becomes: an agent (by pane)
is a row; a worktree with no agent is a row.

*Alternative considered.* Folding co-worktree agents into one row (preferring the agent pane) -
rejected by the user: distinct agents are distinct units of work and must each be visible and
addressable.

### D6: `dD` guarded to the sole occupant
Removing a worktree is permitted only when the target is the only row of that worktree. With two
agents in a worktree, `dD` is refused with a notice, forcing the user to close the co-tenant first.
This replaces a multi-agent warning modal with a simple refusal and avoids any silent co-kill, while
`dd` (close one window) stays instant and reversible.

### D7: Cross-session aggregation with a locator hint
A row that lives outside its repo's `be-` home carries an `[in: <session>]` hint, so the user can
see when an entry is not in the managed home. Jump/attach may therefore land in different sessions
from one section; this is accepted on purpose, since the user navigates repos, not sessions. When a
worktree has windows in more than one session, the row resolves to a deterministic pane: prefer the
agent-bearing pane, then the `be-` session's pane.

## Risks / Trade-offs

- [One section spans multiple tmux sessions, so jump lands in different sessions] -> Surface the
  `[in: <session>]` hint (D7) so the divergence is visible rather than surprising.
- [Switching recognition off the session gate is a broad reconciler rewrite] -> Acceptable per the
  user's direction to disregard migration cost; the existing per-worktree, prefer-agent-pane
  indexing in `managedRows` is largely reusable.
- [Persisting `cwd` widens the agent record / hook contract] -> Additive field; the data already
  arrives at the hook, so no new collection path.
- [Repo vanishes when its last anchoring pane dies, taking windowless slots with it] -> Intended
  recognition-not-discovery behavior, identical to today's managed session disappearing; document it
  so it does not read as "my worktrees were lost."
- [Two agents in one worktree breaks the worktree-is-the-row invariant DESIGN.md asserts] -> Amend
  DESIGN.md (D5); the two-level tree is preserved.

## Migration Plan

1. Promote `cwd` onto the persisted `Agent` record and the hook ingestion path.
2. Replace `managedRepoOf` / `managed` with repo-keyed recognition (start_path UNION agent cwd).
3. Re-key grouping in `groupRows` from `TmuxSession` to repository identity.
4. Add collision-safe per-repo session naming; route spawn (view + `be agents spawn`) to it.
5. Apply the write-domain rule: spawn from a non-`be-` presence creates the `be-` home.
6. Render one-row-per-agent flat, with `[in: <session>]` hints; add the sole-occupant `dD` guard.
7. Amend DESIGN.md (row model) and the affected spec capabilities.

No data migration is required - nothing is persisted beyond agent state files, which gain an additive
field.

## Open Questions

None blocking. The four prior forks are resolved: start_path UNION agent cwd (D2); collision-safe
naming (D4); prefix as write-domain naming policy (D3); cross-session aggregation accepted with hints
(D7). Disambiguation encoding in D4 (short hash vs parent-dir qualifier) is an implementation detail
to settle during the naming task.
