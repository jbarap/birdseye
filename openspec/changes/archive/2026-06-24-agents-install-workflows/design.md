## Context

bird's-eye installs Claude Code status hooks today via `be hook claude install`
(`internal/cli/hook.go` → `agents.InstallHooks`). That install is the model to emulate: it
merges into `~/.claude/settings.json`, preserves all unrelated config and hooks, writes a
`.bak` backup, is idempotent, and uninstall removes only bird's-eye's own entries.

The tool now wants to ship its own opinionated agent *workflows* — an orchestrator that
spawns workers, a QA playbook, a PR playbook — built on the `be agents` verbs
(`agents-headless-cli`). But the binary must stay un-opinionated ("substrate, not
orchestrator"). The resolution from the design language is "opinions ship as removable
artifacts": the workflows are installable skills/commands, never code in the binary, and
both they and the hooks are opt-in.

This change generalizes the installer into `be agents install <type>` with two independent
opt-in tiers, and ships the first workflow artifacts. It depends on `agents-headless-cli`
(the verbs the workflows compose, and the `be agents` namespace the command lives in).

## Goals / Non-Goals

**Goals:**

- `be agents install <type>` (e.g. `claude`) covering two **independent, opt-in** tiers:
  - **hooks** — status detection (the existing safe install).
  - **workflows** — bird's-eye-authored skills/commands composing the `be agents` verbs.
- Tiers selected by `--hooks` / `--workflows`; with no flag, an interactive checklist with
  a one-line explanation per tier. Never an implicit install.
- Symmetric `be agents uninstall <type>` that removes only what bird's-eye wrote, per tier.
- Ship the initial workflow artifacts (orchestrator + QA + PR), embedded in the binary and
  written into the user's agent config on opt-in.
- Preserve the existing hook install/uninstall safety guarantees exactly.

**Non-Goals:**

- The `be agents` verbs the workflows call (defined in `agents-headless-cli`).
- Authoring the full prose of every shipped skill — the *mechanism* is specified here; the
  skill content is created during apply.
- Supporting agent types other than `claude` initially (the `<type>` arg is the extension
  point, not a promise of more types now).

## Decisions

### Type-scoped install command

The installer is `be agents install <type>` / `be agents uninstall <type>`, mirroring the
existing `be hook claude` per-agent grouping.

- *Why:* each agent type has its own config locations and hook schema; scoping by type
  keeps the surface honest and lets new types be added without reshaping the command.
- *Note:* `be hook claude install`/`uninstall` remain as **deprecated, hidden aliases**
  routing to the hooks tier, so existing instructions keep working.

### Two independent opt-in tiers; nothing implicit

`--hooks` and `--workflows` are independent booleans. Passing either installs that tier.
Passing neither presents an interactive checklist (one line explaining each tier) and
installs exactly what the user selects — selecting nothing is a no-op.

- *Why opt-in, not opt-out:* a bare `be agents install claude` must never silently write
  workflow opinions into a user's config. The user always chooses.
- *Why a checklist on no-flag:* discoverability without surprise — the user sees both tiers
  and what each does before anything is written.
- *Alternative considered:* a single `--all` / interactive default that installs both.
  Rejected — it reintroduces implicit opinion installation.

### Hooks tier reuses the existing safe install verbatim

The hooks tier calls the existing `agents.InstallHooks` / `UninstallHooks`, keeping the
preserve-everything, `.bak`-backup, idempotent, refuse-malformed behavior.

- *Why:* that behavior is already correct and spec'd; this change only changes how it is
  *reached*, not what it does.

### Workflows tier writes removable, namespaced skill files

The workflows tier copies embedded skill/command artifacts into the agent's config (for
Claude, under its skills/commands directory) in a **bird's-eye-namespaced** location, so
uninstall can remove exactly the tool's files without touching the user's own skills.
Install is idempotent (re-running rewrites only changed files) and backs up / refuses to
clobber a user-modified file the way the hooks install does.

- *Why namespaced:* clean, safe uninstall requires knowing precisely which files are ours;
  a dedicated subdirectory (or filename prefix) gives that without a separate manifest.
- *Why embedded:* the artifacts ship *with* the binary (`//go:embed`) so install needs no
  network and the shipped content matches the binary version.
- *Removable by construction:* deleting the workflow files breaks nothing core — the
  binary, the verbs, and the hooks tier are all independent of them.

### Uninstall is symmetric and tier-scoped

`be agents uninstall <type>` accepts the same `--hooks` / `--workflows` selection (no flag
→ checklist of what is currently installed) and removes only bird's-eye's entries/files for
the chosen tiers.

- *Why:* symmetry with install and the "removes only its own" guarantee already expected of
  the hooks path.

## Risks / Trade-offs

- **Clobbering user-edited workflow files.** [A user edits an installed skill, then
  re-installs and loses changes] → mitigated by the same idempotent compare + `.bak` backup
  the hooks install uses, scoped to the namespaced location; unchanged files are left
  alone.
- **Workflows reference verbs that must exist.** [A shipped skill calls `be agents spawn`
  before `agents-headless-cli` lands] → this change depends on `agents-headless-cli`;
  ordering is enforced by that dependency.
- **Agent config layout drift.** [Claude Code changes where skills live] → the write
  location is resolved per type in one place (like `DefaultSettingsPath`), so a layout
  change is a single-point fix; captured as an open question below.
- **Interactive checklist in a non-TTY.** [`be agents install claude` piped/scripted with
  no flags has no way to prompt] → in a non-interactive context the no-flag form SHALL
  error telling the caller to pass `--hooks`/`--workflows`, rather than hang or guess.

## Migration Plan

1. Add `be agents install <type>` / `uninstall <type>` with `--hooks` / `--workflows` and
   the no-flag interactive checklist (erroring in a non-TTY).
2. Route the hooks tier to the existing `InstallHooks`/`UninstallHooks`; keep
   `be hook claude install`/`uninstall` as hidden deprecated aliases.
3. Embed the workflow artifacts (`//go:embed`); implement namespaced, idempotent,
   backup-on-clobber file install/uninstall for the workflows tier.
4. Author the initial orchestrator / QA / PR skill artifacts that compose the `be agents`
   verbs.
5. Update README's hook-setup section to `be agents install claude`, documenting both
   tiers and the deprecated alias.

Rollback is reverting the release; uninstall cleanly removes installed artifacts.

## Open Questions

- Exact Claude Code skills/commands directory and the namespaced layout within it
  (subdirectory `birds-eye/` vs a filename prefix). Resolve against current Claude Code
  conventions during apply.
- Whether re-install should *upgrade* shipped skills to the running binary's version
  automatically, or report drift and require a flag. Leaning: idempotent upgrade of
  unmodified files, leave modified ones with a warning.
- The concrete skill set and each skill's contract (inputs/outputs) beyond the initial
  orchestrator/QA/PR — out of scope for the mechanism, tracked for the content pass.
