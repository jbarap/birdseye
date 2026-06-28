## Why

A freshly created worktree is a clean checkout: everything git-ignored is missing - `.env`
and local secrets, installed dependencies, generated files, local config. So a new worktree
is not actually ready to work in until someone hand-copies files and runs setup. Today
birdseye drops straight from `git worktree add` into the agent (or hands back a bare
directory), with nothing in between. Every comparable agent-orchestration tool fills this gap;
birdseye should too, and in a way that also serves the plain `be worktree add` CLI.

## What Changes

- **Honor `.worktreeinclude`** (the cross-tool standard, `.gitignore` syntax) at the repo
  root: on worktree creation, copy the files that both match an include pattern **and** are
  git-ignored from the primary worktree into the new worktree. Tracked files are never copied.
- **Run a configured setup command on every worktree creation**, taken from `[worktree] setup`
  of the effective configuration. The command runs in the new worktree with
  `BIRDSEYE_WORKTREE`, `BIRDSEYE_REPO`, and `BIRDSEYE_BRANCH` exported.
- **Introduce a repo-local `.birdseye/config.toml` that uses the same schema as the user
  config** and overrides any user-level setting for repository-scoped operations (worktree
  add, spawn): a key the project sets wins, the rest is inherited (scalars override, tables
  merge, lists replace). This makes "where does setup come from" one general override mechanism
  that also covers, e.g., a per-repo `[agents] command`.
- **Both steps run only on worktree creation** (the `worktree.Add` path), never on slot
  re-wake (`Open`/`OpenShell`), matching every tool's "skipped on resume" rule.
- **Execution differs by caller, resolution is shared:**
  - `be worktree add` (agentless) runs setup **inline, streaming to the terminal**, exiting
    non-zero on failure. A `--no-setup` flag skips it for a bare directory.
  - `be agents spawn` / the TUI spawn (agent path) **chains setup into the new tmux window**
    as `<setup> && exec <agent>`, so the TUI never blocks, output is visible in the agent's
    pane, and a failed setup stops at a shell instead of starting the agent in a broken env.
- The copy step lives inside `worktree.Add`, so both callers get it with no duplication. It is
  implemented with git plumbing (`git ls-files --others --ignored` + the include patterns) so
  the "matched and git-ignored, never tracked" guardrail comes from git itself. A missing or
  unmatched include is a no-op; a real copy error is a non-fatal warning.

Out of scope (v1): a cleanup/teardown script on `dD` removal.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `worktree-provider`: worktree creation gains two new behaviors - carrying git-ignored
  local files via `.worktreeinclude`, and running a configured setup command (with the
  env contract, creation-only rule, inline execution for `be worktree add`, and failure
  semantics).
- `agents-cli`: `be agents spawn` provisions the new worktree (copy + setup) and chains the
  setup command into the agent's tmux window rather than blocking, so a failed setup stops
  before the agent starts.
- `be-cli`: configuration loading gains a repo-local `.birdseye/config.toml` using the same
  schema as the user config, layered over the user config to form the effective configuration
  for repository-scoped operations (any key overridable; tables merge, lists replace).

## Impact

- `internal/config`: a repo-local `.birdseye/config.toml` decoded onto a deep copy of the user
  config (`Overlay`), shared `mergeFile` layering, used by repo-scoped callers.
- `internal/worktree`: copy step inside `Add`, plus the env contract for the setup command.
- `internal/cli/worktree.go`: inline streamed setup for `be worktree add`, `--no-setup` flag.
- `internal/fleet/fleet.go`: holds the user config and overlays the repo config per spawn,
  resolving the effective agent and setup commands; `Open` uses the effective agent command;
  re-wake adds no provisioning.
- New artifacts read by birdseye: `.worktreeinclude` and `.birdseye/config.toml` at repo root.
