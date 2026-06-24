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
)

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

func mustModel(t *testing.T, list []Agent, prev Previewer, km Keymap) model {
	t.Helper()
	m, err := nm(&fakeSource{list: list}, prev, km)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// nm builds a model from a plain agent Source (wrapped as rows) with the default
// refresh, mirroring the old newModel(Source, ...) test signature.
func nm(src Source, prev Previewer, km Keymap) (model, error) {
	return newModel(AgentsAsRows(src), prev, km, 0)
}

// rowsOf converts agents to RowAgent rows for the grouping tests.
func rowsOf(ags ...Agent) []Row {
	out := make([]Row, len(ags))
	for i, a := range ags {
		out[i] = agentRow(a)
	}
	return out
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

func TestRefreshPreservesSelectionAcrossResort(t *testing.T) {
	src := &fakeSource{list: agentsN(3)} // sessions a,b,c all working → grouped order a,b,c
	m, err := nm(src, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	if m.rows[1].SessionID != "b" {
		t.Fatalf("precondition: expected b at index 1, got %s", m.rows[1].SessionID)
	}
	m.cursor = 1 // on "b"

	// b's session now needs attention, so its group floats to the top and b moves
	// to index 0. Selection must follow b by identity, not cling to index 1.
	updated := agentsN(3)
	updated[1].Status = StatusNeedsAttention
	src.list = updated
	m = send(m, tickMsg(time.Now()))
	if m.cursor != 0 || m.rows[m.cursor].SessionID != "b" {
		t.Fatalf("selection should follow agent b to index 0, got %d (%s)", m.cursor, m.rows[m.cursor].SessionID)
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
		{Kind: RowSlot, SessionID: "slot:proj:spike", TmuxSession: "proj", Worktree: "spike", Managed: true, Dir: "/d"},
		{Kind: RowAnchor, SessionID: "anchor:proj", TmuxSession: "proj", Worktree: "main", Managed: true, Dir: "/d", IsPrimary: true},
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
	long := strings.Repeat("the quick brown fox jumps over the lazy dog ", 6)
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString(long)
		sb.WriteByte('\n')
	}
	prev := &fakePreviewer{out: map[string]string{"a": sb.String(), "b": sb.String()}}

	for _, dim := range []struct{ w, h int }{{120, 24}, {160, 30}, {100, 40}} {
		m := mustModel(t, agentsN(2), prev, DefaultKeymap())
		m.width, m.height = dim.w, dim.h
		if h := lipgloss.Height(m.View()); h > m.height {
			t.Fatalf("at %dx%d: view height %d exceeds terminal height %d (would scroll list out of view)",
				dim.w, dim.h, h, m.height)
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
		ag("web", "0", "ship landing", StatusDone),
	}
	ordered, items := groupRows(rowsOf(in...))

	want := []string{
		"S:api-server",          // needs-attn session floats above web (all-done)
		"A:refactor auth@win 1", // agents ordered by urgency within the session, window is
		"A:write tests@win 1",   // just a label (here win 1's two agents happen to be adjacent)
		"A:migrate db@win 2",    // no window headers, no inline-collapse special case
		"S:web",
		"A:ship landing@win 0",
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("grouped shape mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestGroupAgentsWindowLabelNotForcedContiguous(t *testing.T) {
	// Within one session, urgency ordering wins: two agents sharing a window are not
	// pulled adjacent — a more-urgent agent from another window sits between them.
	in := []Agent{
		ag("s", "1", "low", StatusIdle),
		ag("s", "1", "high", StatusNeedsAttention),
		ag("s", "2", "mid", StatusWorking),
	}
	ordered, items := groupRows(rowsOf(in...))
	want := []string{
		"S:s",
		"A:high@win 1", // needs-attn
		"A:mid@win 2",  // working sits between the two win 1 agents
		"A:low@win 1",  // idle
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("non-contiguity shape mismatch:\n got %v\nwant %v", got, want)
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
		ag("api", "0", "real one", StatusDone),
		{SessionID: "loose-id", Title: "loose", Status: StatusWorking}, // no tmux session
	}
	ordered, items := groupRows(rowsOf(in...))

	// The ungrouped bucket (working, more urgent) floats above the done session.
	want := []string{
		"S:ungrouped",
		"A:loose", // no tmux window → no inline label
		"S:api",
		"A:real one@win 0",
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("ungrouped shape mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestFoldCollapsesAndUnfolds(t *testing.T) {
	in := []Agent{
		ag("arewa", "1", "refactor auth", StatusNeedsAttention),
		ag("arewa", "1", "write tests", StatusWorking),
		ag("web", "0", "ship landing", StatusDone),
	}
	m, err := nm(&fakeSource{list: in}, nil, DefaultKeymap())
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
	if !strings.Contains(v, "▸ arewa") {
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
	src := &fakeSource{list: []Agent{
		ag("arewa", "1", "a1", StatusWorking),
		ag("arewa", "1", "a2", StatusWorking),
		ag("web", "0", "w1", StatusWorking),
	}}
	m, err := nm(src, nil, DefaultKeymap())
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
	m, err := nm(&fakeSource{list: in}, nil, DefaultKeymap())
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
	m, err := nm(&fakeSource{list: in}, nil, DefaultKeymap())
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
