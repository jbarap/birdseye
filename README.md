# birdseye (`be`)

> Use tmux? Use agents? Need order? Get a bird's-eye view.

**Shamelessly vibe coded. Frequent breaking changes.**

`be` is a lightweight, hackable birdseye view over your tmux sessions and the AI
agents running inside them. It blends the sessions you already have with quick ways
to create new ones into one fuzzy picker, and gives you an at-a-glance triage view of
your agents. Built on tmux, driven by `fzf`, extensible through a small Go provider
interface.

## Quick start

```sh
just install          # build `be` into your GOBIN
# or: just build     → ./bin/be (static, CGO-free, copy anywhere)

be dash               # live agent status view (the TUI)
be sessions           # fuzzy picker: attach to a session or create a new one
be agents install claude   # opt-in Claude Code integration (status hooks, workflow skills)
```

## What it does

- **`be dash`** - a live, tmux-popup-friendly TUI showing your Claude Code agents and
  their status (needs-attention / working / idle / done). Two lenses over the same
  agents sit in a sidebar - an **Agents** triage list and a **Projects** repo→worktree
  tree - with a preview of the selected session beside them. Projects are grouped into
  namespace tabs by parent directory. Spin agents up (`n`) and tear them down (`dd` /
  `dD`) in place; open a new session without leaving the view (`s`). Vim-native,
  configurable keys.

- **Notifications** - with the Claude hooks installed, `be` pings you only when an agent
  needs you (a permission/input block), stays silent while it works, and fires a single
  digest when the last working agent settles. Auto-detects `notify-send` / `osascript`,
  or route delivery yourself with `[agents.notify].command`.

- **`be agents`** - the same agent lifecycle, headless, for scripts and orchestrating
  agents: `list`, `status`, `spawn`, `send`, `jump`, `close`, `delete` (`--json` on the
  data verbs). The dash and these verbs share one operation layer, so they behave
  identically.

- **`be sessions`** - one fuzzy picker over running tmux sessions, `tmuxp` templates,
  `zoxide`/directory roots, and git worktrees. `--json` enumerates the same candidates.

- **`be worktree`** - spin up grouped-sibling worktrees at `<repo>.worktrees/<slug>`
  from inside any repo. `be` recognizes any clone; no special layout required.

## Requirements

| Tool     | Required | Used for                              |
|----------|----------|---------------------------------------|
| `tmux`   | yes      | the session backend                   |
| `fzf`    | yes      | the picker UI                         |
| `tmuxp`  | optional | template provider                     |
| `zoxide` | optional | directory provider                    |
| `git`    | optional | `be worktree`, agent orchestration    |

Optional tools degrade gracefully: a missing one disables its provider with a
non-fatal warning.

## Work handles

Agents are addressed by a git-derived `repo/worktree` handle (a worktreeless agent by
its tmux pane id, e.g. `%5`), so the handle survives session/window renames, tmux
restarts, and respawns. `<repo>` is a repository's primary worktree, `<repo>/<worktree>`
a linked worktree. Because handles are derived, any agent is addressable with no import
step - even ones `be` never spawned.

```sh
be agents list --json
be agents spawn <dir> --branch <b> [--prompt <p>]   # new worktree + window + agent
be agents send  <repo>/<worktree> "<input>"
be agents jump  <repo>/<worktree>
be agents close <repo>/<worktree>                    # close window, keep worktree
be agents delete <repo>/<worktree> [--force]         # also git worktree remove
```

> **Breaking change:** bare `be` no longer launches anything - a subcommand is required.
> The TUI is `be dash`; `be agents` is the headless verb namespace. Update popup
> keybindings and scripts.

### Recommended tmux popup

```tmux
bind-key g display-popup -E -w 80% -h 60% "be dash"
```

The dash refreshes in place, so a popup left open stays current.

## Repositories & orchestration

`be` *recognizes* - it never takes over - the common worktree-per-agent layout, and
groups what it shows by **repository** (its shared git dir) rather than by tmux session.
A repo lights up when any pane's path or agent's working directory resolves into one of
its worktrees; no path convention or whole-session gate. `be` owns no state - every
refresh re-derives the picture from tmux + git, so unrecognized sessions behave exactly
as before.

New agents land in the repo's on-demand **`<repo>-<hash>` home session**, so `be` only
authors windows in sessions it named. In a recognized repo the dash adds a `⌂ base` row
for the primary worktree and a `◌ slot` row for each agentless worktree (a spawn target),
plus three actions (all rebindable under `[agents.keys]`):

- **`n`** - new agent: form for branch + worktree (auto-slugged), then creates the
  worktree, opens a tmux window there, and runs the agent command *through the window's
  shell* (so it inherits `direnv`, your profile, `PATH`).
