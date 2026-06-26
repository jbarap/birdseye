package agents

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jbarap/birdseye/internal/providers/dir"
	"github.com/jbarap/birdseye/internal/theme"
)

// stripANSI drops SGR escapes so a test can assert on the plain text layout (column
// positions, alignment) rather than the styled bytes.
func stripANSI(s string) string { return ansi.Strip(s) }

type fakeSource struct {
	list []Agent
	err  error
}

func (f *fakeSource) Agents() ([]Agent, error) { return f.list, f.err }

type fakePreviewer struct {
	out   map[string]string
	fail  map[string]bool
	calls int
}

func (f *fakePreviewer) Preview(a Agent) (string, error) {
	f.calls++
	if f.fail[a.SessionID] {
		return "", errors.New("capture failed")
	}
	return f.out[a.SessionID], nil
}

// agentsN builds n agents s0..s(n-1), all working.
func agentsN(n int) []Agent {
	out := make([]Agent, n)
	for i := range out {
		id := string(rune('a' + i))
		out[i] = Agent{SessionID: id, Title: id, Status: StatusWorking, TmuxSession: id}
	}
	return out
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(m model, msg tea.Msg) model {
	nm, _ := m.Update(msg)
	return nm.(model)
}

// mustModel builds a test model focused on the Workspaces lens. Most interaction tests
// drive that lens (its tree, sections, slots, base, fold, and cross-group motions), so it
// is the convenient default here; production opens on the Agents lens instead (pinned by
// TestDefaultFocusIsAgents). Lens-focus tests set m.focus explicitly.
func mustModel(t *testing.T, list []Agent, prev Previewer, km Keymap) model {
	t.Helper()
	m, err := nm(&fakeSource{list: list}, prev, km)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = lensWorkspaces
	return m
}

// nm builds a model from a plain agent Source (wrapped as rows) with the default
// refresh, mirroring the old newModel(Source, ...) test signature.
func nm(src Source, prev Previewer, km Keymap) (model, error) {
	return newModel(AgentsAsRows(src), prev, km, 0)
}

// rowsOf converts agents to RowAgent rows for the grouping tests. Grouping is by
// repository, so each agent's tmux session stands in for the repository it groups under:
// the row carries Repo == GitDir == the session name. An agent with no session has no
// repository and falls into the ungrouped bucket.
func rowsOf(ags ...Agent) []Row {
	out := make([]Row, len(ags))
	for i, a := range ags {
		r := agentRow(a)
		if a.TmuxSession != "" {
			r.Repo, r.GitDir = a.TmuxSession, a.TmuxSession
		}
		out[i] = r
	}
	return out
}

// rowModel builds a model over fixed, repo-tagged rows (rowsOf), for the section/fold
// mechanics tests that need repository-keyed grouping rather than the plain agent source.
// Like mustModel it focuses the Workspaces lens (see mustModel); production opens on Agents.
func rowModel(km Keymap, ags ...Agent) (model, error) {
	m, err := newModel(fixedRows{rowsOf(ags...)}, nil, km, 0)
	if err == nil {
		m.focus = lensWorkspaces
	}
	return m, err
}

// muterRows is a RowSource that also persists mute intent in memory and stamps it back onto
// the rows it yields — a tiny stand-in for the ClaudeSource mute store, so the `m` toggle can
// be driven through the model end to end.
type muterRows struct {
	rows  []Row
	muted map[string]bool
}

func (s *muterRows) Rows() ([]Row, error) {
	out := make([]Row, len(s.rows))
	copy(out, s.rows)
	for i := range out {
		out[i].Muted = s.muted[out[i].muteKey()]
	}
	return out, nil
}

func (s *muterRows) SetMuted(key string, muted bool) error {
	if s.muted == nil {
		s.muted = map[string]bool{}
	}
	if muted {
		s.muted[key] = true
	} else {
		delete(s.muted, key)
	}
	return nil
}

// TestMuteToggleMovesAgentToMutedBand drives the `m` key: it persists a mute for the selected
// agent's location through the Muter seam, the reload restamps it, and the agent lands in the
// MUTED band; pressing `m` again clears it.
func TestMuteToggleMovesAgentToMutedBand(t *testing.T) {
	src := &muterRows{rows: []Row{
		{Kind: RowAgent, SessionID: "a", TmuxSession: "a", TmuxPane: "%1", Title: "a", Status: StatusWorking},
	}}
	m, err := newModel(src, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = lensAgents
	m.width, m.height = 200, 30

	m = send(m, key("m"))
	if !src.muted["pane:%1"] {
		t.Fatalf("pressing m should persist a mute for the agent's location, store=%v", src.muted)
	}
	if got := agentsByBand(m.agentRows)[bandMuted]; len(got) != 1 || got[0].SessionID != "a" {
		t.Fatalf("muted agent should be in the MUTED band, got %+v", got)
	}

	m = send(m, key("m"))
	if src.muted["pane:%1"] {
		t.Fatalf("pressing m again should clear the mute, store=%v", src.muted)
	}
	if got := agentsByBand(m.agentRows)[bandMuted]; len(got) != 0 {
		t.Fatalf("MUTED band should be empty after unmuting, got %+v", got)
	}
}

func TestNavigationVimMotions(t *testing.T) {
	m := mustModel(t, agentsN(5), nil, DefaultKeymap())

	m = send(m, key("j"))
	m = send(m, key("down"))
	if m.cursor != 2 {
		t.Fatalf("j/down should reach 2, got %d", m.cursor)
	}
	m = send(m, key("G"))
	if m.cursor != 4 {
		t.Fatalf("G should jump to bottom 4, got %d", m.cursor)
	}
	m = send(m, key("k"))
	if m.cursor != 3 {
		t.Fatalf("k should go up to 3, got %d", m.cursor)
	}
	// gg chord: g arms, second g fires top.
	m = send(m, key("g"))
	if m.cursor != 3 {
		t.Fatalf("lone g should arm, not move; got %d", m.cursor)
	}
	m = send(m, key("g"))
	if m.cursor != 0 {
		t.Fatalf("gg should jump to top, got %d", m.cursor)
	}
	// half-page down/up clamp to bounds.
	m = send(m, key("ctrl+d"))
	if m.cursor != 4 {
		t.Fatalf("ctrl+d should clamp to bottom 4, got %d", m.cursor)
	}
	m = send(m, key("ctrl+u"))
	if m.cursor != 0 {
		t.Fatalf("ctrl+u should clamp to top 0, got %d", m.cursor)
	}
}

func TestChordAbortedByOtherKey(t *testing.T) {
	m := mustModel(t, agentsN(5), nil, DefaultKeymap())
	m = send(m, key("G")) // cursor 4
	m = send(m, key("g")) // arm
	m = send(m, key("j")) // not the completing key: clears chord, j moves down (clamped)
	if m.cursor != 4 {
		t.Fatalf("aborted chord then j at bottom should stay 4, got %d", m.cursor)
	}
	if m.pending != "" {
		t.Fatalf("pending chord should be cleared")
	}
}

func TestSelectAndQuit(t *testing.T) {
	m := mustModel(t, agentsN(3), nil, DefaultKeymap())
	m = send(m, key("j"))
	nm, cmd := m.Update(key("enter"))
	m = nm.(model)
	if m.chosen == nil || m.chosen.SessionID != "b" {
		t.Fatalf("enter should choose current agent, got %+v", m.chosen)
	}
	if cmd == nil {
		t.Fatal("enter should return a quit command")
	}

	m2 := mustModel(t, agentsN(3), nil, DefaultKeymap())
	if _, cmd := m2.Update(key("q")); cmd == nil {
		t.Fatal("q should return a quit command")
	}
}

func TestRebindReplacesDefaultKey(t *testing.T) {
	km, err := ResolveKeymap(map[string][]string{"down": {"p"}})
	if err != nil {
		t.Fatal(err)
	}
	m := mustModel(t, agentsN(3), nil, km)
	m = send(m, key("p"))
	if m.cursor != 1 {
		t.Fatalf("custom key p should move down, got %d", m.cursor)
	}
	m = send(m, key("j"))
	if m.cursor != 1 {
		t.Fatalf("default j should no longer move after rebind, got %d", m.cursor)
	}
}

func TestRefreshKeepsStableOrderAndSelection(t *testing.T) {
	src := &fakeSource{list: agentsN(3)} // sessions a,b,c all working → name order a,b,c
	m, err := nm(src, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	if m.rows[1].SessionID != "b" {
		t.Fatalf("precondition: expected b at index 1, got %s", m.rows[1].SessionID)
	}
	m.cursor = 1 // on "b"

	// b's agent now needs attention. Ordering is by name, not status, so b must NOT
	// move - the section a user is watching holds position - and selection stays on it.
	updated := agentsN(3)
	updated[1].Status = StatusNeedsAttention
	src.list = updated
	m = send(m, tickMsg(time.Now()))
	if m.rows[0].SessionID != "a" || m.rows[1].SessionID != "b" || m.rows[2].SessionID != "c" {
		t.Fatalf("order must stay a,b,c regardless of status, got %s,%s,%s",
			m.rows[0].SessionID, m.rows[1].SessionID, m.rows[2].SessionID)
	}
	if m.cursor != 1 || m.rows[m.cursor].SessionID != "b" {
		t.Fatalf("selection should stay on b at index 1, got %d (%s)", m.cursor, m.rows[m.cursor].SessionID)
	}
}

func TestRefreshClampsWhenSelectedVanishes(t *testing.T) {
	src := &fakeSource{list: agentsN(3)}
	m, _ := nm(src, nil, DefaultKeymap())
	m.cursor = 2 // on "c"

	src.list = agentsN(1) // only "a" remains
	m = send(m, tickMsg(time.Now()))
	if m.cursor != 0 {
		t.Fatalf("vanished selection should clamp to 0, got %d", m.cursor)
	}
	if m.cursor >= len(m.rows) {
		t.Fatalf("cursor out of bounds: %d of %d", m.cursor, len(m.rows))
	}
}

func TestPreviewFollowsCursorAndRefresh(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{"a": "doing A", "b": "doing B"}}
	m := mustModel(t, agentsN(2), prev, DefaultKeymap())
	if m.preview != "doing A" {
		t.Fatalf("initial preview should be A's, got %q", m.preview)
	}
	m = send(m, key("j"))
	if m.preview != "doing B" {
		t.Fatalf("preview should follow cursor to B, got %q", m.preview)
	}
	// New output for B shows up on refresh.
	prev.out["b"] = "B updated"
	m = send(m, tickMsg(time.Now()))
	if m.preview != "B updated" {
		t.Fatalf("preview should refresh with tick, got %q", m.preview)
	}
}

// TestWindowlessStructuralRowShowsNoPreview pins the rule that a base or slot with no
// live window of its own previews nothing — capturing its session would surface a sibling
// worktree's window and read as if the empty slot were running something. The previewer is
// primed with output for the slot's id to prove the gate, not the capture, suppresses it.
func TestWindowlessStructuralRowShowsNoPreview(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{
		"slot:proj:spike": "a sibling window's output",
		"anchor:proj":     "the active window's output",
	}}
	rows := []Row{
		{Kind: RowSlot, SessionID: "slot:proj:spike", TmuxSession: "proj", Worktree: "spike", Dir: "/d"},
		{Kind: RowAnchor, SessionID: "anchor:proj", TmuxSession: "proj", Worktree: "main", Dir: "/d", IsPrimary: true},
	}
	m, err := newModel(fixedRows{rows}, prev, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.preview != "" {
		t.Fatalf("a windowless slot must show no preview, got %q", m.preview)
	}
	m = send(m, key("j")) // onto the windowless base
	if m.preview != "" {
		t.Fatalf("a windowless base must show no preview, got %q", m.preview)
	}
}

// TestPreviewPlaceholderByRowKind pins the empty-preview copy to the row: a windowless
// slot or base reads as a dormant, actionable state (not a failed capture), while a real
// window that produced nothing keeps the faint generic placeholder.
func TestPreviewPlaceholderByRowKind(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		want string // substring the placeholder must contain
		deny string // substring it must not contain
	}{
		{"slot", Row{Kind: RowSlot, TmuxSession: "proj", Repo: "proj", Worktree: "spike", Dir: "/d"}, "spawns an agent", "no preview available"},
		{"base", Row{Kind: RowAnchor, TmuxSession: "proj", Repo: "proj", Worktree: "main", Dir: "/d", IsPrimary: true}, "opens a shell", "no preview available"},
		{"windowed agent, empty capture", Row{Kind: RowAgent, SessionID: "a", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%1"}, "no preview available", "spawns an agent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := newModel(fixedRows{[]Row{tc.row}}, &fakePreviewer{}, DefaultKeymap(), 0)
			if err != nil {
				t.Fatal(err)
			}
			m.focus = lensWorkspaces // slot/base are Workspaces-lens rows
			got := m.previewPlaceholder()
			if !strings.Contains(got, tc.want) {
				t.Fatalf("placeholder %q should contain %q", got, tc.want)
			}
			if strings.Contains(got, tc.deny) {
				t.Fatalf("placeholder %q should not contain %q", got, tc.deny)
			}
		})
	}
}

func TestPreviewPlaceholderOnErrorAndStatusUnaffected(t *testing.T) {
	prev := &fakePreviewer{
		out:  map[string]string{"a": "ok"},
		fail: map[string]bool{"a": true},
	}
	m := mustModel(t, agentsN(1), prev, DefaultKeymap())
	if m.preview != "" {
		t.Fatalf("failed capture should yield empty preview, got %q", m.preview)
	}
	// Status is hook-derived, never inferred from preview output.
	if m.rows[0].Status != StatusWorking {
		t.Fatalf("status must not be derived from preview, got %q", m.rows[0].Status)
	}
}

func TestViewWideShowsPreviewNarrowHides(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{"a": "hello"}}
	m := mustModel(t, agentsN(2), prev, DefaultKeymap())

	m.width, m.height = 120, 30
	if !strings.Contains(m.View(), "preview") {
		t.Fatalf("wide view should include the preview pane")
	}

	m.width, m.height = 40, 30
	if strings.Contains(m.View(), "preview") {
		t.Fatalf("narrow view should hide the preview pane")
	}
}

