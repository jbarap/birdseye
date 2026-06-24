## 1. Shared operation layer

- [x] 1.1 Extract the TUI `orchestrator` lifecycle methods (spawn/close/delete, session ensure-or-reuse) into a package both `be dash` and the verbs call, so neither face holds private powers.
- [x] 1.2 Re-point the Bubble Tea model's actions at the shared layer; confirm `be dash` behavior is unchanged.

## 2. Handle resolution

- [x] 2.1 Implement `repo/worktree` handle derivation from git (`git-common-dir` + `git worktree list`): `<repo>` = primary worktree basename; `<repo>/<worktree>` = linked worktree dir basename.
- [x] 2.2 Implement reverse resolution (handle → worktree path, repo, tmux session/window/pane), and tmux-pane-id addressing for worktreeless agents.
- [x] 2.3 Detect basename-collision ambiguity and error with disambiguating paths instead of guessing.

## 3. Verb tree (`internal/cli/agents.go`)

- [x] 3.1 Replace the current `be agents` (TUI alias) with a verb-namespace command.
- [x] 3.2 `be agents list` / `status` with `--json` and a human table fallback; emit the stable record contract (handle, repo, worktree, branch, path, session{id,name}, window, pane, status, kind).
- [x] 3.3 `be agents spawn <dir>` with `--branch` and `--prompt`: resolve repo, ensure-or-reuse session, add sibling worktree, open window, start agent, print the new handle.
- [x] 3.4 `be agents close <handle>` (window only) and `be agents delete <handle>` (worktree too) with `--force`; refuse `delete` on a primary worktree.
- [x] 3.5 `be agents jump <handle>` (connect to pane) and `be agents send <handle> <input>` (best-effort `send-keys`, report dispatched-not-received).

## 4. Headless discovery (`be sessions --json`)

- [x] 4.1 Add a `--json` flag to `be sessions` that serializes the provider registry's candidates (name, label, type, kind, dir) without invoking fzf.
- [x] 4.2 Verify disabled providers are absent from the JSON, matching the interactive picker.

## 5. Root command restructure (`internal/cli/cli.go`)

- [x] 5.1 Drop bare-`be` RunE; make no-subcommand print "subcommand required" + available commands and exit non-zero.
- [x] 5.2 Add `be dash` wired to the current `runAgents`; keep `be sessions` and the hidden `be list` alias.
- [x] 5.3 Remove the `be agents` → TUI alias now that `be agents` is the verb namespace.

## 6. Docs

- [x] 6.1 README: replace bare-`be`/`be agents` TUI references with `be dash`; document the `be agents` verbs, the `repo/worktree` handle scheme, `be sessions --json`, and the breaking change.
- [x] 6.2 Update the recommended tmux popup keybinding to `be dash`.
- [x] 6.3 DESIGN.md: note the one-operation-layer / two-faces parity and the durability addressing layers.

## 7. Verification

- [x] 7.1 `just check` (go vet + go test); add tests for handle round-tripping and JSON record shape.
- [x] 7.2 Manual smoke: spawn into a fresh repo dir, list --json, send, jump, close, delete; confirm parity with the same actions in `be dash`.
