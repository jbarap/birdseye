## Why

bird's-eye wants to ship its own opinionated agent workflows — an orchestrator that spawns workers,
runs QA, opens PRs — without baking any of that into the binary, which must stay un-opinionated
("substrate, not orchestrator"). The resolution is to ship opinions as *removable artifacts*:
installable skills/commands plus the existing status hooks, all opt-in.

## What Changes

- Generalize `be hook claude install` into `be agents install <type>` (e.g. `claude`), covering two
  independent tiers:
  - **hooks** — enable status detection (so the watch layer can see agent state).
  - **workflows** — bird's-eye-authored skills/commands that compose the `be agents` verbs into
    opinionated playbooks (orchestrator, QA, PR).
- Each tier is independently **opt-in** via `--hooks` / `--workflows`. With no flag, an interactive
  checklist with a one-line explanation per item — never an implicit install.
- Removing the workflow skills breaks nothing core; the hooks tier keeps today's safe, idempotent
  install/uninstall behavior.
- Ship the initial workflow artifacts (orchestrator + QA/PR playbooks) as the installable content.

## Capabilities

### New Capabilities

- `agents-install`: the tiered, opt-in integration installer and the shipped workflow artifacts.

### Modified Capabilities

- `agent-view`: the existing safe hook install/uninstall is reached through `be agents install`
  (hooks tier); behavior preserved.

## Impact

- `internal/cli`: `be agents install` with tier selection and an interactive checklist; the existing
  hook install logic moves under it (a deprecated `be hook` alias may remain).
- Bundled skill/command artifacts (content) shipped with the binary and written into the user's agent
  config on opt-in.
- **Depends on** `agents-headless-cli` — the workflow skills compose the `be agents` verbs.
