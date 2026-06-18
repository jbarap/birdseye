# bird's-eye (`be`)

SHAMELESSLY VIBE CODED.

A lightweight, hackable bird's-eye view over your tmux sessions — and the AI
agents running inside them. `be` blends *existing* sessions with *ways to create
new ones* into one fuzzy picker, and gives you an at-a-glance triage view of your
agents.

Built on tmux (a proven backend), driven by `fzf`, and extensible through a small
provider interface.

## Features

- **One fuzzy picker** (`be`) over running tmux sessions, `tmuxp` templates,
  `zoxide`/directory roots, and git worktrees — attach to what exists or create
  what doesn't.
- **`be agents`** — a tmux-popup-friendly view of your Claude Code sessions and
  their status (needs-attention / working / idle / done), so you know which need
  you.
- **`be worktree`** — clone a repo into `<root>/<repo>/main` and spin up sibling
  worktrees, each surfaced as a session candidate.
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
be                    # open the picker (same as `be list`)
be agents             # agent status view
be worktree clone <url>
be worktree add <repo> <name> [branch]
```

### Recommended tmux popup keybinding

Add to `~/.tmux.conf` to pop the agents view from anywhere:

```tmux
bind-key g display-popup -E -w 80% -h 60% "be agents"
```

`enter` in the view jumps to the selected agent's session; `q` dismisses it.

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

# Enable/disable providers by type (omit a type to leave it enabled).
[providers]
worktree = false

[tmuxp]
# dir = "/custom/tmuxp/config/dir"   # defaults to tmuxp's own config dir

[dir]
use_zoxide = true
roots = ["~/projects", "~/work"]

[worktree]
root = "~/code"                       # managed repos live here

[agents]
stale_after = "5m"                    # status older than this shows as "unknown"
```

## Claude Code hook setup (for `be agents`)

`be agents` reads status that Claude Code hooks write — there is no polling or
pane-scraping. Install the hooks once:

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
| `Notification`                                                   | needs-attention  |
| `Stop`                                                           | idle             |
| `SessionEnd`                                                     | done             |
| `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`  | working          |

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
