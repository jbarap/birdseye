## Context

`ClaudeSource.Agents()` (`internal/agents/claude.go`) reads per-session JSON records on each
refresh and (a) prunes records past their TTL and (b) downgrades non-terminal records older
than `staleAfter` to `unknown`. Both are time-based guesses at "is this agent still here."

The agent is a Claude process. The hook that writes each record (`be hook claude record
<event>`) is spawned *by* that process, so the Claude session process is one of the hook's
ancestors. Recording its pid lets us answer presence directly — "is that process running?" —
which is exactly the question the heuristics were approximating. This supersedes the earlier
pane-existence draft: a tmux pane can outlive the agent (a closed Claude leaves a shell), so
pane liveness is the wrong signal; process liveness is the right one, and it needs no tmux.

A subtlety we hit in practice: Claude Code runs each hook inside a short-lived shell, so the
hook's *immediate* parent (`os.Getppid()`) is that shell, which exits the moment the hook
returns. Recording it would make a still-running agent's pid dead within milliseconds, and the
entry would vanish on the very next refresh. So we record the Claude process, found by walking
up the ancestry, not the immediate parent.

## Goals / Non-Goals

**Goals:**
- Record the Claude process id on every hook write.
- Reclaim a record the moment its process is gone, on the next read; keep it as long as the
  process runs, regardless of status age.
- Reclaim records with no usable process id (legacy) rather than leaking them.
- Remove time-based retention and the stale→unknown downgrade entirely.
- Work independent of tmux (process liveness needs no tmux server).

**Non-Goals:**
- No verifying *what* the process is (we trust that the hook's parent is Claude; we don't
  inspect its command line).
- No defending against pid reuse via start-time matching (accepted small risk, see Risks).
- No retention windows, no `unknown`-on-staleness, no pane enumeration.

## Decisions

### Liveness = the Claude session process is alive

`HandleHook` records `rec.PID = sessionPID()`, the pid of the Claude session process that owns
the hook. `ClaudeSource.Agents()` keeps a record iff `processAlive(r.PID)`:

```go
func processAlive(pid int) bool {
    if pid <= 0 { return false }            // no/!valid pid → not a live agent
    err := syscall.Kill(pid, 0)             // signal 0 = existence probe
    return err == nil || errors.Is(err, syscall.EPERM) // EPERM = exists, other owner
}
```

`syscall.Kill(pid, 0)` is the portable Unix existence check (Linux/macOS, which is the tool's
domain): `nil` = running, `ESRCH` = gone, `EPERM` = running but owned by another user. A
non-positive pid (legacy records, or a malformed value) is treated as not-alive, so those are
reclaimed. The check is injected as a `func(int) bool` field on the source (defaulting to
`processAlive`), mirroring the existing `now` seam, so tests are deterministic without spawning
real processes.

### Finding the Claude process: walk the ancestry

`sessionPID()` starts at `os.Getppid()` and walks up parents until it finds the process whose
command basename is `claude`, returning that pid. Parent/command lookup goes through `ps -o
ppid=,comm= -p <pid>`, which works on Linux and macOS (the tool's domain) and is also a package
var so a synthetic process tree can be injected in tests. The walk is bounded (≤32 hops, stops
at pid ≤ 1) so a malformed or cyclic chain can't loop. If no `claude` ancestor is found — an
unexpected install — it falls back to the immediate parent as a best effort.

We match the Claude process by command name rather than recording the immediate parent because
that parent is a per-invocation shell that has already exited by the time the next refresh runs
(verified in practice: the recorded pid was dead while the agent was plainly alive). The
matched process is the long-lived session, so its liveness tracks the agent's.

### Remove the heuristics and their config

The TTL prune (`expired`, `forget_done`, `forget_stale`) and the `staleAfter → unknown`
downgrade are deleted. `unknown` survives only as the value for a record that exists but has no
written status. The three `agents.*` duration config keys and the `Duration` config type are
removed; `toml.Decode` ignores unknown keys, so existing configs that still set them keep
loading (the keys are just inert).

### Drop the pane-liveness machinery from the previous draft

The `PaneLister` seam, `livePanes`, `SetPaneLister`, and `tmux.Client.ListPanes` added in the
pane-based draft are removed. Cleanup no longer touches tmux at all, which also means it works
for agents that were never in a pane.

Alternative considered: keep pane existence as a secondary signal for pid-less legacy records.
Rejected — it reintroduces the wrong signal (the exact lingering-shell case the user hit) and
the tmux dependency, to serve only transitional records that are cheaply reclaimed anyway.

## Risks / Trade-offs

- [Claude process not named `claude`] → We identify the session process by its command
  basename. If an install runs Claude under a different name (e.g. a `node`-based launch), the
  walk finds no `claude` ancestor and falls back to the immediate parent — the ephemeral shell —
  so a live agent's entry would blink out and return on its next hook event. This is directly
  observable in manual testing (entries flicker while Claude is idle). The native `be hook`
  install runs Claude as `claude`, which the walk handles. If another launcher name shows up,
  the fix is to add it to the match set or relax to "first non-shell ancestor."
- [Pid reuse] → A dead agent's pid could be reused by an unrelated process, making a gone agent
  look alive (it lingers). Far less harmful than wrongful deletion, and live agents re-stamp
  their pid on every hook event. Accepted for now; could add start-time matching later.
- [Pid-less legacy records reclaimed] → A pre-upgrade record (no pid) is removed on first read.
  A still-running pre-upgrade session re-appears on its next hook event; a stopped one is
  correctly gone. One-time upgrade behavior.

## Open Questions

- None. The Claude-ancestor assumption is documented and manually verifiable; everything else
  follows from process liveness.
