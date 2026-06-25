## Why

The dash currently decides per tmux **session** whether it is a "managed repo": a session
lights up with its full worktree set only when *every* pane resolves to one and the same
repository, and otherwise collapses to a single bare agent row. This whole-session, all-or-nothing
gate is brittle and opaque. One stray non-git pane drops the entire session to plain; a lone agent
running in a repo never surfaces that repo's worktrees; and a non-managed session gives the user no
signal as to why it is dark or how to change that. The user has to enter the session and guess.

The deeper issue is that the grouping axis is the *ephemeral* thing (the tmux session) while the
meaningful unit is the *durable* thing (the repository and its worktrees). "Managed" conflates two
independent questions - can I see/understand this, and can I safely act on it - and welds them into
one gate, so ambiguity in the second silently destroys the first.

## What Changes

- **BREAKING (UX): group the dash by repository, not by tmux session.** A repo section is the unit
  the view renders; tmux sessions become an implementation detail. A section aggregates rows drawn
  from any session whose presence resolves to that repository.
- **Recognition becomes a union, not a session-wide gate.** A repo surfaces when *any* pane's start
  path **or** any active agent's working directory resolves (via git-common-dir) into it. The
  whole-session purity gate and the `managed` boolean are removed.
- **`be-<repo>` sessions are be's write domain.** be authors windows and panes only in sessions it
  named; every other session is read-only and merely observed. Spawning from a section whose only
  presence is in a user-made session creates the repo's `be-` home rather than injecting into the
  user's session.
- **Collision-safe session naming keyed on repo identity.** The per-repo session name is a function
  of the git common dir (basename as the friendly part, disambiguated on collision), so two repos
  sharing a basename get distinct homes and spawning is deterministic.
- **Cross-session aggregation with a locator hint.** A row that lives outside its repo's `be-` home
  carries an `[in: <session>]` hint. Jump/attach may land in different sessions from one section.
- **One row per agent, rendered flat.** A worktree is no longer 1:1 with a row: two agents in one
  worktree show as two rows, with the worktree label repeating. The two-level tree (section -> row)
  is preserved; no third indent level is added.
- **`dD` (remove worktree) is guarded to the sole occupant.** Removing a worktree is permitted only
  when the target is the only row of that worktree; otherwise it is refused with a notice, forcing
  the user to close the co-tenant agents first. No silent co-kill.
- **Agent working directory is persisted.** The hook already reports `cwd`; it is promoted onto the
  persisted agent record so the reconciler can resolve it for recognition and grouping.

## Capabilities

### New Capabilities
<!-- None. This change reshapes existing capabilities. -->

### Modified Capabilities
- `agent-view`: replace managed-repo recognition/presentation with repo-first grouping; recognition
  by start-path-union-agent-cwd; one-row-per-agent flat rendering; cross-session aggregation with
  `[in: <session>]` hints; `be-` write-domain spawn routing; sole-occupant `dD` guard; persist agent
  cwd on the agent record.
- `tmux-backend`: collision-safe, repo-identity-keyed session naming for be-created sessions.
- `agents-cli`: expose the agent working directory in the JSON contract; spawn routes to the repo's
  deterministic `be-` home.

## Impact

- `internal/agents` (workspace reconciler, view rendering, row model, agent record, claude hook
  ingestion), `internal/fleet` (spawn routing to the per-repo `be-` session), `internal/tmux`
  (session naming), `internal/providers/dir` and `internal/worktree` (collision-safe naming).
- DESIGN.md: amend the orthogonal row model - the two-level tree holds, but "the worktree is the
  row" softens to "an agent is a row; a worktree with no agent (base/slot) is a row." Remove the
  whole-session managed-repo recognition language.
- No persistent state is introduced: recognition stays derived each tick; the only durable signal is
  the `be-` session name (a naming policy, not tracked ownership).