func TestPreviewLinesStripsTrailingBlanks(t *testing.T) {
	// A shell prompt at the top with a blank rest of screen (capture-pane pads
	// the visible screen) must not render as an all-blank tail.
	pane := "~/projects ❯\n" + strings.Repeat("\n", 40)
	got := previewLines(pane, 6, 50)
	if len(got) != 1 || !strings.Contains(got[0], "❯") {
		t.Fatalf("expected the prompt line, got %q", got)
	}

	// A full screen keeps its most recent (bottom) h lines.
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	got = previewLines(sb.String(), 3, 50)
	if len(got) != 3 || got[0] != "line 17" || got[2] != "line 19" {
		t.Fatalf("expected last 3 lines, got %q", got)
	}

	// Truly blank capture yields nothing (caller shows a placeholder).
	if got := previewLines("\n\n\n", 6, 50); len(got) != 0 {
		t.Fatalf("blank pane should yield no lines, got %q", got)
	}
}

func TestViewFitsTerminalHeightWithTallPreview(t *testing.T) {
	// A preview far taller than the popup must not push the total view past
	// m.height; otherwise the alt-screen scrolls and the agent list (top) is
	// pushed out of view. Lines are long on purpose: if they wrap inside the
	// preview frame they inflate its height past the terminal.
	// ASCII lines: cell width equals rune count, so even a naive truncation keeps
	// them in bounds.
	long := strings.Repeat("the quick brown fox jumps over the lazy dog ", 6)
	// Wide-glyph lines: each rune renders as two cells, so a rune-count truncation
	// undercounts the width by half, the line overflows the frame, wraps, and
	// inflates the preview past the terminal — the bug this guards against.
	wide := strings.Repeat("広い画面の日本語テキスト🚀🔥✨ ", 12)
	for name, body := range map[string]string{"ascii": long, "wide": wide} {
		var sb strings.Builder
		for i := 0; i < 200; i++ {
			sb.WriteString(body)
			sb.WriteByte('\n')
		}
		prev := &fakePreviewer{out: map[string]string{"a": sb.String(), "b": sb.String()}}

		for _, dim := range []struct{ w, h int }{{120, 24}, {160, 30}, {100, 40}} {
			m := mustModel(t, agentsN(2), prev, DefaultKeymap())
			m.width, m.height = dim.w, dim.h
			if h := lipgloss.Height(m.View()); h > m.height {
				t.Fatalf("%s at %dx%d: view height %d exceeds terminal height %d (would scroll list out of view)",
					name, dim.w, dim.h, h, m.height)
			}
		}
	}
}

