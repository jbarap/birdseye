## 1. Install command surface (`internal/cli`)

- [x] 1.1 Add `be agents install <type>` and `be agents uninstall <type>` under the `be agents` namespace, with `<type>` validated (initially `claude`).
- [x] 1.2 Add `--hooks` and `--workflows` flags (independent booleans) plus the existing `--settings` / `--command` passthrough for the hooks tier.
- [x] 1.3 Implement no-flag behavior: interactive checklist (one line per tier) in a TTY; error asking for flags in a non-TTY; selecting nothing is a no-op.
- [x] 1.4 Keep `be hook claude install`/`uninstall` as hidden, deprecated aliases routing to the hooks tier.

## 2. Hooks tier wiring

- [x] 2.1 Route the hooks tier to the existing `agents.InstallHooks` / `UninstallHooks`, preserving backup, idempotency, malformed-refusal, and custom-path behavior.
- [x] 2.2 Confirm `be agents uninstall claude --hooks` removes only bird's-eye's entries.

## 3. Workflows tier — embedded artifacts

- [x] 3.1 Add the workflow skill/command artifacts to the repo and embed them with `//go:embed`.
- [x] 3.2 Resolve the per-type agent config destination (a bird's-eye-namespaced location under Claude Code's skills/commands dir), in one place like `DefaultSettingsPath`.
- [x] 3.3 Implement idempotent install: write only changed artifacts; back up or refuse to clobber a user-modified artifact.
- [x] 3.4 Implement uninstall that removes only bird's-eye's namespaced artifacts, leaving user skills intact.

## 4. Ship the initial workflows

- [x] 4.1 Author the orchestrator workflow (spawns/polls/steers workers via `be agents spawn`/`list`/`status`/`send`).
- [x] 4.2 Author the QA workflow and the PR workflow composing the `be agents` verbs.
- [x] 4.3 Verify each shipped workflow references only stable `be agents` verbs and `be sessions --json`.

## 5. Docs

- [x] 5.1 README: replace the `be hook claude install` setup section with `be agents install claude`, documenting both tiers, the interactive checklist, and the deprecated alias.
- [x] 5.2 Document removability: uninstalling workflows (or never installing them) leaves the binary, verbs, and hooks fully functional.

## 6. Verification

- [x] 6.1 Tests: tier independence (install one, the other untouched), idempotent re-install, namespaced uninstall removes only ours, non-TTY no-flag error.
- [x] 6.2 `just check` (go vet + go test).
- [x] 6.3 Manual smoke: `--hooks` only, `--workflows` only, both, no-flag checklist, and symmetric uninstall.
