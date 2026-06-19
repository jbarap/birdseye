## Why

`be agents` decides whether a tracked agent is still around with time-based heuristics:
state older than `stale_after` is shown as `unknown`, and records are garbage-collected only
after long `forget_done` / `forget_stale` TTLs. The list accumulates garbage, and — worse —
the signal is wrong: closing Claude Code while leaving its tmux pane open (a leftover shell)
leaves the entry lingering with a stale status, because pane existence answers "is there a
terminal here," not "is the agent alive." The agent *is* a process. The hook that writes the
state runs as a child of that process, so we can record the Claude process id and use its
liveness directly: when the process is gone, the agent is gone.

## What Changes

- Record the Claude process id in each state record. The hook runs (a few levels) below the
  Claude session, so it captures the session pid by walking up its ancestry to the `claude`
  process on every event — not its immediate parent, which is a per-hook shell that exits at
  once.
- Make process liveness the single cleanup signal: on each read, `be agents` deletes the
  state for any record whose process is no longer running. An entry exists exactly as long as
  its Claude process does — independent of tmux, so it works for agents that were never in a
  pane, and it reclaims an agent the moment Claude exits even if the pane stays open.
- **Remove** time-based retention entirely (`forget_done` / `forget_stale`) and the
  `stale_after → unknown` downgrade. There is no timer and no stale heuristic anymore.
- Reclaim records that cannot be tied to a live process — including legacy records written
  before pid tracking (no pid) — rather than letting them linger forever.
- **BREAKING (config)**: the `agents.stale_after`, `agents.forget_done`, and
  `agents.forget_stale` keys are retired. They are ignored if still present (configs do not
  break), but they no longer do anything.

## Capabilities

### New Capabilities
<!-- None: this changes the behavior of an existing capability. -->

### Modified Capabilities
- `agent-view`: The "Garbage collection of aged-out state" requirement is replaced — cleanup
  is now driven by Claude process liveness rather than retention windows. The "Claude Code
  status via hooks" requirement is updated: the hook records the process id, and a record whose
  process is gone is removed rather than shown as `unknown` (the `stale_after` downgrade is
  removed).

## Impact

- Code: `internal/agents/store.go` (new `pid` field on the record); `internal/agents/claude.go`
  (`HandleHook` captures `os.Getppid()`; `ClaudeSource.Agents` reclaims records whose process is
  dead via `syscall.Kill(pid, 0)`; removal of the TTL/stale logic and the `PaneLister` seam
  added in the previous draft); `internal/cli/agents.go` (no longer injects a pane lister);
  `internal/config/config.go` (removes the three duration knobs and the now-unused `Duration`
  type); `internal/tmux` (the unused `ListPanes` helper is dropped).
- Behavior assumes the Claude session process is named `claude` and is an ancestor of the hook
  process — true for the documented `be hook …` install. The immediate parent is a per-hook
  shell that exits at once, so we walk up to the `claude` process; an install that runs Claude
  under another name would need that name added to the match (see design Risks).
- No persisted-schema migration needed: old records simply lack a `pid` and are reclaimed on
  first read. Config schema shrinks but stays backward-compatible (unknown keys are ignored).
