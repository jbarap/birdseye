# bird's-eye (`be`)

SHAMELESSLY VIBE CODED.

A lightweight, hackable bird's-eye view over your tmux sessions — and the AI
agents running inside them. `be` blends *existing* sessions with *ways to create
new ones* into one fuzzy picker, and gives you an at-a-glance triage view of your
agents.

Built on tmux (a proven backend), driven by `fzf`, and extensible through a small
provider interface.

## Prior art / references

bird's-eye draws on ideas/visuals/concepts from these projects:

- [sesh](https://github.com/joshmedeski/sesh) — smart tmux session manager and picker.
- [tmux-agent-status](https://github.com/samleeney/tmux-agent-status) — surfacing AI agent status in tmux.
- [superset](https://github.com/superset-sh/superset) — a workspace over agent sessions.
- [tsm](https://github.com/adibhanna/tsm) — terminal session manager; its minimal, titled panels
  shaped bird's-eye's panel chrome.

## Features

- **`be`** — a tmux-popup-friendly view of your Claude Code sessions and
  their status (needs-attention / working / idle / done), so you know which need
  you. It **updates live** as sessions change, shows a **preview** of the selected
  session's terminal, and navigates with **vim-native, configurable** keys. When a
  tmux session is a **managed repo** (see below) it also surfaces that repo's
  worktrees and lets you spin agents up and tear them down in place. Press `s` to
  open a new session from the same fuzzy picker without leaving the view.
- **One fuzzy picker** (`be sessions`) over running tmux sessions, `tmuxp` templates,
  `zoxide`/directory roots, and git worktrees — attach to what exists or create
  what doesn't.
- **`be worktree`** — clone a repo into `<parent>/<repo>/<default-branch>` (anywhere on
  disk) and spin up sibling worktrees from inside it; worktrees discovered under
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
be                    # agent status view (also `be agents`)
be sessions           # open the fuzzy session picker
be worktree clone <url> [parent]   # → <parent-or-cwd>/<repo>/<default-branch>
be worktree add <name> [branch]    # run from inside the repo; new sibling worktree
```

### Recommended tmux popup keybinding

Add to `~/.tmux.conf` to pop the agents view from anywhere:

```tmux
bind-key g display-popup -E -w 80% -h 60% "be"
```

The agents view refreshes in place as hooks report new state, so a popup left open
stays current. When the popup is wide enough it shows a live preview of the selected
session's terminal beside the list; on narrow popups the preview is hidden. Navigate
with vim keys — `j`/`k` (or arrows) to move, `gg`/`G` for top/bottom, `ctrl+d`/`ctrl+u`
for half-page — `enter` jumps to the selected agent's session, and `q` dismisses it.
`s` opens a new session via the fuzzy picker (the same one as `be sessions`) and drops
you back in the view with it listed — handy for opening a repo before spinning agents up.
All of these keys are configurable (see `[agents.keys]` below).

### Managed repos (agent orchestration)

`be agents` *recognizes* — it never takes over — the common worktree-per-agent layout.
A tmux session is a **managed repo** when one of its windows started in a
`<repo>/<default-branch>` directory (the structure `be worktree` creates). bird's-eye
owns no state for this: every refresh it re-derives the picture from tmux + git, so an
unrecognized session looks and behaves exactly as before.

In a managed repo the view shows, beside live agents:

- a `󰘬` indicator and worktree count on the session bar,
- a `⌂ base` row for the default-branch checkout,
- a `◌ slot` row for each worktree window with no agent (a spawn target).

Two actions become available (configurable, see `[agents.keys]`):

- **`n` — new agent**: opens a small form with two fields — **branch** and **worktree** —
  where the worktree directory auto-fills from a slugified branch name (`feature/login` →
  `feature-login`) until you edit it. `⏎` creates, `⇥` switches fields (so you can tweak
  the worktree name), `esc` cancels. It then creates the worktree on that branch, opens a
  tmux window rooted there, and runs the configured agent command (`[agents] command`,
  default `claude`) **inside that window's interactive shell**.
- **`dd` — delete agent**: always asks `(y/n)` first. On confirm it removes the agent's
  window; for a managed worktree it also runs `git worktree remove`, escalating to a
  second force confirmation when the worktree has uncommitted changes. An incidental agent
  (one not in a worktree) is just closed; the repo anchor can't be deleted.

Agents in non-managed sessions, and incidental shells inside a managed session, keep
working exactly as today — orchestration is purely additive.

The agent command is run *through the window's shell* (typed in with `tmux send-keys`),
not as tmux's bare child process. That means it inherits your normal per-directory
environment — `direnv`, shell profile, `PATH` — exactly as if you opened a pane in the
worktree and typed the command yourself. If you select a per-project tool config via
`direnv` (e.g. a `CLAUDE_CONFIG_DIR` set from an `.envrc`), the spawned agent picks it
up automatically.

### The Claude Code trust / permission prompt

A freshly created worktree is a directory Claude Code hasn't seen, so by default it
opens with *"Quick safety check: Is this a project you created or one you trust?"* and,
on each action, a permission prompt. Neither is a bird's-eye behavior — both come from
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

Config lives at `~/.config/birds-eye/config.toml` (TOML). All keys are optional;
built-in defaults apply when absent. Example:

```toml
# Display order of candidate types in the picker.
order = ["tmux", "tmuxp", "dir", "worktree"]

# Per-type display labels.
[labels]
tmux     = "session"
tmuxp    = "template"
dir      = "dir"
worktree = "worktree"

# Per-type icons shown in the picker. Defaults are Nerd Font glyphs; override
# with your own text/emoji, or set a type to "" to hide its icon. (Requires a
# Nerd Font for the defaults to render.)
[icons]
tmux     = ""
tmuxp    = "󰏭"
dir      = ""
worktree = "󰘬"

# Enable/disable providers by type (omit a type to leave it enabled).
[providers]
worktree = false

[tmuxp]
# dir = "/custom/tmuxp/config/dir"   # defaults to tmuxp's own config dir

[dir]
use_zoxide = true
roots = ["~/projects", "~/work"]

[worktree]
# Optional discovery paths scanned for managed repos (each a <repo>/<default-branch>
# container of git worktrees). Worktrees can live anywhere; these only feed the picker.
# Omit or leave empty to list none.
roots = ["~/code", "~/work"]

[agents]
refresh = "1s"                        # live-refresh interval (a Go duration)
command = "claude"                    # agent command for `n`, run in the window's shell
# command = "claude --permission-mode=auto"   # fewer permission prompts (see above)
# accent = "#c792ea"                  # agents-view accent (title + cursor); #rrggbb

# Rebind the agents-view keys. Each action lists the keys that trigger it;
# omit an action to keep its default. A two-letter value of the same key (e.g.
# "gg") is a chord — that key pressed twice. Defaults shown below.
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
delete_agent = ["dd"]                  # "dd" is the d key pressed twice (a chord)
quit      = ["q", "esc", "ctrl+c"]
```

## Claude Code hook setup (for `be agents`)

`be agents` derives status from state that Claude Code hooks write — status is never
inferred from pane contents. (The view does poll that state to refresh live, and the
preview pane reads terminal output for display only; neither affects an agent's
status.) Install the hooks once:

```sh
be hook claude install                     # merges into ~/.claude/settings.json
be hook claude install --settings <path>   # for a non-standard settings location
be hook claude uninstall                   # removes only bird's-eye's hooks
```

Install is **safe**: it preserves every other key and any unrelated hooks you
already have, refuses to touch a malformed file, writes a `.bak` backup before
changing anything, and is idempotent. Uninstall removes only the entries
bird's-eye added. By default the installed hook calls the absolute path of the
`be` binary you ran (robust against PATH differences in Claude's hook
environment); override with `--command`.

Each agent type is its own subcommand (`be hook claude …`), so support for other
agents can be added later without changing this interface. The install wires up
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
`$XDG_STATE_HOME/birds-eye/agents/` (one file per session). The default
event→status mapping:

| Hook event                                                       | Status           |
|------------------------------------------------------------------|------------------|
| `Notification` (permission prompt / unrecognized message)        | needs-attention  |
| `Notification` (idle "waiting for your input" nudge)             | idle             |
| `Stop`                                                           | idle             |
| `SessionEnd`                                                     | done             |
| `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`  | working          |

`Notification` is split by its message: Claude's periodic idle nudge maps to **idle**, while a
permission/approval prompt — or any message bird's-eye doesn't recognize — maps to
**needs-attention**, so an unfamiliar notification still surfaces to you.

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
