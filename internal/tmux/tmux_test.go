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

	if err := c.Ensure("proj", "/tmp/proj"); err != nil {
		t.Fatal(err)
	}
	if !rec.called("new-session -d -s proj -c /tmp/proj") {
		t.Fatalf("expected new-session call, calls=%v", rec.calls)
	}
}

func TestEnsureReusesWhenPresent(t *testing.T) {
	rec := newRecorder()
	rec.replies["has-session -t =proj"] = "" // present (no error)
	c := NewWithRunner(rec.run, false)

	if err := c.Ensure("proj", "/tmp/proj"); err != nil {
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
