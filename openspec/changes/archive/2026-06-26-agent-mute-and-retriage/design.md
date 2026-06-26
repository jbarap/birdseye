## Context

The dashboard's whole thesis is triage: order agents by how much they want the user's
attention. Two things undercut that today. First, the `done` status earns a permanent `DONE`
band but has no durable meaning - rows are deduped per tmux location (`dedupKey`), so a `done`
record is either superseded by a fresh session in the same pane or reclaimed by the
process-liveness GC moments later. Second, the band order puts `WORKING` above `IDLE`, which
inverts urgency: a working agent needs nothing, while an idle one has yielded and is waiting
on the user.

Separately, there is no way to say "I've seen this, leave it alone for now." Every status is
*detected* from hooks and the pane title via `reconcile`, recomputed each refresh. A user
intent has nowhere to live in that model.

## Goals / Non-Goals

**Goals**
- Remove `done` and let liveness GC own end-of-life, unchanged.
- Reorder triage to `needs-attention -> idle -> working`.
- Let the user mute an agent to the bottom band without affecting what it is actually doing.
- Make mute survive refreshes and a session rotating in place at the same location.

**Non-Goals**
- No change to how working/idle/needs-attention are *detected* (the title detector, the
  needs-attention latch, the batched tmux query all stay).
- No persistence of mute across a full process exit - a muted agent whose process dies is
  reclaimed like any other; mute is not a tombstone.
- No new agent-type abstraction; mute is generic to the `Agent` model.

## Decisions

### Mute is a flag, not a status

Every member of `Status` is detection-driven and recomputed each refresh. If mute were a
`Status` value, `reconcile` would overwrite it on the next poll from the pane title. So mute
is a separate persisted boolean (`Agent.Muted`, mirrored on the record), orthogonal to
`Status`. Detection keeps producing the real working/idle/needs-attention value underneath;
mute only changes *placement* (which band) and *priority* (lowest), never the agent's
displayed glyph. Unmuting is therefore instant truth - the real status was live the whole
time, so the agent drops straight back to its detected band with no stale restore.

### Banding and ordering

`Status.rank()` becomes `needs-attention(0) -> idle(1) -> working(2)`. The sort key gains a
higher-priority mute term: a muted agent sorts after every unmuted one regardless of its
detected rank. The Agents-lens bands become `NEEDS YOU / IDLE / WORKING / MUTED`. Inside the
`MUTED` band each row still renders its real status glyph, so the band is a *position*, not a
replacement state.

### Snooze semantics (mute auto-clear)

Mute means "park this at its current state, but wake me if something new demands me." A
transition **into** `needs-attention` (a permission/approval prompt) clears the mute and
returns the agent to `NEEDS YOU`. Working<->idle transitions do **not** clear it - those are
the agent doing its job, which is exactly what the user chose to ignore. This keeps a genuine
block from ever being hidden while still letting the user silence routine churn.

### Where mute persists (per-location, not per-session)

State records are keyed per session id, but the displayed agent is per tmux location (pane)
after dedup, and the mute action targets what the user sees. So the mute flag follows the
agent's location: when the user mutes the selected row, the flag is written such that a new
session rotating into the same pane inherits the muted state, and the flag is reclaimed by the
same liveness GC when the location's process exits. This avoids a mute silently evaporating
the instant Claude restarts in place.

### `SessionEnd` becomes a no-op for status

With `done` gone, `SessionEnd` no longer maps to a status. It may still update identity/liveness
bookkeeping, but it produces no band of its own; the process-liveness GC is the sole owner of
removing an ended agent.

### Muted agents and the Workspaces urgency badge

A repository section bar summarizes its most-urgent live agent status. A muted agent SHALL NOT
raise a section's badge - muting is a request to stop competing for attention, and letting a
muted agent still inflate a section badge would defeat it. A section whose only live agents are
muted renders without a status badge.

## Risks / Trade-offs

- **Mute is invisible to other surfaces.** It lives in the dashboard's model; a future
  notifier or status line must decide whether to honor it. Acceptable now; flagged for when a
  second consumer appears.
- **Per-location persistence is subtle.** Tying the flag to a pane that can be reused risks a
  stale mute on a genuinely new agent. Mitigated by snooze auto-clear (a new block un-mutes)
  and by GC reclaiming the location on process exit.
- **Removing `done` is breaking** for anything that reads the status set. The set is internal,
  so impact is contained to this repo.

## Open Questions

- Should the `MUTED` band be foldable by default (collapsed) so muted agents take zero vertical
  space until expanded? Leaning yes, but deferring to implementation feedback.
