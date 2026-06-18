## 1. Project scaffold

- [x] 1.1 Initialize Go module and repo layout (`cmd/be`, `internal/{config,provider,providers,tmux,picker,agents}`)
- [x] 1.2 Add dependencies (Cobra, TOML decoder, Bubble Tea/Lipgloss) and a Makefile/`go build` target producing a static `be` binary
- [x] 1.3 Wire Cobra root command with `--version` and a default `list` command (`be` == `be list`)
- [x] 1.4 Set up basic test harness and CI-friendly `go test ./...` and `go vet` targets

## 2. Configuration (be-cli)

- [x] 2.1 Define config struct and built-in defaults (enabled providers, type ordering/labels, per-provider options)
- [x] 2.2 Locate and load TOML config from the XDG config dir, applying defaults for absent keys
- [x] 2.3 Fail with a clear, file-identifying error on malformed config; cover with tests
- [x] 2.4 Centralize external-tool discovery (probe `tmux`/`tmuxp`/`zoxide`/`git`/`fzf` on PATH once)

## 3. Provider registry & interface (be-cli, session-providers)

- [x] 3.1 Define `Provider` interface and `Candidate` type (Name, Label, Type, Action: attach|create)
- [x] 3.2 Implement the registry that aggregates enabled providers and skips disabled ones
- [x] 3.3 Isolate per-provider failures (skip + non-fatal warning) and continue with remaining providers
- [x] 3.4 Implement deterministic dedup by canonical session Name (existing-session beats creator); add tests

## 4. tmux backend (tmux-backend)

- [x] 4.1 Implement `has-session` check and idempotent `new-session -d` create with working directory
- [x] 4.2 Implement context-aware connect: `attach-session` outside tmux, `switch-client` inside (branch on `$TMUX`)
- [x] 4.3 Detect missing `tmux` and surface tmux stderr on command failure; add tests against a fake/exec shim

## 5. Built-in providers (session-providers)

- [x] 5.1 Running-tmux-sessions provider: list active sessions as attach candidates; no error when server is down
- [x] 5.2 tmuxp provider: list templates as create candidates that `tmuxp load`; skip with warning when `tmuxp` absent
- [x] 5.3 Directory/zoxide provider: offer dirs (zoxide + configured roots) as create candidates named by directory; no-op when none available

## 6. Picker (session-picker)

- [x] 6.1 Implement the picker by piping labeled, type-grouped candidates into `fzf` and reading the selection; require `fzf` and fail clearly (with install hint) when absent
- [x] 6.2 Apply configurable type ordering/labels; render existing-vs-create distinction
- [x] 6.3 Dispatch selection to its Action (attach via tmux backend, or create-then-connect); clean exit on cancel
- [x] 6.4 Handle empty-state and non-interactive/no-TTY invocation with clear messages and exit codes

## 7. Agent view (agent-view)

- [x] 7.1 Define the `Agent` abstraction (location + status enum: needs-attention / working / idle / done, plus unknown) and the per-session state store it reads
- [x] 7.2 Implement the Claude Code agent backed by Claude Code hooks: ship a documented hook config that writes per-session state, and read it (render missing/stale state as unknown)
- [x] 7.3 Build the `be agents` Bubble Tea/Lipgloss view: grouped rows, status colors, needs-attention surfaced first
- [x] 7.4 Implement jump-to-selected-agent via the tmux backend; verify it runs and exits cleanly inside `tmux display-popup -E be agents`
- [x] 7.5 Empty-state handling when no agents are present

## 8. Worktree provider (worktree-provider)

- [x] 8.1 Implement `be worktree` clone into `<repo>/main` (no re-clone when present); require `git` with a clear error when absent
- [x] 8.2 Implement sibling worktree creation via `git worktree add <repo>/<name>` on a branch; decline duplicates with a conflict message
- [x] 8.3 Expose managed `main`/worktrees as provider candidates that ensure a tmux session rooted in the worktree dir

## 9. Integration, docs & polish

- [x] 9.1 End-to-end smoke test: `be` lists candidates and connects; `be agents` and `be worktree` run against a scripted tmux/git environment
- [x] 9.2 Verify required tools (`fzf`/`tmux`) fail with clear messages when absent, and optional tools (`tmuxp`/`zoxide`/`git`) degrade gracefully
- [x] 9.3 Write README/usage: install, config example, Claude Code hook setup, recommended `be agents` tmux popup keybinding, and how to add a new provider
- [x] 9.4 Tag a build and confirm the produced binary runs on a clean machine without a Go toolchain

## 10. Post-review refinements

- [x] 10.1 Set the module path to `github.com/jbarap/birds-eye` (correct GitHub username)
- [x] 10.2 Use a `justfile` (just) for build/test/install/check recipes instead of a Makefile
- [x] 10.3 Add `be hooks install` / `be hooks uninstall` that safely merge/remove hooks in the Claude settings file: preserve unrelated config and hooks, back up before writing, idempotent, refuse malformed JSON; `--settings` for a custom path and `--command` override (defaults to the absolute be path)
- [x] 10.4 Harden tmuxp load: detect the session actually created (snapshot sessions around the load) and connect to it when the template's session_name differs from the file name
