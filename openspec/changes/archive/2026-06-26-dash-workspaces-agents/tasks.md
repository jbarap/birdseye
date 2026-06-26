## 1. Row data model

- [x] 1.1 Add a `LastChanged` timestamp to the agent row/source so the Agents lens can order by most-recent status change; populate it where status is written (hooks + title reconcile), and leave it zero when unknown. *(Reused the existing `Row.Updated` field - already populated on both the plain and orchestration paths via `agentRow`; no new field needed.)*
- [x] 1.2 Surface an incidental-reason fact for rows whose directory does not resolve to a recognized git repository (e.g. "not a git repo"). *(Implemented as the derived helper `incidentalReason(Row)` rather than a stored field: recognition is git-native, so the reason is a constant for `GitDir==""`; a stored field would only duplicate `GitDir`.)*
- [x] 1.3 Unit-test that the recency timestamp orders the band and that the incidental reason is set only for non-repo rows. *(`TestAgentsByBand`/`TestAgentsByBandNameFallback`, `TestIncidentalReason`.)*

## 2. Workspaces lens: stable ordering

- [x] 2.1 Replace `groupRank`/status-driven section ordering in `groupRows` with a stable sort by repository name (incidental `(no-repo)` section ordered last). *(Removed `groupRank` entirely.)*
- [x] 2.2 Change within-section row ordering to stable: `⌂ base` anchor first, worktree rows by worktree name, `◌ slot` rows last; keep a worktree's multiple-agent rows together at that name position. Removed status from the `rowLess`/`rowRank` sort key.
- [x] 2.3 Updated the ordering tests to assert a section holds position across a status change (`TestRefreshKeepsStableOrderAndSelection`) and that within-section rows are name-ordered, anchor first, slots last, independent of status (`TestGroupWithinSectionAnchorFirstSlotsLast`, `TestGroupAgentsWithinSectionNameOrderIndependentOfStatus`); updated the no-repo-bucket and flat-order tests.

## 3. Workspaces lens: status badge on the section bar

- [x] 3.1 Reused the existing themed `statusGlyph` set (no new tokens needed; guard test passes).
- [x] 3.2 `sectionBadge` computes the most-urgent status + count; `sessionBarRight` renders it (glyph + count, never color alone) on the bar, with no badge for a no-agent section.
- [x] 3.3 `TestSectionBadgeAndIncidentalNote` asserts the most-urgent glyph+count with agents and no badge without; stable ordering (badge doesn't move sections) is covered by `TestRefreshKeepsStableOrderAndSelection`.

## 4. Agents lens: triage bands

- [x] 4.1 Build the Agents-lens projection (`agentsByBand`): assign each agent to a fixed band (`bandOf`), order within a band by `Updated` newest-first, falling back to worktree+title when `Updated` is zero. `bandLabels` defines the fixed header order. *(Pure logic; rendering is task 4.2 in Stage B.)*
- [x] 4.2 `renderAgents` always draws all four band headers in fixed order (`bandHeader`, faint when empty), agents beneath. `buildAgentsLens` stores per-band counts on the header items.
- [x] 4.3 `agentsByBand` adds no retention window, so `DONE` membership stays governed solely by the existing liveness GC (documented in the function); the existing "Ended agent leaves when its process exits" GC test still covers reclamation.
- [x] 4.4 Band assignment, recency-within-band, and name-fallback are tested (`TestAgentsByBand`, `TestAgentsByBandNameFallback`); `TestAgentsLensStructure` pins that all four headers are always present. Move-between-bands follows directly from `bandOf` being status-only (a re-band never touches `agentsByBand`'s within-band order).

## 5. Layout: two always-visible lenses + preview

- [x] 5.1 `renderLenses` lays Workspaces and Agents side by side; the Agents lens claims a content-sized column (`agentsReserve`) carved off before the Workspaces+preview split (`mainWidth`).
- [x] 5.2 Below `dualLensMinWidth` (140), `renderLenses` shows only the focused lens; `h`/`l` still toggle which one, and the preview is shared.
- [x] 5.3 `TestDualLensLayoutAndNarrowFallback` checks both-lenses at 200 cols and single-lens at 80; `TestViewWideShowsPreviewNarrowHides` still covers preview show/hide.

## 6. Focus and linked selection

- [x] 6.1 Each lens has its own cursor/scroll (`cursor`/`top` and `agentCursor`/`agentTop`); `focus` tracks the active lens; `activeCursor` routes shared nav.
- [x] 6.2 Added `focus_left`/`focus_right` actions bound to `h`/`l` (and `left`/`right`) through the existing keymap/resolver; `j`/`k` move the focused lens, `tab` folds Workspaces only.
- [x] 6.3 `currentRow` and `refreshPreview` route to the focused lens, so the preview follows focus.
- [x] 6.4 `renderRows`/`renderAgents` mirror-highlight the focused selection's counterpart with a faint accent cursor glyph (`hlMirror`); non-agent selections (headers, anchor, slot) yield no counterpart, so no mirror.
- [x] 6.5 `setFocus`/`selectAgentInFocusedLens` land on the counterpart when present (else keep the cursor); a counterpart inside a folded section lands on its header (`sessionHasAgent`).
- [x] 6.6 `TestLensFocusSwitchCarriesCounterpart` covers switching, routed navigation, and linked counterpart landing.

## 7. Incidental (no-repo) section

- [x] 7.1 The incidental bucket renders through the same section→row structure; `groupRows` sets its bar `note` from `incidentalReason`, shown via `sessionBarRight` (e.g. "not a git repo").
- [x] 7.2 `TestSectionBadgeAndIncidentalNote` asserts the incidental section's note; `TestGroupAgentsUngroupedBucket` covers its standard structure and last-place ordering.

## 8. CLI wiring and chrome

- [x] 8.1 No wiring change needed: `agents.Run`'s signature is unchanged and the new actions flow through `DefaultKeymap`/config automatically; the binary builds and the dash is driven as before.
- [x] 8.2 Help legend adds an `h/l lens` hint and relabels fold as `fold (workspaces)` (`helpOrder`/`actionLabel`/`renderHelp`).

## 9. Design language and docs

- [x] 9.1 `DESIGN.md` documents the two lenses, the bands, and the section-bar badge. Mirror-highlight styling resolved *without* a new token: a faint accent `CursorGlyph` (the parked open question - chose to reuse `Accent` + faint over adding a token).
- [x] 9.2 `README.md` dash description rewritten for the two lenses (no screenshots reference the old single list).

## 10. Verification

- [x] 10.1 Theme guard test (`TestNoHardcodedDesignTokensOutsideTheme`) passes - the new rendering uses only `theme` tokens (`Accent`/`Gray`/`RowHL`/`CursorGlyph`) plus non-color faint/bold.
- [x] 10.2 `gofmt` clean; `go vet ./...` clean; `go test ./...` green across all packages; `be` binary builds.
- [x] 10.3 Live check (user-driven): ran `be dash` and reviewed live; the iterations that came out of it (Agents lens moved to the left with default focus, band folding in both lenses, edge-to-edge shared section bars with aligned empty-band labels) are folded into the design/specs above.
