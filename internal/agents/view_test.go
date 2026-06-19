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
	m, err := newModel(&fakeSource{list: list}, prev, km)
	if err != nil {
		t.Fatal(err)
	}
	return m
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
	if m.pending != 0 {
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
	km, err := ResolveKeymap(map[string][]string{"down": {"n"}})
	if err != nil {
		t.Fatal(err)
	}
	m := mustModel(t, agentsN(3), nil, km)
	m = send(m, key("n"))
	if m.cursor != 1 {
		t.Fatalf("custom key n should move down, got %d", m.cursor)
	}
	m = send(m, key("j"))
	if m.cursor != 1 {
		t.Fatalf("default j should no longer move after rebind, got %d", m.cursor)
	}
}

func TestRefreshPreservesSelectionAcrossResort(t *testing.T) {
	src := &fakeSource{list: agentsN(3)} // sessions a,b,c all working → grouped order a,b,c
	m, err := newModel(src, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	if m.agents[1].SessionID != "b" {
		t.Fatalf("precondition: expected b at index 1, got %s", m.agents[1].SessionID)
	}
	m.cursor = 1 // on "b"

	// b's session now needs attention, so its group floats to the top and b moves
	// to index 0. Selection must follow b by identity, not cling to index 1.
	updated := agentsN(3)
	updated[1].Status = StatusNeedsAttention
	src.list = updated
	m = send(m, tickMsg(time.Now()))
	if m.cursor != 0 || m.agents[m.cursor].SessionID != "b" {
		t.Fatalf("selection should follow agent b to index 0, got %d (%s)", m.cursor, m.agents[m.cursor].SessionID)
	}
}

func TestRefreshClampsWhenSelectedVanishes(t *testing.T) {
	src := &fakeSource{list: agentsN(3)}
	m, _ := newModel(src, nil, DefaultKeymap())
	m.cursor = 2 // on "c"

	src.list = agentsN(1) // only "a" remains
	m = send(m, tickMsg(time.Now()))
	if m.cursor != 0 {
		t.Fatalf("vanished selection should clamp to 0, got %d", m.cursor)
	}
	if m.cursor >= len(m.agents) {
		t.Fatalf("cursor out of bounds: %d of %d", m.cursor, len(m.agents))
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
	if m.agents[0].Status != StatusWorking {
		t.Fatalf("status must not be derived from preview, got %q", m.agents[0].Status)
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

func TestViewEmptyState(t *testing.T) {
	m := mustModel(t, nil, nil, DefaultKeymap())
	if !strings.Contains(m.View(), "No agents") {
		t.Fatalf("empty view should show an informative empty state")
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

// shape renders the grouped items into compact tags for assertion: S:<session>,
// W:<window>(<count>), A:<title> (nested under a window header), A:<title>@<window>
// (collapsed single-agent window, window shown inline).
func shape(ordered []Agent, items []renderItem) []string {
	var got []string
	for _, it := range items {
		switch it.kind {
		case kindSession:
			got = append(got, "S:"+it.label)
		case kindWindow:
			got = append(got, fmt.Sprintf("W:%s(%d)", it.label, it.count))
		case kindAgent:
			tag := "A:" + ordered[it.agentIdx].Title
			switch {
			case it.nested:
				tag += "*"
			case it.window != "":
				tag += "@" + it.window
			}
			got = append(got, tag)
		}
	}
	return got
}

func TestGroupAgentsHierarchyCollapseAndOrder(t *testing.T) {
	in := []Agent{
		ag("api-server", "1", "write tests", StatusWorking),
		ag("api-server", "1", "refactor auth", StatusNeedsAttention),
		ag("api-server", "2", "migrate db", StatusIdle),
		ag("web", "0", "ship landing", StatusDone),
	}
	ordered, items := groupAgents(in)

	want := []string{
		"S:api-server",     // needs-attn session floats above web (all-done)
		"W:win 1(2)",       // window with 2 agents gets a header (index fallback)
		"A:refactor auth*", // needs-attn ordered first within the window
		"A:write tests*",
		"A:migrate db@win 2", // single-agent window collapses inline, even though
		"S:web",              // its session (api-server) spans multiple windows
		"A:ship landing@win 0",
	}
	if got := shape(ordered, items); !reflect.DeepEqual(got, want) {
		t.Fatalf("grouped shape mismatch:\n got %v\nwant %v", got, want)
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
	ordered, items := groupAgents(in)
	want := []string{
		"S:api",
		"W:editor(2)", // multi-agent window shows its tmux name, not "win 1"
		"A:a1*",
		"A:a2*",
		"A:a3@server", // collapsed single-agent window shows its name inline
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
	ordered, items := groupAgents(in)

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
	m, err := newModel(&fakeSource{list: in}, nil, DefaultKeymap())
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
	a, ok := m.currentAgent()
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
	m, err := newModel(src, nil, DefaultKeymap())
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
	m, err := newModel(&fakeSource{list: in}, nil, DefaultKeymap())
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
	m, err := newModel(&fakeSource{list: in}, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.agents) != 3 {
		t.Fatalf("expected 3 selectable leaves, got %d", len(m.agents))
	}

	// G lands on the last agent leaf, never a header; one j past it stays put.
	m = send(m, key("G"))
	if m.cursor != 2 || m.agents[m.cursor].Title != "w1" {
		t.Fatalf("G should land on last agent w1, got cursor %d", m.cursor)
	}

	// The view brackets the leaves with session and window headers.
	v := m.View()
	if !strings.Contains(v, "api") || !strings.Contains(v, "win 1") || !strings.Contains(v, "web") {
		t.Fatalf("view should show session and window headers:\n%s", v)
	}
}
