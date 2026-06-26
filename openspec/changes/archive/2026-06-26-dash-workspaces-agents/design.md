## Context

`be dash` today is a single section list (left) plus a preview pane (right, width set by
`split`). `groupRows` (internal/agents/view.go) groups rows by repository, sorts each
group's rows by `rowLess`, then orders the **groups** by `groupRank` - the lowest
(most-urgent) status rank among a group's live agents - with name as the tiebreaker.
Status ranks are `attn=0 < work=1 < idle=2 < done=3` (agent.go); a group with no live
agents is treated as `idle`.

The consequence is that section position encodes current urgency, and urgency is
volatile: a work↔idle transition changes a section's rank and moves it, so the section
you are watching jumps - often from the top to the alphabetical bottom of the idle tier -
exactly as its agent settles. No-agent sections share the idle tier and interleave with
real work by name.

This design splits the dash into two always-visible lenses over the same row set, each
with its own ordering contract, so topology (stable) and attention (volatile) stop
fighting over one sort key.

## Survey (what existing tools do)

- **mprocs / overmind / lazygit / lazydocker / k9s**: primary list position is **stable**
  (config order, category, or name). Status is a column, badge, or color - never the
  sort key. Live status changes do not reorder the list.
- **gh dash**: surfaces are user-configured *sections* (saved queries) over one item
  source - the "two lenses over the same rows" model.
- **email TUIs (aerc/neomutt)** and other triage surfaces: the one place auto-ordering is
  accepted. They sort by **recency (monotonic, only moves forward)**, with unread as a
  *flag*, not a position. They do **not** sort by a status enum that flickers.

Takeaway: stable position + status-as-badge is the near-universal convention; the only
defensible auto-ordering surface is a recency-ordered triage list. That is exactly the
shape we adopt - Workspaces is the stable list, Agents is the triage list, and even
Agents orders by recency-within-band rather than raw status rank.

## Goals / Non-Goals

**Goals:**
- Workspaces lens never reorders sections by status; a watched section holds position.
- "Most urgent on top" survives in the Agents lens without rows teleporting on a flicker.
- A non-managed workspace explains itself inline; managed and unmanaged repos share one
  shape.
- Both lenses are views of one row set with linked selection.

**Non-Goals:**
- No change to how status is *detected* (hooks + OSC title already land that).
- No new tmux backend queries beyond existing status/topology data.
- Not introducing user-configurable lens definitions (gh-dash style) yet - two fixed
  lenses for now; the seam should not preclude it later.
- Not redesigning the preview pane.

## Decisions

**1. Two always-visible lenses, not a toggle.** The user chose always-visible. Precedent
(mprocs/lazygit) says an always-on second panel removes context-switching and is cheap.
Toggle is retained only as the narrow-terminal fallback, not the primary mode.
*Alternative considered:* tab-toggle as primary (claude-squad/gh-dash) - rejected because
the whole point is seeing topology and triage at once.

**2. Workspaces orders by a stable key (name); status is a badge.** Section position
becomes identity, not urgency. The most-urgent status among a section's agents renders as
a colored left-edge / count badge on the bar (`◐2`, `○1`), pairing color with a glyph per
the design language. *Alternative:* keep an urgency float but debounce it - rejected;
debounce only slows the churn, it doesn't remove the "watched thing moves" failure.

