## Why

`be sessions` happily opens a plain (non-git) directory as a tmux session, but `be dash`
never shows it: the reconciler recognizes only git repositories and live agents, so a
directory session with no repo pane and no agent is silently dropped. Birdseye opens a place
it then refuses to show you. Separately, the directory provider names those sessions by bare
basename, so two directories sharing a basename (e.g. `~/a/v3` and `~/b/v3`) collide - the
registry dedups by name and drops the second candidate from the picker entirely.

## What Changes

- Extract session-name construction into a pure `internal/sessname` package
  (`Sanitize`, `For(base, identity)`, `Home(gitCommonDir)`), removing the current layering
  inversion where core packages (`agents`, `fleet`, `cli`) import the picker provider
  `providers/dir` solely to call `HomeSession`.
- The directory provider names each session `For(base, cleanPath)` = `<base>-<hash6>`, hashing
  the directory path. This fixes the name collision (and the second-candidate silent drop) and
  the lossy-sanitize collision (`v.3` / `v:3` / `v 3` all mapping to `v_3`).
- The dash reconciler surfaces an **open** tmux session that has no repo pane and hosts no
  agent as a single jump-only row (a new `RowDir` kind). It carries the session's directory,
  supports Enter (jump) and preview, and offers no worktree/spawn actions (it is not a git
  repository). Dormant (closed) directories are not surfaced - they remain the picker's job.
- **Follow-up (noted, not in this change):** the directory and worktree providers overlap - a
  zoxide entry that is itself a git repo is offered by both under different names, so opening
  the directory-flavored candidate mints a second, non-home session for the repo. Resolving the
  directory provider's paths through the git seam (using `Home(gitCommonDir)` when a path is
  inside a worktree) would merge them via registry dedup. Tracked as a linked follow-up.

## Capabilities

### New Capabilities
<!-- None. internal/sessname is an implementation package, not a user-facing capability. -->

### Modified Capabilities
- `session-providers`: the directory / zoxide provider names sessions with a stable
  path-derived hash suffix rather than the bare directory basename, so distinct directories
  sharing a basename produce distinct, non-colliding candidates.
- `agent-view`: the managed-repo reconciler additionally surfaces an open tmux session that
  resolves to no repository and hosts no agent as a jump-only directory row, rather than
  dropping it.

## Impact

- `internal/sessname` (new): pure naming functions.
- `internal/providers/dir`: uses `sessname.For`; `HomeSession` moves to `sessname.Home`.
- `internal/worktree`, `internal/fleet`, `internal/agents`, `internal/cli`: import `sessname`
  instead of `providers/dir` for naming.
- `internal/agents` (`agent.go`, `workspace.go`, `view.go`): new `RowDir` kind, one recognition
  branch in `Workspace.Rows()`, view rendering + verb gating for the new row.
- No config changes. No breaking changes to the `be sessions --json` contract (candidate names
  gain a suffix; the `dir` field is unchanged).
