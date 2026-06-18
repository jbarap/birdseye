## Why

Switching between tmux sessions today means manually scanning tmux's built-in
session list and remembering which `tmuxp` template maps to which project — there
is no unified, fuzzy-searchable view that blends *existing* sessions with *ways to
create new ones*. As AI coding agents proliferate, several Claude Code sessions run
in parallel with no at-a-glance signal of which need attention, are working, or are
done. Bird's-eye (`be`) gives a single, lightweight, hackable entry point: a
bird's-eye view over sessions and the agents inside them, built on tmux as a proven
backend.

## What Changes

- Introduce a new Go CLI, `be`, that produces a single portable, self-contained binary.
- Add a picker (`be` / `be list`) that lists session *candidates* grouped by type —
  existing tmux sessions to attach/switch to, plus ways to create new sessions — and
  dispatches the chosen action. The picker shells out to `fzf`, which is a hard
  requirement.
- Define a **modular provider architecture**: a small interface that any source can
  implement to contribute session candidates. Ship built-in providers for running
  tmux sessions, `tmuxp` templates, and `zoxide`/recent directories.
- Add a tmux backend that materializes a selection into a live session (create, attach
  when outside tmux, switch-client when inside) idempotently by session name.
- Add `be agents`: a popup-friendly view (e.g. `tmux display-popup -E be agents`) of
  AI agent sessions with essential status (needs-attention / working / idle / done),
  built on an agent abstraction so multiple agent types can plug in; the Claude Code
  implementation reports status via Claude Code hooks. Designed to grow toward richer
  orchestration later.
- Add `be worktree`: git worktree management that clones a repo into `<repo>/main` and
  creates sibling worktrees (`<repo>/wt1`, …), surfaced as a session provider.
- Establish config (`~/.config/birds-eye/config.toml`) for enabling providers,
  ordering/labeling candidate types, and per-provider options — keeping the tool
  hackable and extensible.

## Capabilities

### New Capabilities
- `be-cli`: The `be` binary, command surface, configuration loading, and the provider
  registry / extension model that wires everything together.
- `session-picker`: The fuzzy-finder TUI that aggregates candidates from providers,
  groups/labels them by type, and routes the selected candidate to its action.
- `session-providers`: The provider interface plus built-in providers (running tmux
  sessions, `tmuxp` templates, `zoxide`/directory sources) that enumerate existing
  sessions and offer new-session creators.
- `tmux-backend`: Operations that create, attach to, and switch between tmux sessions
  idempotently, abstracting the tmux CLI from the rest of the tool.
- `agent-view`: The `be agents` overview that detects AI agent sessions and reports
  their essential status for quick triage.
- `worktree-provider`: Git worktree lifecycle (clone-to-`main`, create sibling
  worktrees) exposed as a session provider.

### Modified Capabilities
<!-- None — this is a greenfield project with no existing specs. -->

## Impact

- **New repository scaffold**: Go module, `cmd/be` entrypoint, `internal/` packages
  (config, providers, picker, tmux, agents, worktree), build tooling.
- **External dependencies**: `fzf` (required, drives the picker) and tmux (required
  runtime); a TUI library (Bubble Tea/Lipgloss) for the `be agents` view; optional
  `tmuxp` / `zoxide` / `git` binaries probed at runtime and degraded gracefully when
  absent. Claude Code hooks are configured to feed the agent view.
- **User environment**: introduces a config directory and a recommended tmux popup
  keybinding for `be agents`; no changes to existing tmux/tmuxp configs are required.
- **No existing systems modified**: greenfield; nothing is removed or broken.
