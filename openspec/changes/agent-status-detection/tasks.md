## 1. Investigation (resolve the one open question)

- [ ] 1.1 Observe a Claude agent's `#{pane_title}` while it is **blocked on a permission prompt**
  (`tmux display-message -p -t <pane> '#{pane_title}'`). Record whether it is a Braille frame, a
  `✳`, or a distinct string. If distinct, note it - it could let needs-attention be read from the
  title and simplify the latch. This does not block implementation (the latch works either way).
  _Not yet observed: no agent was sitting on a permission prompt during apply. Opportunistic;
  the latch handles both outcomes._

## 2. tmux pane-title read

- [x] 2.1 Add `PaneTitles()` to `internal/tmux/tmux.go`: one `tmux list-panes -a -F
  '#{pane_id}\t#{pane_title}'`, returning `map[paneID]title`. A failure yields an empty map, not
  an error that aborts a refresh.
- [x] 2.2 Unit-test `PaneTitles` parsing (tab split, empty titles, missing/garbage lines) against a
  mocked runner, in `internal/tmux/tmux_test.go`.

## 3. Claude title→level matcher

- [x] 3.1 Add a documented, versioned glyph-class matcher in `internal/agents/claude.go` beside
  `idleNotification`: first glyph in U+2800–U+28FF → `StatusWorking`; first glyph U+2733 (`✳`) →
  `StatusIdle`; otherwise `("", false)` (no signal). Match the Braille **class**, not a frame.
- [x] 3.2 Table-test the matcher: several Braille frames (e.g. `⠐`, `⠂`) → working, `✳` → idle,
  empty / plain text / other glyph → no signal.

## 4. Level-detector seam

- [x] 4.1 Define a `levelDetector` interface (`Level(paneID string, titles map[string]string)
  (Status, bool)`) so reconciliation consumes a level without knowing its source.
- [x] 4.2 Implement Claude's title detector against that interface using the matcher from 3.1.

## 5. Reconciliation

- [x] 5.1 Implement a pure `reconcile(hookStatus Status, level Status, haveLevel bool) Status`
  encoding the precedence table: needs-attention latch clears only on a working level (idle/none
  held); working/idle taken from the level when present; fall back to `hookStatus` when no level;
  `done` always wins.
- [x] 5.2 Unit-test every cell of the precedence table, including stale-working→idle, attention
  held while idle, attention cleared on working, and no-signal fallback.

## 6. Wire into ClaudeSource

- [x] 6.1 Add an injectable `titles func() (map[string]string, error)` field on `ClaudeSource`
  (default = the tmux `PaneTitles` call), mirroring the existing overridable `alive`.
- [x] 6.2 In `Agents()`, fetch titles once per call, then for each live record apply the detector +
  `reconcile` to set the agent's status before dedup/sort. Records without a `TmuxPane` skip the
  level read and keep the hook status.
- [x] 6.3 Test `Agents()` with injected titles: a recorded-working record whose title is `✳` renders
  idle; a record with no title keeps its hook status; the title fetcher is invoked once per refresh.

## 7. Docs / philosophy

- [x] 7.1 Update the `internal/agents/agent.go` package header to drop the "rather than scraping
  panes" absolute and describe the layered model (hooks for identity/existence/attention/done; live
  level signal for working/idle).
- [x] 7.2 Update `DESIGN.md` and `CLAUDE.md` notes that assert hook-only status. _DESIGN.md primitive
  list updated to "hooks plus the live OSC-title level signal"; CLAUDE.md carried no status-source
  assertion to revise._
- [x] 7.3 Update the README status table to reflect that working/idle is title-derived.

## 8. Verify

- [x] 8.1 `go build ./...` and `go test ./internal/...` green.
- [ ] 8.2 Manual check against a live agent: interrupt a working agent with Esc and confirm the dash
  flips it to idle without a `Stop` hook; confirm a permission prompt still shows needs-attention.
  _Requires a live throwaway agent; left for manual verification rather than touching real sessions._
