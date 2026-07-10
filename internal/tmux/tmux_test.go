package tmux

import (
	"errors"
	"strings"
	"testing"
)

// recorder is a fake Runner that records calls and replies from a script.
type recorder struct {
	calls   [][]string
	replies map[string]string // joined args -> stdout
	errs    map[string]error  // joined args -> error
}

func newRecorder() *recorder {
	return &recorder{replies: map[string]string{}, errs: map[string]error{}}
}

func (r *recorder) run(args ...string) (string, error) {
	r.calls = append(r.calls, args)
	key := strings.Join(args, " ")
	if err, ok := r.errs[key]; ok {
		return "", err
	}
	return r.replies[key], nil
}

func (r *recorder) called(prefix string) bool {
	for _, c := range r.calls {
		if strings.HasPrefix(strings.Join(c, " "), prefix) {
			return true
		}
	}
	return false
}

func TestEnsureCreatesWhenAbsent(t *testing.T) {
	rec := newRecorder()
	rec.errs["has-session -t =proj"] = errors.New("no session") // absent
	c := NewWithRunner(rec.run, false)

	if err := c.Ensure("proj", "/tmp/proj", "proj"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("new-session -d -s proj -c /tmp/proj -n proj") {
		t.Fatalf("expected new-session naming window 0, calls=%v", rec.calls)
	}
}

func TestEnsureReusesWhenPresent(t *testing.T) {
	rec := newRecorder()
	rec.replies["has-session -t =proj"] = "" // present (no error)
	c := NewWithRunner(rec.run, false)

	if err := c.Ensure("proj", "/tmp/proj", "proj"); err != nil {
		t.Fatal(err)
	}
	if rec.called("new-session") {
		t.Fatalf("should not create when session exists, calls=%v", rec.calls)
	}
}

func TestConnectAttachOutsideTmux(t *testing.T) {
	rec := newRecorder()
	c := NewWithRunner(rec.run, false)
	if err := c.Connect("proj"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("attach-session -t proj") {
		t.Fatalf("expected attach-session outside tmux, calls=%v", rec.calls)
	}
}

func TestConnectPaneFocusesWindowAndPane(t *testing.T) {
	rec := newRecorder()
	c := NewWithRunner(rec.run, true) // inside tmux -> switch-client
	if err := c.ConnectPane("proj", "2", "%5"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("select-window -t %5") {
		t.Fatalf("expected select-window by pane id, calls=%v", rec.calls)
	}
	if !rec.called("select-pane -t %5") {
		t.Fatalf("expected select-pane by pane id, calls=%v", rec.calls)
	}
	if !rec.called("switch-client -t proj") {
		t.Fatalf("expected switch-client to land on the session, calls=%v", rec.calls)
	}
}

func TestConnectPaneFallsBackToWindow(t *testing.T) {
	rec := newRecorder()
	c := NewWithRunner(rec.run, false)
	if err := c.ConnectPane("proj", "3", ""); err != nil {
		t.Fatal(err)
	}
	if !rec.called("select-window -t proj:3") {
		t.Fatalf("expected select-window by session:window when no pane, calls=%v", rec.calls)
	}
	if !rec.called("attach-session -t proj") {
		t.Fatalf("expected attach outside tmux, calls=%v", rec.calls)
	}
}

func TestConnectSwitchInsideTmux(t *testing.T) {
	rec := newRecorder()
	c := NewWithRunner(rec.run, true)
	if err := c.Connect("proj"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("switch-client -t proj") {
		t.Fatalf("expected switch-client inside tmux, calls=%v", rec.calls)
	}
}

func TestCapturePane(t *testing.T) {
	rec := newRecorder()
	rec.replies["capture-pane -p -e -t proj:1"] = "line one\nline two\n"
	c := NewWithRunner(rec.run, false)

	out, err := c.CapturePane("proj:1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if out != "line one\nline two\n" {
		t.Fatalf("unexpected capture output: %q", out)
	}

	rec2 := newRecorder()
	rec2.replies["capture-pane -p -e -t proj -S -100"] = "scrollback"
	c2 := NewWithRunner(rec2.run, false)
	if _, err := c2.CapturePane("proj", 100); err != nil {
		t.Fatal(err)
	}
	if !rec2.called("capture-pane -p -e -t proj -S -100") {
		t.Fatalf("expected scrollback range arg, calls=%v", rec2.calls)
	}

	rec3 := newRecorder()
	rec3.errs["capture-pane -p -e -t gone"] = errors.New("no such session")
	c3 := NewWithRunner(rec3.run, false)
	if _, err := c3.CapturePane("gone", 0); err == nil {
		t.Fatal("expected error to propagate when capture fails")
	}
}

func TestListSessionsParsesAndToleratesNoServer(t *testing.T) {
	rec := newRecorder()
	rec.replies["list-sessions -F #{session_name}"] = "alpha\nbeta\n"
	c := NewWithRunner(rec.run, false)
	got, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("expected [alpha beta], got %v", got)
	}

	rec2 := newRecorder()
	rec2.errs["list-sessions -F #{session_name}"] = errors.New("no server running")
	c2 := NewWithRunner(rec2.run, false)
	got2, err := c2.ListSessions()
	if err != nil || len(got2) != 0 {
		t.Fatalf("no-server should yield empty, no error; got %v err %v", got2, err)
	}
}

func TestListPanesParsesStructure(t *testing.T) {
	rec := newRecorder()
	rec.replies["list-panes -a -F #{session_name}\t#{window_index}\t#{window_name}\t#{pane_id}\t#{pane_start_path}"] =
		"arewa\t0\tmain\t%1\t/home/u/birds_eye/main\narewa\t1\tnvim\t%2\t/home/u/birds_eye/feat\n"
	c := NewWithRunner(rec.run, false)

	panes, err := c.ListPanes()
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d: %+v", len(panes), panes)
	}
	if panes[0] != (Pane{Session: "arewa", WindowIndex: "0", WindowName: "main", PaneID: "%1", StartPath: "/home/u/birds_eye/main"}) {
		t.Fatalf("pane[0] mismatch: %+v", panes[0])
	}
	if panes[1].StartPath != "/home/u/birds_eye/feat" || panes[1].PaneID != "%2" {
		t.Fatalf("pane[1] mismatch: %+v", panes[1])
	}
}

func TestListPanesNoServerIsEmpty(t *testing.T) {
	rec := newRecorder()
	rec.errs["list-panes -a -F #{session_name}\t#{window_index}\t#{window_name}\t#{pane_id}\t#{pane_start_path}"] = errors.New("no server")
	c := NewWithRunner(rec.run, false)
	panes, err := c.ListPanes()
	if err != nil || panes != nil {
		t.Fatalf("no server should yield empty, got %+v err=%v", panes, err)
	}
}

func TestPaneTitlesParses(t *testing.T) {
	rec := newRecorder()
	rec.replies["list-panes -a -F #{pane_id}\t#{pane_title}"] =
		"%1\t\xe2\xa0\x82 Working on it\n%2\t\xe2\x9c\xb3 Idle here\n%3\t\n"
	c := NewWithRunner(rec.run, false)

	titles, err := c.PaneTitles()
	if err != nil {
		t.Fatal(err)
	}
	if got := titles["%1"]; got != "⠂ Working on it" {
		t.Fatalf("title[%%1] = %q", got)
	}
	if got := titles["%2"]; got != "✳ Idle here" {
		t.Fatalf("title[%%2] = %q", got)
	}
	// A pane with an empty title is still present, mapped to "".
	if got, ok := titles["%3"]; !ok || got != "" {
		t.Fatalf("title[%%3] = %q ok=%v, want empty present", got, ok)
	}
}

func TestPaneTitlesNoServerIsEmptyMap(t *testing.T) {
	rec := newRecorder()
	rec.errs["list-panes -a -F #{pane_id}\t#{pane_title}"] = errors.New("no server")
	c := NewWithRunner(rec.run, false)
	titles, err := c.PaneTitles()
	if err != nil || titles == nil || len(titles) != 0 {
		t.Fatalf("no server should yield an empty (non-nil) map, got %+v err=%v", titles, err)
	}
}

func TestLifecycleOps(t *testing.T) {
	rec := newRecorder()
	rec.replies["new-window -d -P -F #{window_id} -t proj: -c /tmp/wt -n wt"] = "@7\n"
	c := NewWithRunner(rec.run, false)

	id, err := c.NewWindow("proj", "/tmp/wt", "wt", "claude --permission-mode=auto")
	if err != nil || id != "@7" {
		t.Fatalf("NewWindow = (%q,%v), want (@7,nil)", id, err)
	}
	// The window is named after the worktree it holds, not the running process.
	if !rec.called("new-window -d -P -F #{window_id} -t proj: -c /tmp/wt -n wt") {
		t.Fatalf("expected new-window named after the worktree, calls=%v", rec.calls)
	}
	// The agent command runs inside the window's shell (via send-keys), not as tmux's
	// bare child, so per-directory env (direnv, profile) loads as in a real pane.
	if !rec.called("send-keys -t @7 claude --permission-mode=auto Enter") {
		t.Fatalf("expected the agent command typed via send-keys, calls=%v", rec.calls)
	}
	if err := c.KillWindow("@7"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("kill-window -t @7") {
		t.Fatalf("expected kill-window, calls=%v", rec.calls)
	}
	if err := c.KillSession("proj"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("kill-session -t =proj") {
		t.Fatalf("expected kill-session, calls=%v", rec.calls)
	}
}
