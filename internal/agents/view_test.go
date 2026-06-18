package agents

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	src := &fakeSource{list: agentsN(3)} // a,b,c
	m, err := newModel(src, nil, DefaultKeymap())
	if err != nil {
		t.Fatal(err)
	}
	m.cursor = 1 // on "b"

	// Re-sort so b lands at index 2.
	src.list = []Agent{
		{SessionID: "c", Title: "c", Status: StatusWorking},
		{SessionID: "a", Title: "a", Status: StatusWorking},
		{SessionID: "b", Title: "b", Status: StatusWorking},
	}
	m = send(m, tickMsg(time.Now()))
	if m.cursor != 2 || m.agents[m.cursor].SessionID != "b" {
		t.Fatalf("selection should follow agent b to index 2, got %d (%s)", m.cursor, m.agents[m.cursor].SessionID)
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

func TestViewEmptyState(t *testing.T) {
	m := mustModel(t, nil, nil, DefaultKeymap())
	if !strings.Contains(m.View(), "No agents") {
		t.Fatalf("empty view should show an informative empty state")
	}
}
