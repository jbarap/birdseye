## Why

The `done` status is not useful in practice. At the agent grain (rows are deduped per tmux
location, not per session) a `done` row is transient by construction: either a new session
takes the pane and the row reads `working`/`idle` again, or the Claude process exits and the
liveness GC reclaims it on the next refresh. There is no durable "done" resting state, so the
`DONE` band is dead weight.

What users actually want is the inverse: a way to say "I've seen this agent's state and I
don't want it competing for my attention right now" - without killing it. And the existing
band order (`WORKING` above `IDLE`) contradicts the dashboard's own triage thesis: an idle
agent has yielded its turn and is waiting on you, so it wants your attention *more* than a
busy one that needs nothing.

## What Changes

- **BREAKING** Remove the `done` status: drop `StatusDone`, the `reconcile` done-latch, and
  the `SessionEnd -> done` mapping. A session ending no longer produces a status; the existing
  process-liveness GC removes the agent when its process exits.
- Reorder detected statuses by how much the agent wants attention: `needs-attention -> idle ->
  working`. The Agents-lens triage bands become `NEEDS YOU / IDLE / WORKING`.
- Add **mute**, a user-assigned flag that is *not* a status. Detection keeps running
  underneath; the flag only forces the agent into a fourth, lowest band (`MUTED`), where the
  row still shows its real working/idle glyph.
- Bind a rebindable **mute toggle** (default `m`) that flips the flag on the selected agent.
- **Snooze semantics**: a muted agent that transitions into `needs-attention` auto-clears its
  mute and returns to the `NEEDS YOU` band, so a genuine block can never stay hidden.

## Capabilities

### Modified Capabilities

- `agent-view`: the essential status set drops `done`; Claude hook sourcing no longer derives
  a `done` state; the liveness-GC "ended agent" scenario is reworded off `done`; a new manual
  mute toggle (rebindable command action) is added to the keymap and agent model.
- `dash-lenses`: the Agents-lens triage bands are reordered and the `DONE` band is replaced by
  a `MUTED` band holding user-muted agents.

## Impact

- `internal/agents/agent.go`: remove `StatusDone`; reorder `Status.rank()`; add a `Muted`
  field to `Agent`.
- `internal/agents/claude.go`: drop the `SessionEnd -> done` case and the `StatusDone`
  reconcile latch; persist and read the mute flag on the per-location record; clear it on a
  needs-attention transition.
- `internal/agents/view.go`: rename/reorder the triage bands, add the `MUTED` band, wire the
  `m` mute toggle and its help text.
- Keymap config: a new `mute` command action.
- No external API or CLI surface changes beyond the new key binding.
