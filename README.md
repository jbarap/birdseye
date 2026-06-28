# birdseye (`be`)

> One place to launch, watch, and steer agent work across all your repos.

SHAMELESSLY VIBE CODED.

A lightweight, hackable birdseye view over your tmux sessions — and the AI
agents running inside them. `be` blends *existing* sessions with *ways to create
new ones* into one fuzzy picker, and gives you an at-a-glance triage view of your
agents.

Built on tmux (a proven backend), driven by `fzf`, and extensible through a small
provider interface.

## Prior art / references

birdseye draws on ideas/visuals/concepts from these projects:

- [sesh](https://github.com/joshmedeski/sesh) — smart tmux session manager and picker.
- [tmux-agent-status](https://github.com/samleeney/tmux-agent-status) — surfacing AI agent status in tmux.
- [herdr](https://github.com/ogulcancelik/herdr) - agent multiplexer; its OSC-title approach to
  reading agent state improved birdseye's existing hook-based status detection.
- [agentapi](https://github.com/coder/agentapi) - HTTP API over terminal agents; its
  terminal-activity diffing (quiescent vs changing) informs birdseye's title-less status fallback.
- [superset](https://github.com/superset-sh/superset) — a workspace over agent sessions.
- [tsm](https://github.com/adibhanna/tsm) — terminal session manager; its minimal, titled panels
  shaped birdseye's panel chrome.

## Features

- **`be dash`** — a tmux-popup-friendly view of your Claude Code sessions and
  their status (needs-attention / working / idle / done), so you know which need
  you. It shows two always-visible **lenses** over the same agents: an **Agents** lens
  on the left (a flat triage list in fixed `NEEDS YOU` / `WORKING` / `IDLE` / `DONE`
  section bands, most-urgent on top) and a **Workspaces** lens on the right (the
  repository→worktree tree, ordered by name and held stable so a row never jumps when
  its agent changes state). `h`/`l` switch focus; the selected agent is
  mirror-highlighted in the other lens. It **updates
  live** as sessions change, shows a **preview** of the selected session's terminal, and
  navigates with **vim-native, configurable** keys. It groups what it shows by
  **repository** (see below), letting you spin agents up and tear them down in place.
  Press `s` to open a new session from the same fuzzy picker without leaving the view.
- **`be agents`** — the same agent lifecycle headlessly, for scripts and orchestrating
  agents: `list`, `status`, `spawn`, `send`, `jump`, `close`, `delete`. Data verbs emit
  `--json`. Work is addressed by a derived `repo/worktree` handle (a worktreeless agent
  by its tmux pane id). The dash and these verbs share one operation layer, so they
  always behave identically. `be agents install claude` adds the opt-in Claude Code
  integration (status hooks and/or birdseye's orchestrator/QA/PR skills).
- **One fuzzy picker** (`be sessions`) over running tmux sessions, `tmuxp` templates,
  `zoxide`/directory roots, and git worktrees — attach to what exists or create
  what doesn't. `be sessions --json` enumerates the same candidates non-interactively.
- **`be worktree`** — spin up grouped-sibling worktrees from inside a repo
  (`git clone` the repo yourself; birdseye recognizes any clone). Worktrees live at
  `<repo>.worktrees/<branch-slug>` beside the clone, and worktrees discovered under
  optional configured roots are surfaced as session candidates.
- **Modular providers** — add a new way to create sessions by implementing one
  small Go interface.

## Requirements

| Tool     | Required | Used for                                   |
|----------|----------|--------------------------------------------|
| `tmux`   | yes      | the session backend                        |
| `fzf`    | yes      | the picker UI                              |
| `tmuxp`  | optional | template provider (skipped if absent)      |
| `zoxide` | optional | directory provider (skipped if absent)     |
| `git`    | optional | `be worktree` (errors clearly if absent)   |

Optional tools degrade gracefully: a missing one disables its provider with a
non-fatal warning rather than failing.

## Install

```sh
just install          # installs `be` into your GOBIN
# or
just build            # produces ./bin/be (static, no Go runtime needed)
```

The binary is built with `CGO_ENABLED=0` and is self-contained — copy `bin/be` to
any compatible machine and run it; no Go toolchain required.

## Usage

```sh
be dash               # live agent status view (the TUI)
be sessions           # open the fuzzy session picker
be sessions --json    # enumerate launch candidates non-interactively
git clone <url>                    # clone a repo with git directly — no special layout
be worktree add <name> [branch]    # run from inside the repo; → <repo>.worktrees/<branch-slug>

# Headless agent fleet (anything that runs a shell can drive it):
be agents list --json                       # the fleet as JSON records
be agents status <repo>/<worktree> --json   # one work item
be agents spawn <dir> --branch <b> [--prompt <p>]   # new worktree + window + agent
be agents send  <repo>/<worktree> "<input>"         # best-effort: reports sent, not received
be agents jump  <repo>/<worktree>                   # connect tmux to its pane
be agents close <repo>/<worktree>                   # close the window, keep the worktree
be agents delete <repo>/<worktree> [--force]        # also `git worktree remove`
```

> **Breaking change:** bare `be` no longer launches a view — a subcommand is now
> required, and the TUI moved to `be dash`. `be agents` is the headless verb namespace
> (it used to be an alias for the TUI). Update any popup keybindings and scripts to
> `be dash`.

A **work handle** is derived from git on every call, so it survives session/window
renames, tmux restarts, and an agent respawning: `<repo>` addresses a repository's
primary worktree (its base), `<repo>/<worktree>` a linked worktree, and a worktreeless
agent its tmux pane id (e.g. `%5`). Because handles are derived, an agent birdseye
never spawned is addressable with no import step. When two repos in view share a basename
the handle is ambiguous and the verb refuses with the disambiguating paths rather than
guessing.

### Recommended tmux popup keybinding

Add to `~/.tmux.conf` to pop the agents view from anywhere:

```tmux
bind-key g display-popup -E -w 80% -h 60% "be dash"
```

The agents view refreshes in place as hooks report new state, so a popup left open
stays current. When the popup is wide enough it shows a live preview of the selected
session's terminal beside the list; on narrow popups the preview is hidden. Navigate
with vim keys — `j`/`k` (or arrows) to move, `gg`/`G` for top/bottom, `ctrl+d`/`ctrl+u`
for half-page — `enter` jumps to the selected agent's session, and `q` dismisses it.
`s` opens a new session via the fuzzy picker (the same one as `be sessions`) and drops
you back in the view with it listed — handy for opening a repo before spinning agents up.
All of these keys are configurable (see `[agents.keys]` below).

### Repositories (agent orchestration)

birdseye *recognizes* — it never takes over — the common worktree-per-agent layout,
in both `be dash` and the `be agents` verbs, and groups what it shows by **repository**
rather than by tmux session. Recognition is **git-native** and **repo-first**: a repository
(identified by its shared git directory, `git rev-parse --git-common-dir`) lights up when
**any** pane's start path **or any** active agent's working directory resolves into one of
its worktrees. There is no whole-session gate - a stray non-git pane never hides a repo, and
a session touching two repos shows both as separate sections. One repo's rows may come from
several sessions; a row outside the repo's home session is flagged `[in: <session>]`.
No path convention is required, so worktrees recognize wherever they live - including ones
another tool created. birdseye owns no state for this: every refresh it re-derives the
picture from tmux + git, so an unrecognized session looks and behaves exactly as before.

New agents always land in the repository's **`<repo>-<hash>` home session** - be's write domain,
created on demand - so be only ever authors windows in sessions it named, never in yours. The
trailing hash marks the session as be-derived (and keeps same-named repos distinct); opening the
repo from the picker, `be agents spawn`, and the dash's `n` all resolve to that one home.

In a recognized repository the view shows, beside live agents:

- a `󱘎` indicator and worktree count on the section bar,
- a `⌂ base` row for the repo's **primary worktree** (git's main worktree, whatever branch
  it has checked out — git itself refuses to remove it, so the base can't be deleted),
- a `◌ slot` row for each worktree with no agent — a spawn target — **including worktrees
  with no open tmux window**, since the set is enumerated from `git worktree list`.

Two actions become available (configurable, see `[agents.keys]`):

- **`n` — new agent**: opens a small form with two fields — **branch** and **worktree** —
  where the worktree directory auto-fills from a slugified branch name (`feature/login` →
  `feature-login`) until you edit it. `⏎` creates, `⇥` switches fields (so you can tweak
  the worktree name), `esc` cancels. It then creates the worktree on that branch, opens a
  tmux window rooted there, and runs the configured agent command (`[agents] command`,
  default `claude`) **inside that window's interactive shell**.
- **`dd` — close**: closes the row's tmux window and nothing else — **instant, no
  confirmation, no filesystem effect**. Under the sibling-worktree layout the worktree
  survives and the row drops to a `◌ slot`, so closing is reversible (re-open a window in
  it). On a windowless slot it's a no-op. Closing the base's window ends the session if
  it was the last. Identical in effect to `be agents close`.
- **`dD` — delete**: closes the window **and** runs `git worktree remove`, behind a
  centered popup confirmation (consistent with the new-agent modal — never the old inline
  `(y/n)` line). A dirty worktree folds a force-remove choice into the same popup,
  defaulting to cancel. The repo base is refused, since git won't remove a primary
  worktree. Delete is permitted only on the **sole occupant** of a worktree: if two agents
  share one worktree it is refused with a notice, so a worktree is never removed out from
  under a co-located agent — close the others first. Identical in effect to `be agents delete`.

Safety lives in the **key**, not the dialog: `dd` can never touch disk regardless of how
fast you confirm, and only the irreversible `dD` prompts.

Agents with no git context render as incidental, ungrouped rows, and your ordinary shells
keep working exactly as today — orchestration is purely additive. Two agents in the same
worktree show as two rows (the worktree label repeats), each independently addressable.

The agent command is run *through the window's shell* (typed in with `tmux send-keys`),
not as tmux's bare child process. That means it inherits your normal per-directory
environment — `direnv`, shell profile, `PATH` — exactly as if you opened a pane in the
worktree and typed the command yourself. If you select a per-project tool config via
`direnv` (e.g. a `CLAUDE_CONFIG_DIR` set from an `.envrc`), the spawned agent picks it
up automatically.

### The Claude Code trust / permission prompt

A freshly created worktree is a directory Claude Code hasn't seen, so by default it
opens with *"Quick safety check: Is this a project you created or one you trust?"* and,
on each action, a permission prompt. Neither is a birdseye behavior — both come from
Claude Code — but two settings make `n` prompt-free without weakening anything by hand:

- **Trust once, inherit everywhere.** Claude Code records folder trust per directory and
  **walks up parent directories** when checking it. So trusting a *parent* of your
  worktrees once — run `claude` in, say, your repo root or worktree root and accept the
  dialog — covers every worktree created beneath it. No per-worktree prompts, and
  nothing edited by hand. (Because the agent runs through your shell, this is checked in
  the correct profile if you use per-project `CLAUDE_CONFIG_DIR`.)
- **Reduce permission prompts.** Set the agent command's permission mode, e.g.
  `command = "claude --permission-mode=auto"`. `auto` lets routine, low-risk actions
  through while still gating the dangerous ones — safer than `--dangerously-skip-permissions`,
  which turns *all* gates off (and, on current Claude Code, does **not** skip the trust
  dialog anyway).

The git worktree is the isolation boundary either way: an agent works on its own branch
in its own directory.

## Configuration

Config lives at `~/.config/birdseye/config.toml` (TOML). All keys are optional;
built-in defaults apply when absent. Example:

```toml
# Display order of candidate types in the picker.
order = ["tmux", "tmuxp", "dir", "repo"]

# Per-type display labels.
[labels]
tmux  = "session"
tmuxp = "template"
dir   = "dir"
repo  = "repo"

# Per-type icons shown in the picker. Defaults are Nerd Font glyphs; override
# with your own text/emoji, or set a type to "" to hide its icon. (Requires a
# Nerd Font for the defaults to render.)
[icons]
tmux  = ""
tmuxp = "󰏭"
dir   = ""
repo  = "󰘬"

# Enable/disable providers by type (omit a type to leave it enabled).
[providers]
repo = false

[tmuxp]
# dir = "/custom/tmuxp/config/dir"   # defaults to tmuxp's own config dir

[dir]
use_zoxide = true
roots = ["~/projects", "~/work"]

[repo]
# Optional discovery paths scanned for git repos. Each repo found directly under a root
# becomes one `repo` picker candidate that opens a session at the repo's primary worktree.
# Omit or leave empty to list none.
roots = ["~/code", "~/work"]

[agents]
refresh = "1s"                        # live-refresh interval (a Go duration)
command = "claude"                    # agent command for `n`, run in the window's shell
# command = "claude --permission-mode=auto"   # fewer permission prompts (see above)
# accent = "#c792ea"                  # agents-view accent (title + cursor); #rrggbb
# split = 0.4                         # the agents-list pane's share of the width when the
                                      # preview is shown: the list fills out to this share
                                      # (columns flex to fit, no clipping) and the preview takes
                                      # the rest. Default 0.4, clamped to 0.3–0.8

# Rebind the agents-view keys. Each action lists the keys that trigger it;
# omit an action to keep its default. A two-rune value of two typeable keys (e.g.
# "gg", "dd", "dD") is a chord — those two keys pressed in sequence. Defaults shown below.
[agents.keys]
up        = ["k", "up"]
down      = ["j", "down"]
top       = ["gg"]
bottom    = ["G"]
half_up   = ["ctrl+u"]
half_down = ["ctrl+d"]
select    = ["enter"]
new_session  = ["s"]                   # open a session via the picker, stay in the view
new_agent    = ["n"]
close        = ["dd"]                   # close the window (safe, no disk effect); "dd" is a chord
delete       = ["dD"]                   # close + git worktree remove, behind a popup; "dD" is a chord
quit      = ["q", "esc", "ctrl+c"]

[worktree]
# Global fallback setup command, run on every worktree creation when a repo does not
# define its own (see "Worktree provisioning" below). Empty means no global default.
# setup = "make bootstrap"
```

### Worktree provisioning

A freshly created worktree is a clean checkout, so anything git-ignored (your `.env`,
installed dependencies, generated files) is missing. birdseye provisions a new worktree at
creation time — on both `be worktree add` and `be agents spawn` / the dash `n` action — so
it is ready to work in. Provisioning runs only when a worktree is **created**, never when a
dormant slot is re-opened.

Two complementary, repo-root files drive it:

- **`.worktreeinclude`** — the cross-tool convention (also honored by Claude Code, Codex,
  Conductor, …). It uses `.gitignore` syntax and lists the local files to carry over from the
  repository's primary worktree. A file is copied only when it both matches a pattern **and**
  is git-ignored, so tracked files are never duplicated. Use it for small, irreplaceable local
  files (secrets, certs, local config) — not for `node_modules`/`.venv`, which the setup
  command should regenerate.

  ```text
  # .worktreeinclude
  .env
  .env.local
  config/secrets.json
  ```

- **`.birdseye/config.toml`** — a repo-local config (commit it) that uses the **same schema as
  the user config** and overrides any user-level setting for repository-scoped operations
  (`be worktree add`, `be agents spawn`, the dash `n`). A key the project sets wins; everything
  else is inherited. The `[worktree] setup` command runs in the new worktree after the copy
  step; a project can equally pin its own `[agents] command`, etc.

  ```toml
  # .birdseye/config.toml
  [worktree]
  setup = "./scripts/worktree-setup.sh"

  [agents]
  command = "claude --permission-mode=auto"   # this repo's agent, overriding the user default
  ```

The setup command runs with the new worktree as its working directory and these variables
exported: `BIRDSEYE_WORKTREE` (the new worktree's path), `BIRDSEYE_REPO` (the primary
worktree's path — the copy source), and `BIRDSEYE_BRANCH` (the branch checked out). On
`be worktree add` it runs inline and a non-zero exit fails the command (the worktree is kept
so you can fix and retry); pass `--no-setup` to skip it. On an agent spawn it is chained into
the agent's tmux window (`<setup> && <agent>`), so setup output is visible in the pane and the
agent starts only if setup succeeds.

## Claude Code integration (`be agents install`)

birdseye ships its Claude Code integration as **two independent, opt-in tiers** — you
choose what (if anything) to install, and the binary itself stays un-opinionated:

- **hooks** — status detection. Agent status shown in `be dash` and reported by `be
  agents` comes from state Claude Code hooks write; status is never inferred from pane
  contents. (The view polls that state to refresh live, and the preview pane reads
  terminal output for display only; neither affects status.)
- **workflows** — birdseye-authored **skills** (orchestrator, QA, PR) that compose the
  `be agents` verbs into opinionated playbooks. These are removable artifacts, never code
  in the binary.

```sh
be agents install claude --hooks               # status detection only
be agents install claude --workflows           # the skills only
be agents install claude --hooks --workflows   # both
be agents install claude                        # no flag → interactive checklist (in a terminal)
be agents install claude --hooks --settings <path>   # non-standard settings location

be agents uninstall claude --workflows         # symmetric, tier-scoped; removes only what we wrote
be agents uninstall claude                      # no flag → checklist of installed tiers
```

Nothing is ever installed implicitly: passing no tier flag shows a checklist (one line per
tier) in an interactive terminal, and in a non-TTY (piped/scripted) it **errors** asking
for `--hooks`/`--workflows` rather than guess. Selecting nothing is a no-op.

> **Deprecated alias:** `be hook claude install` / `uninstall` still work (hidden) and route
> to the hooks tier, so existing instructions keep functioning. Prefer `be agents install
> claude --hooks`.

### Removability

The two tiers are independent and the workflows tier is **removable by construction**.
Uninstalling the workflow skills — or never installing them — leaves the `be` binary, every
`be agents` verb, and the hooks tier fully functional; the skills only *compose* the verbs,
they are never required by them. The hooks tier is likewise independent: removing it does
not touch the skills. Both removals touch only what birdseye wrote.

### Hooks tier details

The hooks install is **safe**: it preserves every other key and any unrelated hooks you
already have, refuses to touch a malformed file, writes a `.bak` backup before
changing anything, and is idempotent. Uninstall removes only the entries
birdseye added. By default the installed hook calls the absolute path of the
`be` binary you ran (robust against PATH differences in Claude's hook
environment); override with `--command`. The workflows tier mirrors this safety for its
files: re-install rewrites only changed artifacts and backs up a hand-edited one to `.bak`
rather than clobbering it.

Each agent type is its own `<type>` argument (`be agents install claude …`), so support for
other agents can be added later without reshaping the command. The hooks install wires up
these events in your Claude settings:

```json
{
  "hooks": {
    "SessionStart":     [{ "hooks": [{ "type": "command", "command": "be hook claude record SessionStart" }] }],
    "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "be hook claude record UserPromptSubmit" }] }],
    "Notification":     [{ "hooks": [{ "type": "command", "command": "be hook claude record Notification" }] }],
    "Stop":             [{ "hooks": [{ "type": "command", "command": "be hook claude record Stop" }] }],
    "SessionEnd":       [{ "hooks": [{ "type": "command", "command": "be hook claude record SessionEnd" }] }]
  }
}
```

`be hook claude record <event>` (invoked by those hooks) reads the hook payload on stdin and records state under
`$XDG_STATE_HOME/birdseye/agents/` (one file per session). The default
event→status mapping:

| Hook event                                                       | Status           |
|------------------------------------------------------------------|------------------|
| `Notification` (permission prompt / unrecognized message)        | needs-attention  |
| `Notification` (idle "waiting for your input" nudge)             | idle             |
| `Stop`                                                           | idle             |
| `SessionEnd`                                                     | done             |
| `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`  | working          |

`Notification` is split by its message: Claude's periodic idle nudge maps to **idle**, while a
permission/approval prompt — or any message birdseye doesn't recognize — maps to
**needs-attention**, so an unfamiliar notification still surfaces to you.

The **working ↔ idle** distinction above is then corrected on every refresh from the agent
pane's current terminal title, which Claude updates live (a spinner glyph while working, a
sparkle when idle). Hooks are edge-triggered and can go stale - "working" lingers after an Esc
interrupt that fires no `Stop` - so the live title is the authority for working/idle, while
hooks remain the authority for needs-attention and done. When no title is readable, the
hook-written status stands.

Status not refreshed within `agents.stale_after` is shown as **unknown**.

## Adding a provider

A provider is any type implementing `internal/provider.Provider`:

```go
type Provider interface {
    Type() string                       // source tag, also the config key
    Candidates() ([]Candidate, error)   // attach-or-create entries
}
```

Each `Candidate` carries a canonical session `Name`, a `Label`, a `Type`, a
`Kind` (attach existing vs create new), and an `Action(Backend) error` that
materializes/connects the session. Register it in `internal/cli.buildRegistry`
and it appears in the picker — no changes to the picker or tmux code required.

## Development

```sh
just check     # go vet + go test ./...
just build     # build ./bin/be
just --list    # all recipes
```
