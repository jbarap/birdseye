# birdseye (`be`)

> Use tmux? Use agents? Need order? Get a bird's-eye view.

**Shamelessly vibe coded. Frequent breaking changes.**

`be` gives you one view over your tmux sessions and the AI agents running inside them:
a fuzzy picker for attaching to or creating sessions, and a live triage view of what
each agent is doing. Built on tmux, driven by `fzf`.

## Install

```sh
just install       # build `be` into your GOBIN
# or: just build  → ./bin/be (static, copy anywhere)
```

Needs `tmux` and `fzf`. `tmuxp`, `zoxide`, and `git` add features if present.

## Use

```sh
be dash        # live TUI of your agents and their status
be sessions    # picker: attach to a session or make a new one
be worktree    # grouped-sibling git worktrees from inside a repo
be agents ...  # headless agent lifecycle for scripts (list/spawn/send/close/…)
```

In `be dash`, vim keys navigate; `n` spawns an agent in a new worktree, `dd` closes it,
`dD` deletes the worktree, `s` opens a session. All keys are configurable.

For Claude Code status and notifications, install the hooks:

```sh
be agents install claude
```

## Configure

Optional, at `~/.config/birdseye/config.toml`. `be config` shows everything:

```sh
be config              # effective config
be config --defaults   # annotated template of every option
be config --check      # validate your config
```

## Develop

```sh
just check    # go vet + go test ./...
just build    # build ./bin/be
just --list   # all recipes
```

The visual language lives in [DESIGN.md](DESIGN.md); adding a session source means
implementing one small `internal/provider.Provider` interface.

## References

Birdseye draws lots of stuff from these great projects:
- [sesh](https://github.com/joshmedeski/sesh)
- [tmux-agent-status](https://github.com/samleeney/tmux-agent-status)
- [herdr](https://github.com/ogulcancelik/herdr)
- [agentapi](https://github.com/coder/agentapi)
- [superset](https://github.com/superset-sh/superset)
- [tsm](https://github.com/adibhanna/tsm)