**3. Agents lens is banded, recency within band.** Fixed bands in fixed order
`NEEDS YOU / WORKING / IDLE / DONE`. Within a band, sort by last-status-change time,
newest first. This keeps urgent work at the top (its band is first) while a flicker only
moves a row *between* bands, never past peers inside one. *Alternative:* raw `status.rank`
sort (the user's literal first proposal) - rejected after survey; that is the churn we are
escaping, just relocated to the second pane. *Alternative:* recency with no bands -
rejected; bands give the at-a-glance "anything need me?" read that pure recency loses.

**4. Linked cursors over one row set.** One model owns the rows; each lens is a
projection (ordering + grouping) of indices into that set. Focus lives in one lens at a
time (a key switches focus between lenses); the non-focused lens shows the same selected
agent highlighted, and the preview follows the focused selection. This avoids two
independent cursors the user must mentally reconcile.

**5. Recency reuses the existing `Row.Updated`; the incidental reason is derived, not
stored.** Recency ordering needs a monotonic per-agent last-status-change time - as built,
the existing `Row.Updated` (already populated on both the plain and orchestration paths)
serves this, so no new field was added. The inline non-managed hint is likewise a derived
helper (`incidentalReason`, a constant for `GitDir == ""`) rather than a stored
`ManagedReason`, since git recognition is re-derived each tick and a stored field would only
duplicate `GitDir`. (This corrects an earlier draft that proposed new `LastChanged` /
`ManagedReason` row facts.)

**6. Focus model: per-lens cursor, spatial switch, mirrored highlight (resolves Q1).**
Each lens keeps its own cursor (lazygit/k9s panel model). One lens is focused at a time;
because the lenses sit side by side - **Agents on the left, Workspaces on the right** - `h`/
left switches focus to Agents and `l`/right to Workspaces, while `j`/`k` navigate within the
focused lens. The dashboard **opens focused on the Agents lens** so triage is the default
read. The focused lens drives the preview; its selection is mirror-highlighted (a faint
accent cursor glyph) in the other lens when a counterpart agent exists. Switching focus lands
the cursor on that counterpart when one exists (on the band/section header when it sits in a
folded band or section), else on the band top / last position. Selections with no counterpart
(empty slot, section header, `⌂ base` anchor) show no mirror - gracefully handling that
Workspaces carries non-agent rows while Agents is agents-only. `tab` folds within the focused
lens (Decision 9). *Alternative:* a single shared cursor that physically hops between two
differently-ordered lists - rejected as spatially confusing (which list does `down` advance?).

**7. Band headers are always rendered as section bars, faint when empty (resolves Q2).**
Positional stability is the thesis of this change and applies to bands too: fixed furniture
builds muscle memory, and an empty `NEEDS YOU` header is a genuine "all clear" signal. The
four-line cost is acceptable. As built, every band header renders as a full-width section bar
sharing the Workspaces lens's bar treatment (so a band and a repository header read as the
same kind of thing): a populated band is bold with a right-pinned count, an empty band keeps
the bar but goes faint with its label aligned to the populated ones. *Alternative:* vanish
empty bands (more compact; a band appearing is arguably a meaningful event) - not chosen,
because the goal is to stop the surface changing shape under the user. *Alternative:* plain
dim text for headers (the first cut) - rejected because it read as text, not sections.

**8. `done` lifetime is governed by the existing liveness GC, not a new timer (resolves
Q3).** A `done` agent stays in the `DONE` band exactly as long as its Claude process is
alive; when that process exits, the existing "Garbage collection of aged-out state" GC
reclaims it and it vanishes. That requirement is explicit that the system SHALL NOT use
time-based retention windows, so a triage-specific TTL is ruled out. A done-but-running
agent lingering in `DONE` is correct - it is still resumable. (This corrects an earlier
draft of this design that floated a TTL.)

**9. Both lenses fold (revised); within-section order goes stable too.** The existing
"Foldable sections" requirement carries over to the Workspaces lens, where it matters more
now that two lenses share vertical space. As built, the Agents lens **also folds**: `tab`
collapses the band (Workspaces section) under the cursor in the focused lens, each onto a
navigable header stand-in carrying the same `▾`/`▸` glyph and a hidden count. This was a
revision of the original "Agents stays flat" decision - once the bands render as real section
bars (Decision 7), folding them is the consistent affordance and reclaims vertical space by
collapsing `IDLE`/`DONE` to keep the actionable bands on screen; the fixed four-band order is
unchanged, so positional stability holds. An empty band has nothing to collapse and is not
foldable. Consequence: with status no longer driving order, within-section worktree rows in
Workspaces also go stable - `⌂ base` anchor first, worktrees by name, `◌ slot` rows last,
status shown as a per-row badge (previously they were status-ranked, the same churn at the
row level).

## Risks / Trade-offs

- **Width pressure** → Two lenses + preview need real estate. Mitigation: the
  Workspaces/Agents pair gets the `split` width; preview collapses first, then the
  narrow-terminal tab-toggle fallback engages below a threshold.
- **Recency timestamp source** → If status detection can't supply a reliable
  last-changed time, recency degrades. Mitigation: fall back to name ordering within a
  band when timestamps are absent (still stable, just not recency-ranked).
- **Two focusable lists complicate nav/keymap** → The current single-list cursor/nav and
  fold logic must grow a notion of "active lens." Mitigation: model the lenses as two
  projections sharing selection state; a single focus-switch key, reuse existing
  per-list nav within each.
- **Losing the old "one urgent thing jumps to the very top of everything" affordance** →
  Some users rely on attn dominating the whole screen. Mitigation: the `NEEDS YOU` band
  is always the first thing in the Agents lens, and the Workspaces badge still flags it;
  attention is more discoverable, just not by hijacking section order.

## Open Questions

All four prior open questions are resolved in Decisions 6-9. The two remaining unknowns are
now resolved as built:

- **Mirror-highlight styling** → resolved: the non-focused lens shows the counterpart with a
  faint accent cursor glyph (the focused cursor is the full accent glyph), reusing `Accent`
  + faint rather than adding a theme token.
- **Lens-switch when the counterpart is folded** → resolved: the cursor rests on the folded
  band/section header (the section is left collapsed), in both lenses, consistent with the
  existing fold-navigation behavior.
