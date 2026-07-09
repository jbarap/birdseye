## 1. Working-set and batch state

- [x] 1.1 Add a helper in `internal/agents` that reports whether any *live* tracked agent other
      than the finishing one is `StatusWorking`, reusing the existing liveness check
      (`recordAlive` / pid+start-time GC) so a dead "working" record is not counted.
- [x] 1.2 Add a best-effort batch store at the state root (sibling to `mutes.json`) with
      append-a-finish and read-and-clear operations, using the atomic-rename discipline that
      record writes already use.
- [x] 1.3 Unit-test the working-set helper (live vs stale working records) and the batch store
      (append, read-and-clear, missing-file returns empty).

## 2. Edge and digest logic

- [x] 2.1 In `maybeNotify`, on `edgeFinished`: append the agent to the batch and return without
      pushing when the live working set is non-empty; flush a digest and clear the batch when it
      is empty.
- [x] 2.2 Add a digest message builder in `internal/agents/notify.go` that composes one line from
      the batch - routine finish count with any agent currently in needs-attention named first.
- [x] 2.3 Leave `edgeNeedsAttention` on the immediate path unchanged; confirm the existing
      "transition *into* the state" rule still prevents re-notifying a re-fired block.
- [x] 2.4 Ensure mute still suppresses both the escalation and the digest before any push.

## 3. Config semantics

- [x] 3.1 Redefine `[agents.notify].finished` to gate the terminal digest (not per-agent
      finishes); update the field comment in `internal/config/config.go` and keep the two-flag
      table shape and defaults (both classes on).
- [x] 3.2 Update `internal/config/notify_test.go` expectations to the redefined `finished`
      semantics; keep the unknown-key-rejected and explicit-false-wins coverage.

## 4. Behavior verification and docs

- [x] 4.1 Add tests covering: a mid-run finish is silent, the last finish flushes exactly one
      digest, an escalation fires immediately even while agents are working, and a needs-attention
      agent is named first in the digest.
- [x] 4.2 Drive the flow end to end via the use-tty skill: spawn multiple agents in `_birdseye_dev`,
      confirm no per-child pings, one digest on settle, and an immediate escalation on a block.
- [x] 4.3 Update DESIGN.md / config help with the one-sentence contract: silence means it is
      working, a ping means you are needed, the digest means it is done.
- [x] 4.4 Run the repo quality checks (build, `go test ./...`, theme guard) and fix any fallout.
