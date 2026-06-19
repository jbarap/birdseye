## 1. Record the Claude process id

- [x] 1.1 Add a `pid` field (`PID int json:"pid"`) to the on-disk `record` in
  `internal/agents/store.go`.
- [x] 1.2 In `HandleHook`, set `rec.PID = sessionPID()` on every event so the Claude session
  process is recorded.
- [x] 1.3 Add `sessionPID()` + `walkToClaude()`: walk up from `os.Getppid()` (the ephemeral
  per-hook shell) to the nearest ancestor whose command basename is `claude`, via an injectable
  `procParent` (`ps -o ppid=,comm=`); bound the walk and fall back to the immediate parent if no
  Claude ancestor is found.

## 2. Process-liveness cleanup

- [x] 2.1 Add `processAlive(pid int) bool` using `syscall.Kill(pid, 0)` (nil/EPERM = alive;
  non-positive pid = not alive) and an injectable `alive func(int) bool` field on
  `ClaudeSource` defaulting to it.
- [x] 2.2 In `Agents()`, delete the state file for any record where `!s.alive(r.PID)` and drop
  it from the list; keep the dedup/order steps.

## 3. Remove the heuristics

- [x] 3.1 Delete the TTL prune (`expired`, `forget_done`, `forget_stale`) and the
  `stale_after → unknown` downgrade; keep `unknown` only as the default for a record missing a
  status.
- [x] 3.2 Remove the `PaneLister` seam / `livePanes` / `SetPaneLister` and the now-unused
  `tmux.Client.ListPanes`; update `internal/cli/agents.go` to stop injecting a pane lister and
  to call the new `NewClaudeSource()`.
- [x] 3.3 Remove the `stale_after` / `forget_done` / `forget_stale` keys and the `Duration`
  type from `internal/config/config.go`; confirm old configs still load (unknown keys ignored).

## 4. Tests & verification

- [x] 4.1 Unit-test `processAlive` (current pid alive; pid 0/-1 not alive; a reaped child pid
  not alive). Unit-test `walkToClaude` over a synthetic tree (shell→claude→shell resolves to the
  claude pid; a chain with no claude reports not-found).
- [x] 4.2 Unit-test `Agents()` with an injected `alive`: a live-process record is kept; a
  dead-process record and a pid-less legacy record are deleted (files removed).
- [x] 4.3 Update config tests (defaults/override no longer assert the removed durations; a
  retired key still loads); run `go test ./...` and `go vet ./...`.
- [ ] 4.4 Manual: close Claude Code in a still-open tmux pane and confirm its `be agents` entry
  disappears on the next refresh; confirm a running session's entry stays put while idle.
