## 1. Remove the `done` status

- [x] 1.1 Delete `StatusDone` from `internal/agents/agent.go` and reorder `Status.rank()` to
  `needs-attention(0) -> idle(1) -> working(2)` (unknown last).
- [x] 1.2 In `internal/agents/claude.go`, drop the `SessionEnd -> StatusDone` case in `StatusFor`
  and the `StatusDone` short-circuit in `reconcile`; make `SessionEnd` write no status (identity/
  liveness bookkeeping only). *(HandleHook now guards the status write: `if event != "SessionEnd"`,
  so a dying agent keeps its last status instead of flashing to the `working` default.)*
- [x] 1.3 Update any code that branches on `StatusDone` (status glyphs/labels, `dropSupersededByPane`
  comment, view band assignment) so removing the value compiles and reads cleanly. *(Dropped the
  `StatusDone` entries from `statusStyle`/`statusGlyph`/`statusWord`; rewrote `bandOf` as `bandOfRow`.)*
- [x] 1.4 Update/remove tests that assert `done` (e.g. `claude_test.go` SessionEnd cases, the
  "ended agent" GC test wording) to assert reclamation-without-a-status instead.

## 2. Reorder the Agents-lens triage bands

- [x] 2.1 Change the band order/labels in `internal/agents/view.go` to `NEEDS YOU / IDLE /
  WORKING / MUTED`; remove the `DONE` band.
- [x] 2.2 Update `bandOf` (or equivalent) so a muted agent maps to the `MUTED` band regardless of
  its detected status, and a non-muted agent maps by status only. *(`bandOfRow(Row)`.)*
- [x] 2.3 Update band-structure tests to pin the new four-header order and the idle-above-working
  ordering. *(`TestAgentsByBand` gains a MUTED case; `TestAgentsLensStructure` keys off `numBands`.)*

## 3. Mute flag on the agent model

- [x] 3.1 Add a `Muted bool` field to `Agent` and to the persisted record, keyed so it follows the
  agent's tmux location. *(Mute is stored in a separate location-keyed store (`mutes.json`) rather
  than on the per-session record, so it survives a session rotating into the same pane;
  `locationKey` is shared by `dedupKey` and `Row.muteKey` so they cannot drift.)*
- [x] 3.2 In the status pipeline, clear `Muted` when an agent transitions **into** needs-attention
  (snooze auto-unmute); leave it set across working<->idle transitions. *(Stateless: `applyMutes`
  force-unmutes any agent whose current status is needs-attention, which satisfies both scenarios.)*
- [x] 3.3 Make the displayed-agent sort treat muted as the highest-priority term (muted after every
  unmuted agent), independent of `Status.rank()`.
- [x] 3.4 Ensure the muted row still renders its real status indicator (mute changes band/priority,
  not glyph). *(Rows keep their real `Status`; only band placement changes.)*
- [x] 3.5 Confirm the existing liveness GC reclaims a muted agent's location like any other.
  *(`applyMutes` prunes mute keys with no live agent; covered by `TestMutePersistsSnoozeAndReclaims`.)*

## 4. Mute toggle key

- [x] 4.1 Add a `mute` command action to the keymap (default `m`), wired through the existing
  keymap resolver/config like other command actions.
- [x] 4.2 Toggle `Muted` on the selected agent and persist it; reflect the binding in the help line.
  *(`toggleMute` via the optional `Muter` seam; `helpOrder`/`actionLabel` updated.)*
- [x] 4.3 Verify `m` is not already bound; if it is, pick a free default and note it. *(`m` was free.)*
- [x] 4.4 Test the toggle, the snooze auto-unmute on a needs-attention transition, and that
  working<->idle does not unmute. *(`TestMuteToggleMovesAgentToMutedBand`, `TestMutePersistsSnoozeAndReclaims`.)*

## 5. Workspaces badge interaction

- [x] 5.1 Exclude muted agents from a section's most-urgent badge computation (`sectionBadge`),
  so a section whose only live agents are muted renders no badge.
- [x] 5.2 Test that a muted-only section shows no status badge. *(`TestSectionBadgeExcludesMuted`.)*

## 6. Validation

- [x] 6.1 `go build ./...` and the full `go test ./...` pass.
- [x] 6.2 `openspec validate agent-mute-and-retriage` passes.
- [x] 6.3 Manual smoke in `be dash` (via the use-tty skill, driving a detached tmux session with
  real agents): verified bands render `NEEDS YOU / IDLE / WORKING / MUTED` with no `DONE` band;
  `m` moves an agent into/out of the MUTED band and persists to `mutes.json`; a muted agent keeps
  its real status glyph and drops out of its Workspaces section badge; and muting a needs-attention
  agent is refused (snooze auto-prunes it). *This caught a real bug: the `Muter` seam was not
  delegated through `Workspace`/`agentsAsRows`, so mute was a no-op in the real dash - now fixed
  and pinned by `TestMuterDelegation`.*

## 7. Incidental fix (folded in)

- [x] 7.1 Fix the focused selection arrow rendering on the terminal-default (black) background
  instead of the row highlight: `cursorCol`'s `hlCursor` case now sets `Background(rowHL)` so the
  gutter glyph and the highlighted row read as one continuous bar (mirroring what `barGutter` does
  over `SessionBg`). Pre-existing bug, surfaced during this change's smoke test; the mirror gutter
  is deliberately left background-less since a mirror row has no `RowHL` fill.

## 8. Fold-gated section summary (folded in)

- [x] 8.1 Show a Workspaces section bar's right-side summary (urgency badge + worktree count) only
  when the section is **folded**; an expanded bar drops it, since the rows list the worktrees and
  convey statuses themselves. `sessionBar` now renders `sessionBarRight` when folded and only the
  incidental reason (`it.note`) when expanded. *(The worktree glyph stays always-on, on the left.)*
- [x] 8.2 Keep an incidental section's reason ("not a git repo") visible in both fold states - the
  rows do not convey it (spec requires it stated inline).
- [x] 8.3 Spec deltas: `dash-lenses` "Status as a non-positional badge" and `agent-view`
  "Managed-repo presentation" both reworded to fold-gate the badge + worktree count.
- [x] 8.4 Test that the expanded bar carries no badge/wt and the folded bar carries both
  (`TestSectionSummaryOnlyWhenFolded`); verified live via the use-tty smoke test.
