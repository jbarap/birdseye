## Context

Worktree creation in birdseye funnels through one function, `worktree.Add`
(`internal/worktree/worktree.go`), which is reached by two callers at different altitudes:

- `be worktree add` (`internal/cli/worktree.go`) - agentless. Resolves the repo from cwd,
  calls `worktree.Add`, prints the path. No tmux, no agent.
- `fleet.Spawn` (`internal/fleet/fleet.go:246`) - the orchestration core behind both the TUI
  agents view and headless `be agents spawn`. Resolves the repo, ensures the `be-<repo>` home
  tmux session, calls `worktree.Add`, then opens a tmux window rooted at the new worktree
  running the agent command.

Between `worktree.Add` and the agent there is nothing. A new worktree is a clean checkout, so
git-ignored local state (`.env`, deps, generated files) is absent. This change inserts a
provisioning step at that seam. Config today is a single user-level `config.toml`
(`internal/config/config.go`); there is no repo-local config and no notion of a setup command.

## Goals / Non-Goals

**Goals:**
- Carry git-ignored, project-marked local files into a new worktree (`.worktreeinclude`).
- Run a project-defined setup command on every worktree creation, configurable per-repo with a
  global fallback.
- Serve both the agentless CLI and the agent spawn paths from shared logic, with execution that
  fits each (inline+streamed vs. chained into the agent window).
- Run only on creation, never on slot re-wake.

**Non-Goals:**
- A cleanup/teardown hook on `dD` removal (possible follow-up).
- Provisioning worktrees that birdseye did not create (e.g. ones made directly with `git`).
- Container/devcontainer isolation.
- Re-running setup on resume.

## Decisions

### Two artifacts, not one
`.worktreeinclude` (copy) and `.birdseye/config.toml` (`setup` command) stay separate.
`.worktreeinclude` is an established cross-tool standard (Claude Code, Codex, Conductor,
CodeBuddy, opencode); reusing it gives free interop and a battle-tested guardrail. The setup
command is birdseye's own, carried in `.birdseye/config.toml`. The established order is
copy-then-run, and we keep it.

Alternative considered: fold copies into the setup command (just `cp` in the script). Rejected -
a hand-rolled `cp` can clobber a tracked file or miss one, and loses the "from the primary
worktree" source resolution that the convention gives for free.

### `.birdseye/config.toml` is the user config schema, layered (not a bespoke struct)
Rather than a one-off `RepoConfig{Setup}`, the repo-local file is the **same `Config` schema**
as the user config, and a project may override **any** key for repository-scoped operations.
This is implemented by layered TOML decoding: `Default()` → user file → repo file, each layer
decoding onto the same struct. The decoder gives exactly the semantics we want - a scalar the
repo sets overrides, a scalar it omits is inherited, tables merge key-by-key, lists are replaced
- verified empirically before committing to it. `config.Overlay(base, repoRoot)` produces the
effective config by deep-copying `base` and decoding the repo file on top (the deep copy is
essential: decoding mutates maps in place, so without it a spawn would permanently fold one
repo's overrides into the in-memory user config). The setup command is then just
`effective.Worktree.Setup`, and the agent command `effective.Agents.AgentCommand()` - per-repo
agent choice falls out for free.

Scope: overrides apply only where a single repository is in scope - `be worktree add`,
`be agents spawn`, and slot re-open. The dash and picker span many repositories at once, so they
keep using the user config; there is no coherent "the repo" for them.

Alternative considered: a narrow `setup`-only repo file. Rejected per the request to let any
user-level key be overridden by project-level config without inventing more files.

### Copy step lives inside `worktree.Add`; setup execution is split by caller
The copy is pure filesystem and fast, so it belongs inside `worktree.Add` - both callers get it
with zero duplication. The setup command is the slow/interactive part, so where it runs differs:
- `be worktree add`: run inline via `exec.Command` with stdout/stderr wired to the terminal; a
  non-zero exit fails the command. Add a `--no-setup` flag.
- `fleet.Spawn`: do not run inline (would block the TUI keystroke / the spawn call). Instead build
  the window's command as `<env> <setup> && <agent>` so setup runs in the agent's own pane, output
  is visible, and a failure short-circuits the `&&`, leaving a shell. When no setup is configured,
  the window runs the agent directly (today's behavior). `exec` is intentionally *not* used, to
  preserve the existing "agent runs inside the interactive shell" behavior (direnv/profile load).

Alternative considered: run setup synchronously inside `worktree.Add` for both. Rejected - it
freezes the TUI for the duration of an install and needs an async "setting up…" row state to fix.

### Copy via git plumbing, not a custom matcher
Eligible files = matched by `.worktreeinclude` **and** git-ignored. Rather than implement
`.gitignore` semantics, derive the set from git in the primary worktree
(`git ls-files --others --ignored --exclude-from=.worktreeinclude`, each confirmed with
`git check-ignore`), then copy preserving relative paths. This makes the "never copy a tracked
file" guarantee git's, not ours.

### Env contract
`BIRDSEYE_WORKTREE` (new worktree abs path, also the cwd), `BIRDSEYE_REPO` (primary worktree abs
path / copy source), `BIRDSEYE_BRANCH` (branch checked out). Mirrors the `CONDUCTOR_ROOT_PATH`
style convention so a setup script can locate the source when it needs to.

## Risks / Trade-offs

- [Setup runs only on birdseye-created worktrees] → Worktrees made directly with `git` won't be
  provisioned. Accepted: provisioning is a creation-time convenience, and the copy/setup remain
  re-runnable by hand. Re-running on re-wake is explicitly out of scope.
- [Chained-into-window setup pollutes the agent's scrollback] → Minor; the trade for a
  non-blocking TUI and visible setup. The agent starts on a clean prompt after `&&`.
- [A failed spawn setup leaves the worktree on disk] → Intentional and consistent with the CLI
  path; the worktree drops to a slot the user can fix or remove (`dD`).
- [git plumbing flag nuances for "matched and ignored"] → Pin the exact `git ls-files`
  invocation in implementation and cover it with a test using a tracked vs. ignored vs.
  unmatched file, so the guardrail is verified, not assumed.
- [Shell quoting when chaining `<setup> && <agent>`] → Reuse the existing prompt-quoting
  approach (`shellQuote` in fleet) and test the composed window command.
- [Same-schema repo config invites overriding keys that only make sense process-wide] → Repo
  overrides are consumed only by repo-scoped operations; the dash/picker ignore them. A repo
  setting, say, a picker label has no effect, which is harmless but could surprise. Documented.

## Migration Plan

Purely additive. No config migration: absent `.worktreeinclude` and `.birdseye/config.toml`
mean current behavior is unchanged. No data model changes. Rollback is removing the provisioning
calls; created worktrees are unaffected.

## Open Questions

- Resolved: `.birdseye/config.toml` mirrors the user config schema, so the setup command is
  `[worktree] setup` in both files (not a top-level `setup` key), and any key is overridable.