- **`dd`** - close: closes the window only. Instant, no confirmation, no filesystem
  effect; the worktree survives as a `◌ slot`. Reversible.
- **`dD`** - delete: closes the window *and* `git worktree remove`, behind a confirm
  popup. Refuses the base and any worktree shared by co-located agents.

### The Claude Code trust / permission prompt

A fresh worktree is a directory Claude Code hasn't seen, so it opens with a trust check
and per-action permission prompts. Both come from Claude Code, not `be`, and two settings
make `n` prompt-free:

- **Trust once.** Claude Code walks up parent directories when checking folder trust, so
  trusting a *parent* of your worktrees once covers every worktree beneath it.
- **Reduce prompts.** Set the permission mode, e.g.
  `command = "claude --permission-mode=auto"` - lets low-risk actions through while still
  gating dangerous ones (safer than `--dangerously-skip-permissions`).

The git worktree is the isolation boundary: each agent works on its own branch in its
own directory.

## Configuration

Config lives at `~/.config/birdseye/config.toml`. All keys are optional; loading is
**strict** (an unknown key is a hard error naming it). The `be config` command is the
source of truth for the full schema:

```sh
be config              # effective config: defaults + your files, merged
be config --defaults   # annotated default template (every key + how to override)
be config --check      # validate config files; exits non-zero on problems
```

### Worktree provisioning

A fresh worktree is a clean checkout, so git-ignored files (your `.env`, deps) are
missing. `be` provisions on creation - on `be worktree add`, `be agents spawn`, and the
dash `n` - via two repo-root files:

- **`.worktreeinclude`** - `.gitignore`-syntax list of local, git-ignored files to copy
  from the primary worktree (secrets, certs, local config). Not for `node_modules`/`.venv`
  - let the setup command regenerate those.
- **`.birdseye/config.toml`** - commit-able repo-local config using the same schema as
  the user config; overrides user settings for repo-scoped operations. Its `[worktree]
  setup` command runs in the new worktree after the copy, with `BIRDSEYE_WORKTREE`,
  `BIRDSEYE_REPO`, and `BIRDSEYE_BRANCH` exported.

```toml
# .birdseye/config.toml
[worktree]
setup = "./scripts/worktree-setup.sh"

[agents]
command = "claude --permission-mode=auto"
```

## Claude Code integration (`be agents install`)

Two independent, opt-in tiers - the binary itself stays un-opinionated:

- **hooks** - status detection. Agent status comes from state Claude Code hooks write,
  never inferred from pane contents.
- **workflows** - `be`-authored **skills** (orchestrator, QA, PR) that compose the
  `be agents` verbs. Removable artifacts, never code in the binary.

```sh
be agents install claude --hooks               # status detection only
be agents install claude --workflows           # the skills only
be agents install claude                        # interactive checklist (in a terminal)
be agents uninstall claude --workflows         # symmetric, tier-scoped
```

Nothing installs implicitly: no tier flag shows a checklist in a terminal, or errors in a
non-TTY. The hooks install is safe and idempotent - it preserves other keys, backs up to
`.bak`, and uninstall removes only what `be` wrote. Removing either tier leaves the other
fully functional.

Installed hooks record state under `$XDG_STATE_HOME/birdseye/agents/` on Claude Code's
`SessionStart` / `UserPromptSubmit` / `Notification` / `Stop` / `SessionEnd` events. The
working↔idle split is corrected each refresh from the pane's live terminal title (Claude's
spinner vs sparkle); hooks stay authoritative for needs-attention and done. Status not
refreshed within `agents.stale_after` shows as **unknown**.

## Adding a provider

A provider is any type implementing `internal/provider.Provider`:

```go
type Provider interface {
    Type() string                       // source tag, also the config key
    Candidates() ([]Candidate, error)   // attach-or-create entries
}
```

Register it in `internal/cli.buildRegistry` and it appears in the picker - no picker or
tmux changes needed.

## Development

```sh
just check     # go vet + go test ./...
just build     # build ./bin/be
just --list    # all recipes
```

The visual language is documented in [DESIGN.md](DESIGN.md) - read it before touching any
UI surface.

## References

Birdseye draws lots of stuff from these great projects:
- [sesh](https://github.com/joshmedeski/sesh)
- [tmux-agent-status](https://github.com/samleeney/tmux-agent-status)
- [herdr](https://github.com/ogulcancelik/herdr)
- [agentapi](https://github.com/coder/agentapi)
- [superset](https://github.com/superset-sh/superset)
- [tsm](https://github.com/adibhanna/tsm)
