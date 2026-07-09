## Why

birdseye pushes an out-of-band notification every time any tracked agent finishes a turn
(`working -> idle` fires `edgeFinished`, delivered as "<agent> finished"). When a user is
orchestrating several agents, each child's finish fires its own ping, and a flat per-completion
alert trains the user to ignore all of them - including the one that matters, a real block. The
signal that actually warrants an interrupt is narrow: the agent is blocked on the user, or the
whole run has settled. Everything else is progress, and progress is state to be pulled from the
`be agents` view, not an event to be pushed.

## What Changes

- **A single tracked agent's finish no longer pushes on its own.** The `working -> idle` finish of
  a routine (detached/orchestrated, `Kind == "bg"`) agent updates its record for the pull surface
  and stays silent. **BREAKING**: users who relied on a ping per child-agent finish will now get
  the terminal digest instead.
- **Needs-attention stays the immediate escalation push, unchanged.** It is already
  structurally gated (it originates from a real permission/approval prompt or an unrecognized
  notification), so it is the one push that always fires. It is never batched or delayed.
- **A terminal digest replaces the flurry of per-finish pings.** When the set of working agents
  drains to empty (the run settles), birdseye emits one notification summarizing what finished -
  a count, with any anomalies named first - rather than one ping per agent.
- **`[agents.notify].finished` is redefined** from "any agent's working -> idle edge" to "the run
  settled" (the terminal digest). The escalation edge keeps its own flag. Config stays a two-flag
  table; the flags' meaning changes.
- The pull surface (`be agents`) remains the authoritative place to see per-agent progress; this
  change only governs what gets *pushed* out of band.

## Capabilities

### New Capabilities

- `agent-notifications`: The out-of-band notification policy - which agent status transitions are
  pushed to the user immediately (escalations), which settle into a coalesced terminal digest, and
  which stay silent as pull-only state. Covers edge classification, the run-settled digest,
  suppression rules (mute, session kind), and delivery. This behavior lives in code today
  (`internal/agents/notify.go`, `internal/agents/claude.go`) but has no governing spec.

### Modified Capabilities

<!-- No existing spec's requirements change; the emission policy was previously unspecced. -->

## Impact

- `internal/agents/claude.go`: `classifyEdge`, `maybeNotify`, `notificationFor`, `notifyPolicy`,
  `loadNotifyPolicy` - the edge/policy/delivery seam that decides what fires.
- `internal/agents/notify.go`: `Notification` shape and message construction; digest formatting.
- `internal/config/config.go` and `internal/config/notify_test.go`: `Notify` flag semantics.
- User-facing behavior change: fewer, more meaningful pings. Documented in DESIGN.md / config help.
- No new dependencies; delivery (auto notifier / user command template) is unchanged.
