## Context

birdseye's out-of-band notifications are emitted from the hook path in
`internal/agents/claude.go`. Each hook invocation writes the agent's record, then
`maybeNotify` classifies the `(old -> new)` status transition into an `edge`:
`edgeNeedsAttention` (entering a block) or `edgeFinished` (`working -> idle`). Both push a single
line - "<agent> needs you" / "<agent> finished" - through the notifier selected by
`[agents.notify].command` (a user command template) or the auto-detected platform notifier.

Records are flat: one JSON file per session under the agents state dir, with a `Status` and a
`Kind` (`"bg"` for detached/orchestrated/daemon sessions, empty for interactive). There is no
parent/child link between an orchestrator and the agents it spawns. All agents a user is running
are visible as sibling records.

The problem (see proposal): `edgeFinished` pushes once per agent per turn. Under orchestration
that is a flurry of "<agent> finished" pings, none individually actionable, which trains the user
to ignore the channel - including the escalation that matters. Notification policy design should
follow the invariant *every push demands attention*; the moment one push is safely ignorable, all
of them are.

## Goals / Non-Goals

**Goals:**

- Keep exactly one always-on immediate push class: entering needs-attention (the block).
- Stop pushing per-agent finishes; replace the flurry with a single push when the run settles.
- Derive "the run settled" from the existing flat record set - no new parent/child model.
- Keep delivery, mute, and the config table shape unchanged; only the finish edge's meaning moves.

**Non-Goals:**

- No per-agent orchestration tree or explicit parent/child links on records.
- No learned/adaptive throttling or dismissal-based tuning - an unpredictable push policy erodes
  trust faster than noise; the contract must be stateable in one sentence.
- No changes to the `be agents` pull surface rendering; it is already the progress surface.
- No change to how needs-attention is *detected* (that lives in agent-status-detection); this
  change only governs what is *pushed*.

## Decisions

### Decision: "Run settled" = the live working set draining to empty

On a finish edge (`working -> idle`), `maybeNotify` reads the sibling records and asks whether any
*other live* agent is still `StatusWorking`. If yes, the finish is held (no push). If no - this
agent was the last to settle - it flushes one digest push. This bounds a "run" without any
parent/child link: any interleaving of finishes across an orchestrator and its children collapses
to a single push at the moment the whole set goes quiet.

- *Why not a debounce window?* A time window either delays the digest or fires early; the settling
  point is known exactly from the record set, so read it rather than guess at it.
- *Why not require `Kind == "bg"` to suppress?* The operative condition is "siblings still
  working," not session kind. Folding the interactive agent into the same drain rule means a solo
  user driving one agent gets one digest at turn end (same cardinality as today), and an
  orchestrator's own finish participates naturally. `Kind` stays a *refinement input* only (see
  the working-set liveness decision), not the gate.

### Decision: A small batch file accumulates what finished, flushed at drain

To let the digest say more than "something finished," each held finish appends the agent's title
to a batch file at the state root (sibling to `mutes.json`), e.g. `notify-batch.json`. When the
working set drains, `maybeNotify` reads the batch, composes the digest (count of routine finishes,
with any agent currently in needs-attention named first), pushes it, and clears the batch. The
batch is birdseye-owned derived state, best-effort like the notification itself.

- *Why a file, not in-memory?* Hooks run as independent short-lived processes; there is no shared
  memory across them. The batch must persist between hook invocations, exactly like records and
  mutes already do.
- *Concurrency:* writes use the same atomic-rename discipline as record writes. A lost or
  duplicated batch entry only affects a cosmetic count in one digest; it never blocks work or
  drops an escalation, so best-effort is acceptable.

### Decision: The working set is filtered by liveness

A crashed agent whose record is stuck at `StatusWorking` would otherwise suppress *every* future
digest forever. When computing "is any other agent still working," `maybeNotify` MUST count only
*live* records, reusing the existing liveness check (`recordAlive` / the pid+start-time GC that
already backs the view). A dead "working" record does not hold the run open. This is the one place
`Kind` is consulted: a `bg` daemon that is persistently idle simply never enters the working set,
so it never participates - no special-casing needed.

### Decision: Escalation keeps its existing structural gate

needs-attention already originates only from a real cause - a permission/approval prompt, or a
notification message birdseye does not recognize as the benign "waiting for input" (see
agent-status-detection / agent-view). That is exactly the "an escalation must carry a concrete
cause" gate. So the escalation class needs no new gating code; the spec codifies the property, and
`classifyEdge`'s existing "transition *into* the state" rule already prevents re-notifying a
re-fired block.

### Decision: `[agents.notify]` keeps two flags; `finished` is redefined

The table stays `{ needs_attention, finished, command }`. `needs_attention` still gates the
escalation push. `finished` now gates the terminal digest rather than per-agent finishes. Keeping
the key name avoids breaking existing config files; the semantic shift is documented in the config
comment and DESIGN.md. (Alternative considered: rename to `settled`/`digest` for honesty - rejected
as a config break for a change users can absorb via the redefinition, but noted as an open
question if we prefer clarity over compatibility.)

## Risks / Trade-offs

- **Stale `working` record suppresses the digest forever** → count only live records via the
  existing liveness GC; a dead agent cannot hold the run open.
- **Concurrent hooks race on the batch file** → atomic-rename writes; accept that a rare lost/dup
  entry only skews one digest's count, never an escalation.
- **Behavior change surprises users expecting a ping per child** → documented as BREAKING; the
  digest still tells them the run finished, and needs-attention still fires immediately.
- **Solo interactive user gets a digest of one per turn** → same cardinality as today's single
  finish ping, and still gated by the `finished` flag; not a regression.
- **Digest arrives only when the last agent settles** → an orchestration that never fully quiets
  (a persistent bg agent kept working) never digests; acceptable, because such a run has not
  settled, and any *block* within it still escalates immediately.

## Migration Plan

1. Redefine the finish edge in `maybeNotify`: hold on non-empty live working set, flush-digest on
   drain; add the batch-file accumulation and the digest message builder in `notify.go`.
2. Update `notifyPolicy`/`loadNotifyPolicy` comments and `[agents.notify].finished` semantics in
   `config.go`; adjust `notify_test.go` expectations.
3. Update DESIGN.md / config help to state the one-sentence contract: *silence means it is
   working, a ping means you are needed, the digest means it is done.*
4. Rollback is a pure revert - no persisted schema migration beyond an ignorable batch file that a
   reverted binary simply never reads.

## Open Questions

- Keep the config key `finished` (compatibility) or rename to `settled`/`digest` (clarity)?
- Digest granularity: count only, or list agent titles up to a cap before collapsing to "+N more"?
- Should a persistent `bg` daemon that is *intentionally* long-lived be excluded from the
  working-set liveness count by policy, or is "never settles, never digests" the right behavior?
