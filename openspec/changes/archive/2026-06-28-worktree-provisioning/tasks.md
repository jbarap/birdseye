## 1. Config: unified schema + repo-local override (`.birdseye/config.toml`)

- [x] 1.1 Add a `[worktree] setup` key to the user `Config` in `internal/config/config.go`; the repo-local file uses the same `Config` schema (no bespoke struct).
- [x] 1.2 Factor a shared `mergeFile` that layers a TOML file onto a `Config` (missing = no-op, malformed/legacy = error); have `LoadFrom` use it.
- [x] 1.3 Add `Overlay(base Config, repoRoot string)` that deep-copies `base` and decodes `<repoRoot>/.birdseye/config.toml` on top, so any key the project sets overrides while the rest is inherited (scalars override, tables merge, lists replace); discovered relative to the repository.
- [x] 1.4 Unit-test config: no-repo inherits base, repo overrides+inherits, maps merge, base not mutated, malformed repo-local rejected, `[worktree] setup` accepted.

## 2. Worktree copy step (`.worktreeinclude`)

- [x] 2.1 Add `CopyIncludes(primary, dst string)` to `internal/worktree` that resolves eligible files (matched by `.worktreeinclude` AND git-ignored) via git plumbing in the primary worktree and copies each into `dst` preserving relative paths.
- [x] 2.2 Make it best-effort: no `.worktreeinclude` or no matches is a no-op; a per-file copy error is collected and returned as a non-fatal warning, not an abort.
- [x] 2.3 Call `CopyIncludes` from inside `worktree.Add` (after `git worktree add`) so both callers get it; thread the non-fatal warning out for callers to surface.
- [x] 2.4 Unit-test copy: git-ignored matched file copied, tracked file never copied, unmatched/absent is a no-op, nested relative path preserved, copy error is non-fatal.

## 3. Setup command: env contract + resolution from effective config

- [x] 3.1 Resolve the setup command as `config.Overlay(userCfg, primary).Worktree.Setup` at the repo-scoped callers (no bespoke resolver in `worktree`).
- [x] 3.2 Define the env contract helper `worktree.SetupEnv` (`BIRDSEYE_WORKTREE`, `BIRDSEYE_REPO`, `BIRDSEYE_BRANCH`), with cwd = the new worktree.
- [x] 3.3 Run the command via the shell with stdout/stderr wired through for the agentless path, returning the exit error.
- [x] 3.4 Unit-test the env contract and the spawn composition; precedence is covered by the config `Overlay` tests.

## 4. Wire `be worktree add`

- [x] 4.1 In `internal/cli/worktree.go`, after `worktree.Add`, surface any copy warning, then resolve and run setup inline (streamed), failing the command on non-zero exit while leaving the worktree on disk.
- [x] 4.2 Add a `--no-setup` flag that skips the setup command (copy still runs).
- [x] 4.3 Test the command path: setup runs and streams, `--no-setup` skips it, non-zero setup fails the command.

## 5. Wire `fleet.Spawn` (agent path)

- [x] 5.1 Make `Fleet` hold the user `config.Config`; in `Spawn` overlay the repo config to get the effective agent command and `[worktree] setup`, then build the window command as `<env> <setup> && <agent>` (reuse `shellQuote`); when no setup, run the agent directly.
- [x] 5.2 Export the env contract into the spawned window as the prefix on the setup command.
- [x] 5.3 Have `Open` use the effective agent command; `Open`/`OpenShell` add no provisioning (no copy, no setup on re-wake).
- [x] 5.4 Test the composed window command for the configured case and a per-repo agent-command override (real git repo + fake tmux runner).

## 6. End-to-end verification

- [x] 6.1 `go build ./...` and `go test ./...` green.
- [x] 6.2 Verify the agent (spawn) path via a hermetic end-to-end `fleet.Spawn` test (real git repo + fake tmux runner): the window runs `<setup> && <agent>` with the env contract, and a per-repo `[agents] command` overrides the default. (Chosen over a live TUI drive to avoid spawning real agents / stray tmux sessions.)
- [x] 6.3 Run `be worktree add` standalone against a real repo and confirm copy + streamed inline setup with the env contract, `--no-setup` skips setup, and a non-zero setup fails the command while keeping the worktree.
- [x] 6.4 Update README docs and `be worktree add` help text to cover `.worktreeinclude` and the same-schema `.birdseye/config.toml` override.