func TestViewFitsHeightWithNoticeLine(t *testing.T) {
	// The original bug: pressing n on a non-managed row adds a notice line, and the
	// body did not shrink to make room — so the total view exceeded m.height and the
	// alt-screen scrolled the top rows (and the preview top) out of view. The notice
	// must steal from the body budget, never overflow.
	prev := &fakePreviewer{out: map[string]string{}}
	for _, dim := range []struct{ w, h int }{{120, 24}, {160, 30}, {80, 20}} {
		m := mustModel(t, agentsN(40), prev, DefaultKeymap())
		m.orch = &fakeOrch{}
		m.width, m.height = dim.w, dim.h
		m.syncViewport()
		if h := lipgloss.Height(m.View()); h > m.height {
			t.Fatalf("normal at %dx%d: height %d exceeds terminal height %d", dim.w, dim.h, h, m.height)
		}
		// n on a non-managed row surfaces the "place the cursor inside a managed repo"
		// notice; the view must still fit.
		nm, _ := m.Update(key("n"))
		mn := nm.(model)
		if mn.notice == "" {
			t.Fatalf("expected a notice after n on a non-managed row")
		}
		if h := lipgloss.Height(mn.View()); h > mn.height {
			t.Fatalf("with notice at %dx%d: height %d exceeds terminal height %d", dim.w, dim.h, h, mn.height)
		}
	}
}

func TestListWindowsToCursorAndFits(t *testing.T) {
	// A list far taller than the terminal must window to the cursor: the view fits the
	// height, the selected (bottom) row is on-screen, and the off-window top has
	// scrolled away.
	list := make([]Agent, 40)
	for i := range list {
		id := fmt.Sprintf("sess%02d", i)
		list[i] = Agent{SessionID: id, Title: fmt.Sprintf("row%02d", i), Status: StatusWorking, TmuxSession: id}
	}
	m := mustModel(t, list, nil, DefaultKeymap())
	m.width, m.height = 80, 16

	nm, _ := m.Update(key("G"))
	m = nm.(model)
	v := m.View()
	if h := lipgloss.Height(v); h > m.height {
		t.Fatalf("windowed view height %d exceeds terminal height %d:\n%s", h, m.height, v)
	}
	if !strings.Contains(v, "row39") {
		t.Fatalf("the bottom row must be visible after G:\n%s", v)
	}
	if strings.Contains(v, "row00") {
		t.Fatalf("the top row should have scrolled out of the window:\n%s", v)
	}

	// Back to the top brings the first row into view and pushes the bottom out.
	nm, _ = m.Update(key("g"))
	m = nm.(model)
	nm, _ = m.Update(key("g"))
	m = nm.(model)
	v = m.View()
	if !strings.Contains(v, "row00") {
		t.Fatalf("the top row must be visible after gg:\n%s", v)
	}
	if strings.Contains(v, "row39") {
		t.Fatalf("the bottom row should be out of the window after gg:\n%s", v)
	}
}

func TestNoticeIsNotable(t *testing.T) {
	// Feedback must be impossible to miss: an error/rejection carries the ✗ glyph, a
	// neutral confirmation the • glyph (so it reads without color, not the faint help
	// line that made the rejection look like nothing happened).
	m := mustModel(t, agentsN(2), nil, DefaultKeymap())
	m.orch = &fakeOrch{}
	m.width, m.height = 100, 20

	// n on a non-managed row → error notice with ✗.
	nm, _ := m.Update(key("n"))
	mn := nm.(model)
	if mn.noticeLevel != noticeError || !strings.Contains(mn.View(), errGlyph) {
		t.Fatalf("an error notice should render the %q glyph:\n%s", errGlyph, mn.View())
	}

	// A neutral info notice (e.g. delete cancelled) uses • instead.
	mn.setInfo("delete cancelled")
	v := mn.View()
	if !strings.Contains(v, infoGlyph) || strings.Contains(v, errGlyph) {
		t.Fatalf("an info notice should render %q (not %q):\n%s", infoGlyph, errGlyph, v)
	}
}

func TestNoticeAutoDismissAndClearOnMove(t *testing.T) {
	m := mustModel(t, agentsN(3), nil, DefaultKeymap())
	m.orch = &fakeOrch{}
	m.width, m.height = 100, 20

	// n on a non-managed row sets an error notice and arms an auto-dismiss timer.
	nm, cmd := m.Update(key("n"))
	m = nm.(model)
	if m.notice == "" {
		t.Fatal("expected an error notice after n on a non-managed row")
	}
	if cmd == nil {
		t.Fatal("a fresh notice should arm an auto-dismiss command")
	}
	gen := m.noticeGen

	// A stale expiry (an older generation) must not clear the current notice.
	nm, _ = m.Update(noticeExpireMsg{gen - 1})
	m = nm.(model)
	if m.notice == "" {
		t.Fatal("a stale-generation expiry must not clear a newer notice")
	}

	// The matching expiry dismisses it.
	nm, _ = m.Update(noticeExpireMsg{gen})
	m = nm.(model)
	if m.notice != "" {
		t.Fatalf("the matching expiry should dismiss the notice, got %q", m.notice)
	}

	// Re-arm a notice, then move the cursor: it clears at once and the move still happens.
	nm, _ = m.Update(key("n"))
	m = nm.(model)
	if m.notice == "" {
		t.Fatal("precondition: notice re-armed")
	}
	nm, _ = m.Update(key("j"))
	m = nm.(model)
	if m.notice != "" {
		t.Fatalf("moving the cursor should clear the notice, got %q", m.notice)
	}
	if m.cursor != 1 {
		t.Fatalf("the move should still happen (cursor 1), got %d", m.cursor)
	}
}

