## Why

Every agent action lives only inside the TUI as an in-process Go interface, so a human is the only
possible client. For an *orchestration agent* to coordinate other agents — spawn work, poll status,
send feedback — the same lifecycle operations must be a headless, composable surface. Per "the CLI
is the API," exposing agent management as `be agents` verbs lets anything that can run a shell (a
script, or another agent) drive the fleet, with no bespoke API.

## What Changes

- New headless verbs under `be agents`: `list`, `status`, `spawn`, `send`, `jump`, `close`,
  `delete` — emitting `--json` where they return data.
- **Addressing:** a unit of work is addressed by its worktree handle `repo/worktree` (durable across
  renames, tmux restarts, and agent respawns); a live/observed agent with no worktree by its tmux
  id. Handles are *derived*, so an agent bird's-eye did not spawn is addressable ("import")
  automatically — there is no `import` verb.
- `spawn` takes a **directory** (any repo on disk), ensures-or-reuses that repo's tmux session,
  creates a sibling worktree, opens a window, and starts the agent — optionally seeding a `--prompt`.
  Independently usable; not tied to pre-existing tmux state.
- `close` ≡ the dash's `dd` (close the window, keep the worktree); `delete` ≡ `dD` (remove the
  worktree too). Same vocabulary across surfaces.
- **Discovery:** the provider registry gains a headless face — `be sessions --json` lists launch
  candidates (repos/dirs) non-interactively, feeding `spawn`. fzf remains the interactive face of the
  *same* registry; nothing is duplicated.
- **BREAKING:** `be agents` becomes the headless verb namespace; the TUI moves to **`be dash`**; bare
  `be` is dropped (a subcommand is required).

## Capabilities

### New Capabilities

- `agents-cli`: the headless agent-management verb surface, `repo/worktree` handle addressing, and
  directory-based `spawn`.

### Modified Capabilities

- `be-cli`: command surface — `be dash` (TUI), `be agents` (verbs), bare `be` removed.
- `session-providers`: headless `--json` enumeration of candidates (the shared discovery core).
- `agent-view`: the TUI is invoked as `be dash` and its actions resolve to the same operations the
  verbs expose (lifecycle parity).

## Impact

- New `be agents` verb tree in `internal/cli`; the `internal/agents` orchestrator operations exposed
  headlessly behind shared handle resolution.
- `internal/picker` / session providers: non-interactive `--json` listing.
- `internal/cli/cli.go`: root restructure — drop bare `be`, add `be dash`, repurpose `be agents`.
- **Depends on** `worktree-sibling-layout` for git-native recognition and `repo/worktree` handles.
- `send` is best-effort (tmux `send-keys` has no readiness signal) — documented sharp edge, not
  solved here. `watch`/streaming is intentionally out of scope (poll `list`/`status`).