func TestViewEmptyState(t *testing.T) {
	m := mustModel(t, nil, nil, DefaultKeymap())
	if !strings.Contains(m.View(), "No agents") {
		t.Fatalf("empty view should show an informative empty state")
	}
}

func TestPanelTitle(t *testing.T) {
	accent := lipgloss.Color("#c792ea")
	box := frameStyle.Render("a wide row of content\na second row of content")
	out := panelTitle(box, "agents", accent)

	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[0], "agents") {
		t.Fatalf("title should ride on the top border line, got %q", lines[0])
	}
	if !strings.Contains(out, "a wide row of content") || !strings.Contains(out, "a second row of content") {
		t.Fatalf("panel should preserve the body, got %q", out)
	}
	if w, bw := lipgloss.Width(out), lipgloss.Width(box); w != bw {
		t.Fatalf("titled panel width %d should match the frame width %d", w, bw)
	}

	// A panel too narrow for any title keeps its plain top border unchanged.
	tiny := frameStyle.Render("")
	if got := panelTitle(tiny, "agents", accent); got != tiny {
		t.Fatalf("too-narrow panel should keep its plain border unchanged, got %q", got)
	}
}

// ag builds an agent with a unique session id so dedup is irrelevant to grouping tests.
func ag(session, window, title string, st Status) Agent {
	return Agent{
		SessionID:   session + "/" + window + "/" + title,
		TmuxSession: session,
		TmuxWindow:  window,
		Title:       title,
		Status:      st,
	}
}

// shape renders the grouped items into compact tags for assertion: S:<session> for a
// session header, A:<title>@<window> for an agent row (or A:<title> when the agent has
// no window label).
func shape(ordered []Row, items []renderItem) []string {
	var got []string
	for _, it := range items {
		switch it.kind {
		case kindSession:
			got = append(got, "S:"+it.label)
		case kindRow:
			tag := "A:" + ordered[it.rowIdx].Title
			if it.window != "" {
				tag += "@" + it.window
			}
			got = append(got, tag)
		}
	}
	return got
}

func TestGroupAgentsFlatSessionOrderAndWindowLabels(t *testing.T) {
	in := []Agent{
		ag("api-server", "1", "write tests", StatusWorking),
		ag("api-server", "1", "refactor auth", StatusNeedsAttention),
		ag("api-server", "2", "migrate db", StatusIdle),
		ag("web", "0", "ship landing", StatusIdle),
	}
	ordered, items := groupRows(rowsOf(in...))

	want := []string{
		"S:api-server",          // sections by name: api-server before web (not by status)
		"A:migrate db@win 2",    // agents ordered by title within the section, independent of
		"A:refactor auth@win 1", // status and of which window they occupy (no window headers,
		"A:write tests@win 1",   // no inline-collapse special case)
		"S:web",
		"A:ship landing@win 0",
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("grouped shape mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestGroupAgentsWithinSectionNameOrderIndependentOfStatus(t *testing.T) {
	// Within one section, agents order by name (title here), independent of status and of
	// which window they occupy. Titles are deliberately in the reverse of status-rank order
	// to prove status no longer participates in the sort.
	in := []Agent{
		ag("s", "1", "low", StatusIdle),
		ag("s", "1", "high", StatusNeedsAttention),
		ag("s", "2", "mid", StatusWorking),
	}
	ordered, items := groupRows(rowsOf(in...))
	want := []string{
		"S:s",
		"A:high@win 1", // h < l < m by title, regardless of attn/idle/work
		"A:low@win 1",
		"A:mid@win 2",
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("within-section name-order shape mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestGroupAgentsUsesWindowName(t *testing.T) {
	withName := func(session, window, name, title string, st Status) Agent {
		a := ag(session, window, title, st)
		a.TmuxWindowName = name
		return a
	}
	in := []Agent{
		withName("api", "1", "editor", "a1", StatusWorking),
		withName("api", "1", "editor", "a2", StatusWorking),
		withName("api", "2", "server", "a3", StatusIdle),
	}
	ordered, items := groupRows(rowsOf(in...))
	want := []string{
		"S:api",
		"A:a1@editor", // window label is the tmux window name, not "win 1"
		"A:a2@editor",
		"A:a3@server",
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("window-name shape mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestGroupAgentsUngroupedBucket(t *testing.T) {
	in := []Agent{
		ag("api", "0", "real one", StatusIdle),
		{SessionID: "loose-id", Title: "loose", Status: StatusWorking}, // no tmux session
	}
	ordered, items := groupRows(rowsOf(in...))

	// The incidental no-repo bucket always sorts last, after named repositories,
	// regardless of its agents' status.
	want := []string{
		"S:api",
		"A:real one@win 0",
		"S:(no-repo)",
		"A:loose", // no tmux window → no inline label
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("no-repo bucket shape mismatch:\n got %v\nwant %v", got, want)
	}
}

// TestGroupWithinSectionAnchorFirstSlotsLast pins the managed-repo within-section order:
// the anchor first, agents (by worktree name) in the middle, empty slots last - all
// independent of status.
func TestGroupWithinSectionAnchorFirstSlotsLast(t *testing.T) {
	gd := "/repo/.git"
	kindName := map[RowKind]string{RowAgent: "agent", RowAnchor: "anchor", RowSlot: "slot"}
	mk := func(kind RowKind, wt string, st Status) Row {
		return Row{Kind: kind, SessionID: kindName[kind] + wt, TmuxSession: "proj", Repo: "proj", GitDir: gd, Worktree: wt, Status: st}
	}
	in := []Row{
		mk(RowSlot, "zeta", StatusUnknown),
		mk(RowAgent, "delta", StatusIdle),
		mk(RowAgent, "alpha", StatusNeedsAttention), // most urgent, but must NOT float
		mk(RowAnchor, "main", StatusUnknown),
	}
	ordered, _ := groupRows(in)
	var got []string
	for _, r := range ordered {
		got = append(got, kindName[r.Kind]+":"+r.Worktree)
	}
	want := []string{
		"anchor:main", // anchor pinned first
		"agent:alpha", // agents by worktree name (alpha < delta), not status
		"agent:delta",
		"slot:zeta", // empty slot last
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("within-section order mismatch:\n got %v\nwant %v", got, want)
	}
}

// TestAgentsByBand checks the Agents-lens projection: agents bucket by band, only RowAgent
// rows appear, a muted agent lands in MUTED regardless of its status, and within a band the
// most-recently-changed agent comes first.
func TestAgentsByBand(t *testing.T) {
	t0 := time.Now()
	mk := func(title string, st Status, ageSec int) Row {
		return Row{Kind: RowAgent, SessionID: title, Title: title, Status: st, Updated: t0.Add(-time.Duration(ageSec) * time.Second)}
	}
	muted := mk("parked", StatusWorking, 0)
	muted.Muted = true
	in := []Row{
		mk("w-old", StatusWorking, 30),
		mk("w-new", StatusWorking, 1),
		mk("blocked", StatusNeedsAttention, 10),
		mk("quiet", StatusIdle, 5),
		mk("stale", StatusUnknown, 2),       // unknown reads as quiet → IDLE band
		muted,                               // working, but muted → MUTED band
		{Kind: RowAnchor, Worktree: "main"}, // structural rows are excluded
		{Kind: RowSlot, Worktree: "spike"},
	}
	bands := agentsByBand(in)

	titles := func(b agentBand) []string {
		var out []string
		for _, r := range bands[b] {
			out = append(out, r.Title)
		}
		return out
	}
	if got := titles(bandNeedsYou); !reflect.DeepEqual(got, []string{"blocked"}) {
		t.Fatalf("NEEDS YOU = %v", got)
	}
	if got := titles(bandWorking); !reflect.DeepEqual(got, []string{"w-new", "w-old"}) {
		t.Fatalf("WORKING should be newest-first, got %v", got)
	}
	if got := titles(bandIdle); !reflect.DeepEqual(got, []string{"stale", "quiet"}) {
		// stale (2s) is newer than quiet (5s); unknown lands in IDLE.
		t.Fatalf("IDLE = %v", got)
	}
	if got := titles(bandMuted); !reflect.DeepEqual(got, []string{"parked"}) {
		t.Fatalf("MUTED should hold the muted agent regardless of status, got %v", got)
	}
}

// TestAgentsByBandNameFallback: when status-change times are absent (zero Updated), a
// band orders by worktree then title instead of recency.
func TestAgentsByBandNameFallback(t *testing.T) {
	mk := func(wt, title string) Row {
		return Row{Kind: RowAgent, Title: title, Status: StatusWorking, Worktree: wt}
	}
	in := []Row{mk("b", "z"), mk("a", "y"), mk("a", "x")}
	got := agentsByBand(in)[bandWorking]
	var labels []string
	for _, r := range got {
		labels = append(labels, r.Worktree+"/"+r.Title)
	}
	if !reflect.DeepEqual(labels, []string{"a/x", "a/y", "b/z"}) {
		t.Fatalf("name fallback order = %v", labels)
	}
}

func TestIncidentalReason(t *testing.T) {
	if got := incidentalReason(Row{GitDir: "/repo/.git"}); got != "" {
		t.Fatalf("a recognized repo row has no incidental reason, got %q", got)
	}
	if got := incidentalReason(Row{GitDir: ""}); got != "not a git repo" {
		t.Fatalf("a no-repo row explains itself, got %q", got)
	}
}

// TestSectionBadgeAndIncidentalNote: a section bar carries its most-urgent-status badge
// (glyph status + count), a no-agent section carries none, and the incidental bucket states
// its reason.
func TestSectionBadgeAndIncidentalNote(t *testing.T) {
	rows := []Row{
		{Kind: RowAgent, SessionID: "a", Repo: "r", GitDir: "/g", Worktree: "main", Title: "x", Status: StatusNeedsAttention},
		{Kind: RowAgent, SessionID: "b", Repo: "r", GitDir: "/g", Worktree: "feat", Title: "y", Status: StatusWorking},
		{Kind: RowAgent, SessionID: "c", Title: "loose", Status: StatusIdle}, // no-repo
	}
	_, items := groupRows(rows)
	var repoHdr, incHdr *renderItem
	for i := range items {
		if items[i].kind != kindSession {
			continue
		}
		if items[i].label == "(no-repo)" {
			incHdr = &items[i]
		} else {
			repoHdr = &items[i]
		}
	}
	if repoHdr == nil || repoHdr.badgeStatus != StatusNeedsAttention || repoHdr.badgeCount != 1 {
		t.Fatalf("repo bar should badge the most-urgent status (attn, 1), got %+v", repoHdr)
	}
	if incHdr == nil || incHdr.note != "not a git repo" {
		t.Fatalf("incidental bar should state its reason, got %+v", incHdr)
	}

	// A section with only structural rows (no live agents) carries no status badge.
	_, items2 := groupRows([]Row{{Kind: RowAnchor, Repo: "r", GitDir: "/g2", Worktree: "main", IsPrimary: true}})
	for i := range items2 {
		if items2[i].kind == kindSession && items2[i].badgeStatus != "" {
			t.Fatalf("a no-agent section should have no badge, got %q", items2[i].badgeStatus)
		}
	}
}

// TestSectionSummaryOnlyWhenFolded: a section bar's right-side summary (urgency badge +
// worktree count) is shown only when the section is folded, where the rows are hidden.
// Expanded, the rows speak for themselves so the bar carries no summary.
func TestSectionSummaryOnlyWhenFolded(t *testing.T) {
	rows := []Row{
		{Kind: RowAnchor, SessionID: "anchor", TmuxSession: "proj", Repo: "proj", GitDir: "/g", Worktree: "main", IsPrimary: true},
		{Kind: RowAgent, SessionID: "a", TmuxSession: "proj", Repo: "proj", GitDir: "/g", Worktree: "feat", Title: "x", Status: StatusNeedsAttention, TmuxWindow: "1", TmuxPane: "%1"},
	}
	m, err := newModel(fixedRows{rows}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 200, 30
	m.focus = lensWorkspaces

	// The managed section header is the one line carrying the worktree-source glyph.
	header := func() string {
		for _, line := range strings.Split(stripANSI(m.renderRows()), "\n") {
			if strings.Contains(line, managedGlyph) {
				return line
			}
		}
		return ""
	}
	badge := statusGlyph[StatusNeedsAttention]

	if exp := header(); strings.Contains(exp, "wt") || strings.Contains(exp, badge) {
		t.Fatalf("expanded section bar should carry no badge/wt summary, got %q", exp)
	}
	m.folded["/g"] = true
	if fol := header(); !strings.Contains(fol, "wt") || !strings.Contains(fol, badge) {
		t.Fatalf("folded section bar should carry the badge + wt summary, got %q", fol)
	}
}

// TestSectionBadgeExcludesMuted: a muted agent does not raise its section's most-urgent
// badge, so a section whose only live agent is muted shows no badge at all.
func TestSectionBadgeExcludesMuted(t *testing.T) {
	// A muted needs-attention agent alongside an unmuted working one: the badge reflects the
	// working agent, not the louder-but-muted one.
	st, n := sectionBadge([]Row{
		{Kind: RowAgent, SessionID: "a", Title: "x", Status: StatusNeedsAttention, Muted: true},
		{Kind: RowAgent, SessionID: "b", Title: "y", Status: StatusWorking},
	})
	if st != StatusWorking || n != 1 {
		t.Fatalf("muted agent should not raise the badge; want (working,1), got (%q,%d)", st, n)
	}
	// A section whose only live agent is muted carries no badge.
	if st, n := sectionBadge([]Row{{Kind: RowAgent, SessionID: "a", Title: "x", Status: StatusNeedsAttention, Muted: true}}); st != "" || n != 0 {
		t.Fatalf("a muted-only section should have no badge, got (%q,%d)", st, n)
	}
}

// TestAgentsLensStructure: the Agents lens always carries all four band headers, with the
// live agents as navigable leaves beneath them.
func TestAgentsLensStructure(t *testing.T) {
	m := mustModel(t, agentsN(2), nil, DefaultKeymap()) // a,b both working
	headers, agents := 0, 0
	for _, it := range m.agentItems {
		if it.header {
			headers++
		} else {
			agents++
		}
	}
	if headers != int(numBands) {
		t.Fatalf("want %d band headers always present, got %d", numBands, headers)
	}
	if agents != 2 || len(m.agentNav) != 2 {
		t.Fatalf("want 2 navigable agents, got %d items / %d nav", agents, len(m.agentNav))
	}
}

// TestDefaultFocusIsAgents pins the production initial state: the dash opens focused on
// the Agents (left) triage lens. (mustModel overrides this to Workspaces for the bulk of
// interaction tests; here we build the model directly to assert the real default.)
func TestDefaultFocusIsAgents(t *testing.T) {
	m, err := nm(&fakeSource{list: agentsN(2)}, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	if m.focus != lensAgents {
		t.Fatalf("dash should open focused on the Agents lens, got %v", m.focus)
	}
}

// TestLensFocusSwitchCarriesCounterpart: h focuses the left (Agents) lens and lands on the
// same agent selected in Workspaces; l returns focus to the right (Workspaces) lens.
// Navigation routes to the focused lens.
func TestLensFocusSwitchCarriesCounterpart(t *testing.T) {
	m := mustModel(t, agentsN(3), nil, DefaultKeymap()) // a,b,c working, stable order
	m = send(m, key("j"))                               // workspaces cursor → b
	if r, _ := m.currentRow(); r.SessionID != "b" {
		t.Fatalf("precondition: workspaces should select b, got %s", r.SessionID)
	}

	m = send(m, key("h")) // focus the left (Agents) lens
	if m.focus != lensAgents {
		t.Fatalf("h should focus the Agents lens")
	}
	if r, ok := m.currentRow(); !ok || r.SessionID != "b" {
		t.Fatalf("Agents lens should land on the counterpart b, got ok=%v id=%s", ok, r.SessionID)
	}

	m = send(m, key("j")) // navigation now moves the Agents cursor
	if r, _ := m.currentRow(); r.SessionID != "c" {
		t.Fatalf("j in the Agents lens should advance to c, got %s", r.SessionID)
	}

	m = send(m, key("l")) // back to Workspaces; selection is linked, so it follows to c
	if m.focus != lensWorkspaces {
		t.Fatalf("l should focus the Workspaces lens")
	}
	if r, _ := m.currentRow(); r.SessionID != "c" {
		t.Fatalf("the linked selection should carry c back to Workspaces, got %s", r.SessionID)
	}
}

// TestMirrorUsesBulletNotASecondCursor pins the dual-lens highlight hierarchy: the focused
// lens marks its selection with the cursor arrow, and the unfocused lens echoes the same
// agent with the mirror bullet - not a second cursor arrow. Without this the mirror could
// drift back to reusing CursorGlyph and read as a rival pointer.
func TestMirrorUsesBulletNotASecondCursor(t *testing.T) {
	m := mustModel(t, agentsN(3), nil, DefaultKeymap()) // focus defaults to Workspaces
	m.width, m.height = 200, 30                         // wide → both lenses render
	if !m.dualLens() {
		t.Fatalf("precondition: 200 cols should be dual-lens")
	}
	m = send(m, key("j")) // select an agent (b) so it has a counterpart to mirror

	l := m.renderLenses()
	if n := strings.Count(l, theme.CursorGlyph); n != 1 {
		t.Fatalf("exactly one cursor arrow expected (the focused selection), got %d:\n%s", n, l)
	}
	if !strings.Contains(l, theme.MirrorGlyph) {
		t.Fatalf("unfocused lens should echo the selection with the mirror bullet:\n%s", l)
	}
}

// TestAgentBandFoldCollapsesAndUnfolds: tab in the Agents lens folds the band under the
// cursor (collapsing its agents onto the header stand-in) and unfolds it again, mirroring
// the Workspaces fold.
func TestAgentBandFoldCollapsesAndUnfolds(t *testing.T) {
	in := []Agent{
		{SessionID: "u", Title: "urgent", Status: StatusNeedsAttention, TmuxSession: "u"},
		{SessionID: "w1", Title: "busyone", Status: StatusWorking, TmuxSession: "w1"},
		{SessionID: "w2", Title: "busytwo", Status: StatusWorking, TmuxSession: "w2"},
	}
	m, err := nm(&fakeSource{list: in}, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = lensAgents
	m.width, m.height = 80, 30 // narrow → only the focused (agents) lens renders

	if len(m.agentNav) != 3 {
		t.Fatalf("unfolded: want 3 navigable agents, got %d", len(m.agentNav))
	}

	m = send(m, key("j")) // cursor: u (NEEDS YOU) → w1 (WORKING)
	if r, ok := m.agentsCurrentRow(); !ok || r.SessionID != "w1" {
		t.Fatalf("precondition: cursor should be on w1, got ok=%v id=%s", ok, r.SessionID)
	}

	m = send(m, key("tab")) // fold WORKING
	if len(m.agentNav) != 2 {
		t.Fatalf("after fold: want 2 navigable items (u + folded WORKING header), got %d", len(m.agentNav))
	}
	if _, ok := m.agentsCurrentRow(); ok {
		t.Fatalf("cursor should rest on the folded WORKING header, not a single agent row")
	}
	if it := m.agentItems[m.agentNav[m.agentCursor]]; !it.header || it.band != bandWorking {
		t.Fatalf("cursor should rest on the WORKING band header, got %+v", it)
	}
	v := m.View()
	if strings.Contains(v, "busyone") || strings.Contains(v, "busytwo") {
		t.Fatalf("a folded band must hide its agents:\n%s", v)
	}
	if !strings.Contains(v, "▸ WORKING") {
		t.Fatalf("folded band header should show the collapsed glyph and label:\n%s", v)
	}

	m = send(m, key("tab")) // unfold
	if len(m.agentNav) != 3 {
		t.Fatalf("after unfold: want 3 navigable agents again, got %d", len(m.agentNav))
	}
	if r, ok := m.agentsCurrentRow(); !ok || r.SessionID != "w1" {
		t.Fatalf("unfolding should land on the band's first agent w1, got ok=%v id=%s", ok, r.SessionID)
	}
	if v := m.View(); !strings.Contains(v, "busyone") {
		t.Fatalf("unfolded band must show its agents again:\n%s", v)
	}
}

// TestDualLensLayoutAndNarrowFallback: a wide terminal shows both lens panels; a narrow one
// shows only the focused lens (the fallback), still switchable with the focus keys.
func TestDualLensLayoutAndNarrowFallback(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{}}
	m := mustModel(t, agentsN(2), prev, DefaultKeymap())

	// Assert on renderLenses (the lens area) rather than View, so the help legend's
	// "fold (workspaces)" hint doesn't masquerade as a lens title.
	m.width, m.height = 200, 30
	if l := m.renderLenses(); !strings.Contains(l, "workspaces") || !strings.Contains(l, "agents") {
		t.Fatalf("wide layout should show both lens titles")
	}

	m.width, m.height = 80, 30 // below dualLensMinWidth → single lens
	if m.dualLens() {
		t.Fatalf("80 cols should not be dual-lens")
	}
	if l := m.renderLenses(); !strings.Contains(l, "workspaces") || strings.Contains(l, "agents") {
		t.Fatalf("narrow layout should show only the focused (workspaces) lens")
	}
	// Toggling focus swaps which single lens shows; h focuses the left (agents) lens.
	m = send(m, key("h"))
	if l := m.renderLenses(); !strings.Contains(l, "agents") || strings.Contains(l, "workspaces") {
		t.Fatalf("narrow layout after focus-left should show only the agents lens")
	}
}

func TestFoldCollapsesAndUnfolds(t *testing.T) {
	in := []Agent{
		ag("arewa", "1", "refactor auth", StatusNeedsAttention),
		ag("arewa", "1", "write tests", StatusWorking),
		ag("web", "0", "ship landing", StatusIdle),
	}
	m, err := rowModel(DefaultKeymap(), in...)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.nav) != 3 {
		t.Fatalf("unfolded: want 3 navigable rows (the agents), got %d", len(m.nav))
	}

	// Cursor starts on arewa's first agent; tab folds that section.
	m = send(m, key("tab"))
	if len(m.nav) != 2 {
		t.Fatalf("after fold: want 2 navigable rows (arewa header + web agent), got %d", len(m.nav))
	}
	it, _ := m.currentItem()
	if it.kind != kindSession || it.sessionKey != "arewa" {
		t.Fatalf("cursor should rest on the folded arewa header, got %+v", it)
	}
	v := m.View()
	if strings.Contains(v, "refactor auth") || strings.Contains(v, "write tests") {
		t.Fatalf("a folded section must hide its agents:\n%s", v)
	}
	if !strings.Contains(v, "▸ "+managedGlyph+" arewa") {
		t.Fatalf("folded header should show the collapsed glyph and label:\n%s", v)
	}

	// Tab again unfolds; the cursor drops onto the section's first agent.
	m = send(m, key("tab"))
	if len(m.nav) != 3 {
		t.Fatalf("after unfold: want 3 navigable rows, got %d", len(m.nav))
	}
	a, ok := m.currentRow()
	if !ok || a.Title != "refactor auth" {
		t.Fatalf("unfold should land on the first agent, got %+v ok=%v", a, ok)
	}
}

func TestFoldStateSurvivesRefresh(t *testing.T) {
	m, err := rowModel(DefaultKeymap(),
		ag("arewa", "1", "a1", StatusWorking),
		ag("arewa", "1", "a2", StatusWorking),
		ag("web", "0", "w1", StatusWorking),
	)
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, key("tab")) // fold arewa (cursor starts on an arewa agent)
	if !m.folded["arewa"] {
		t.Fatal("arewa should be folded after tab")
	}

	m = send(m, tickMsg(time.Now()))
	if !m.folded["arewa"] {
		t.Fatal("fold state should survive a live refresh")
	}
	it, _ := m.currentItem()
	if it.kind != kindSession || it.sessionKey != "arewa" {
		t.Fatalf("selection should stay on the folded arewa header across refresh, got %+v", it)
	}
}

// TestLocatorHint pins the cross-session locator: a row whose pane lives outside its
// repository's home session carries an `[in: <session>]` hint naming that session, while a
// row in the home, a windowless row, and an ungrouped incidental agent carry none.
func TestLocatorHint(t *testing.T) {
	const gd = "/code/proj/.git"
	home := dir.HomeSession(gd)

	// In the home session: no hint.
	if h := locatorHint(Row{GitDir: gd, TmuxSession: home}); h != "" {
		t.Errorf("a row in the home session should have no hint, got %q", h)
	}
	// In a user session: hint naming that session.
	if h := locatorHint(Row{GitDir: gd, TmuxSession: "proj"}); h != "[in: proj]" {
		t.Errorf("an out-of-home row should be flagged, got %q", h)
	}
	// Windowless (no session) and ungrouped (no repo): no hint.
	if h := locatorHint(Row{GitDir: gd}); h != "" {
		t.Errorf("a windowless row should have no hint, got %q", h)
	}
	if h := locatorHint(Row{TmuxSession: "scratch"}); h != "" {
		t.Errorf("an ungrouped incidental agent should have no hint, got %q", h)
	}
}

// TestColumnsFlexToFitContent pins that the worktree and title columns flex to fit their widest
// values (no clipping while space sits empty), clamped to their bands, and that the hint is its
// own column rather than being folded into the title width.
func TestColumnsFlexToFitContent(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{}}
	rows := []Row{
		// A long worktree label and a long title, both well past the baselines.
		{Kind: RowAgent, SessionID: "a", TmuxSession: "be", Repo: "r", GitDir: "/g", Worktree: "a-long-worktree-name", Title: "a fairly long agent title here", Status: StatusWorking, TmuxWindow: "1", TmuxPane: "%1"},
	}
	m, err := newModel(fixedRows{rows}, prev, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.width = 200

	windowW, nameW, _ := m.colWidths()
	if windowW <= windowColMin {
		t.Fatalf("the worktree column should flex past the baseline %d to fit a long label, got %d", windowColMin, windowW)
	}
	if windowW < lipgloss.Width("a-long-worktree-name")+1 || windowW > windowColMax {
		t.Fatalf("the worktree column should fit the label (clamped to %d), got %d", windowColMax, windowW)
	}
	if nameW <= nameColMin {
		t.Fatalf("the title column should flex past the baseline %d to fit a long title, got %d", nameColMin, nameW)
	}
}

// TestListFillsSplitShare pins that on a wide terminal the list pane fills its configured share
// of the width (rather than collapsing to its content), so the preview is not left oversized.
func TestListFillsSplitShare(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{}}
	rows := []Row{
		{Kind: RowAgent, SessionID: "a", TmuxSession: "be", Repo: "r", GitDir: "/g", Worktree: "wt", Title: "qa", Status: StatusWorking, TmuxWindow: "1", TmuxPane: "%1"},
	}
	m, err := newModel(fixedRows{rows}, prev, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.width = 200
	m.split = 0.55

	rendered := cursorColWidth + m.rowContentWidth() + 4
	// The Workspaces list fills its share of the region left after the Agents lens claims its
	// column (mainWidth), not the full terminal - the two lenses share the list area now.
	want := int(float64(m.mainWidth()) * m.split)
	// The pane should sit at its share (within a small rounding/frame slack), not collapse to
	// the narrow content of a one-row list.
	if rendered < want-6 || rendered > want+6 {
		t.Fatalf("the list should fill ~%d%% of mainWidth %d (=%d), got %d", int(m.split*100), m.mainWidth(), want, rendered)
	}
}

// TestHintsAlignInColumn pins that every locator hint starts at the same column regardless of
// title length: rows with a long title, a short title, and no title must align their "[in: …]".
func TestHintsAlignInColumn(t *testing.T) {
	prev := &fakePreviewer{out: map[string]string{}}
	const gd = "/g"
	rows := []Row{
		{Kind: RowAnchor, SessionID: "x", TmuxSession: "proj", Repo: "r", GitDir: gd, Worktree: "r", IsPrimary: true},
		{Kind: RowAgent, SessionID: "a", TmuxSession: "proj", Repo: "r", GitDir: gd, Worktree: "r", Title: "qa", Status: StatusWorking, TmuxWindow: "1", TmuxPane: "%1"},
		{Kind: RowAgent, SessionID: "b", TmuxSession: "proj", Repo: "r", GitDir: gd, Worktree: "feature-x", Title: "a-much-longer-agent-title", Status: StatusIdle, TmuxWindow: "2", TmuxPane: "%2"},
	}
	m, err := newModel(fixedRows{rows}, prev, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.width = 200

	var cols []int
	for _, line := range strings.Split(stripANSI(m.renderRows()), "\n") {
		if i := strings.Index(line, "[in:"); i >= 0 {
			cols = append(cols, lipgloss.Width(line[:i])) // visual column, not byte offset
		}
	}
	if len(cols) < 3 {
		t.Fatalf("expected a hint on each of the 3 rows, found %d in:\n%s", len(cols), stripANSI(m.renderRows()))
	}
	for _, c := range cols[1:] {
		if c != cols[0] {
			t.Fatalf("locator hints should align in a column, got start columns %v", cols)
		}
	}
}

func TestResolveAccent(t *testing.T) {
	// Empty falls back to the built-in default (non-empty color).
	def, err := ResolveAccent("")
	if err != nil || def == "" {
		t.Fatalf("empty accent should yield the default color, got %q err=%v", def, err)
	}
	// A valid hex is used verbatim.
	got, err := ResolveAccent("#ff8800")
	if err != nil || string(got) != "#ff8800" {
		t.Fatalf("valid hex should pass through, got %q err=%v", got, err)
	}
	// Malformed values are rejected rather than silently ignored.
	for _, bad := range []string{"ff8800", "#fff", "#gggggg", "#ff88000"} {
		if _, err := ResolveAccent(bad); err == nil {
			t.Errorf("accent %q should be rejected", bad)
		}
	}
}

func TestDisplayTitleStripsSessionPrefix(t *testing.T) {
	cases := []struct {
		title, session, want string
	}{
		{"arewa:birds_eye", "arewa", "birds_eye"}, // session prefix dropped
		{"standalone", "arewa", "standalone"},     // no prefix: unchanged
		{"arewa", "arewa", "arewa"},               // would-be-empty: keep original
		{"x:y", "", "x:y"},                        // ungrouped: unchanged
	}
	for _, c := range cases {
		if got := displayTitle(c.title, c.session); got != c.want {
			t.Errorf("displayTitle(%q, %q) = %q, want %q", c.title, c.session, got, c.want)
		}
	}
}

func TestSectionJumpMotions(t *testing.T) {
	// Three sessions; arewa has two agents in one window, the others one each.
	in := []Agent{
		ag("arewa", "1", "a1", StatusWorking),
		ag("arewa", "1", "a2", StatusWorking),
		ag("beta", "0", "b1", StatusWorking),
		ag("gamma", "0", "g1", StatusWorking),
	}
	m, err := rowModel(DefaultKeymap(), in...)
	if err != nil {
		t.Fatal(err)
	}
	// nav: [a1, a2, b1, g1]; section starts at 0 (arewa), 2 (beta), 3 (gamma).
	if starts := m.sectionStarts(); !reflect.DeepEqual(starts, []int{0, 2, 3}) {
		t.Fatalf("section starts = %v, want [0 2 3]", starts)
	}

	// } from inside arewa jumps to the start of beta.
	m = send(m, key("}"))
	if m.cursor != 2 {
		t.Fatalf("} should jump to next section (beta) at nav 2, got %d", m.cursor)
	}
	// } again → gamma.
	m = send(m, key("}"))
	if m.cursor != 3 {
		t.Fatalf("} should jump to gamma at nav 3, got %d", m.cursor)
	}
	// } at the last section clamps to the last row.
	m = send(m, key("}"))
	if m.cursor != 3 {
		t.Fatalf("} at last section should stay at last row 3, got %d", m.cursor)
	}

	// Move into the middle of arewa, then { goes to that section's top first.
	m = send(m, key("g")) // arm gg
	m = send(m, key("g")) // back to top (a1)
	m = send(m, key("j")) // a2 (still arewa, nav 1)
	m = send(m, key("{"))
	if m.cursor != 0 {
		t.Fatalf("{ from mid-section should go to section top (nav 0), got %d", m.cursor)
	}
	// { again at the top of the first section stays at 0.
	m = send(m, key("{"))
	if m.cursor != 0 {
		t.Fatalf("{ at first section top should stay at 0, got %d", m.cursor)
	}
	// From gamma, { steps back to beta's top.
	m = send(m, key("G")) // gamma (nav 3)
	m = send(m, key("{"))
	if m.cursor != 2 {
		t.Fatalf("{ from gamma should go to beta top (nav 2), got %d", m.cursor)
	}
}

func TestNavigationCrossesGroupsLeavesOnly(t *testing.T) {
	in := []Agent{
		ag("api", "1", "a1", StatusWorking),
		ag("api", "1", "a2", StatusWorking),
		ag("web", "0", "w1", StatusWorking),
	}
	m, err := rowModel(DefaultKeymap(), in...)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.rows) != 3 {
		t.Fatalf("expected 3 selectable leaves, got %d", len(m.rows))
	}

	// G lands on the last agent leaf, never a header; one j past it stays put.
	m = send(m, key("G"))
	if m.cursor != 2 || m.rows[m.cursor].Title != "w1" {
		t.Fatalf("G should land on last agent w1, got cursor %d", m.cursor)
	}

	// The view brackets the leaves with session bars and shows each agent's window label.
	v := m.View()
	if !strings.Contains(v, "api") || !strings.Contains(v, "win 1") || !strings.Contains(v, "web") {
		t.Fatalf("view should show session bars and window labels:\n%s", v)
	}
}

// TestOverlayCenter checks the modal compositor: the foreground is spliced centered
// into the background, the canvas keeps the background's dimensions, and background
// cells outside the modal footprint survive intact.
func TestOverlayCenter(t *testing.T) {
	bg := strings.Join([]string{
		"..........",
		"..........",
		"..........",
		"..........",
		"..........",
	}, "\n")
	fg := strings.Join([]string{
		"####",
		"####",
	}, "\n")

	out := overlayCenter(bg, fg)
	lines := strings.Split(out, "\n")

	if got := len(lines); got != 5 {
		t.Fatalf("overlay changed row count: got %d, want 5", got)
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != 10 {
			t.Fatalf("row %d width = %d, want 10 (background width preserved)", i, w)
		}
	}
	// fg is 2 rows tall over 5 → top padding (5-2)/2 = 1, so rows 1 and 2 carry it.
	// fg is 4 wide over 10 → left (10-4)/2 = 3, occupying columns [3,7).
	if want := "...####..."; lines[1] != want {
		t.Fatalf("row 1 = %q, want %q", lines[1], want)
	}
	if want := "...####..."; lines[2] != want {
		t.Fatalf("row 2 = %q, want %q", lines[2], want)
	}
	for _, i := range []int{0, 3, 4} {
		if lines[i] != ".........." {
			t.Fatalf("row %d should be untouched background, got %q", i, lines[i])
		}
	}
}
