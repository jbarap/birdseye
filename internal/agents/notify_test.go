package agents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeNotifier struct{ calls []Notification }

func (f *fakeNotifier) Notify(n Notification) error {
	f.calls = append(f.calls, n)
	return nil
}

type errNotifier struct{}

func (errNotifier) Notify(Notification) error { return errors.New("boom") }

// TestClassifyEdge pins the exact transitions that notify: entering needs-attention and
// working -> idle, and nothing else (including re-firing while already in a state).
func TestClassifyEdge(t *testing.T) {
	cases := []struct {
		old, cur Status
		want     edge
	}{
		{StatusIdle, StatusNeedsAttention, edgeNeedsAttention},
		{StatusWorking, StatusNeedsAttention, edgeNeedsAttention},
		{StatusUnknown, StatusNeedsAttention, edgeNeedsAttention},
		{StatusNeedsAttention, StatusNeedsAttention, edgeNone}, // re-fire while blocked
		{StatusWorking, StatusIdle, edgeFinished},
		{StatusIdle, StatusIdle, edgeNone},       // idle nudge while already idle
		{StatusUnknown, StatusWorking, edgeNone}, // start of work
		{StatusIdle, StatusWorking, edgeNone},
		{StatusNeedsAttention, StatusIdle, edgeNone}, // answered, not a fresh finish
	}
	for _, c := range cases {
		if got := classifyEdge(c.old, c.cur); got != c.want {
			t.Errorf("classifyEdge(%s,%s) = %v, want %v", c.old, c.cur, got, c.want)
		}
	}
}

// stateDir returns the resolved state dir for a test whose XDG_STATE_HOME is dir.
func stateDir(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(dir, "birdseye", "agents")
}

func onPolicy(f Notifier) notifyPolicy {
	return notifyPolicy{needsAttention: true, finished: true, notifier: f}
}

// TestHandleHookNotifiesOnEdges drives a realistic event sequence and asserts one notification
// per qualifying edge, with no notification on start-of-work or a re-fired block.
func TestHandleHookNotifiesOnEdges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "")
	stubNoTmuxRecovery(t)
	f := &fakeNotifier{}
	p := onPolicy(f)

	// Start of work: no notification.
	if err := handleHook("UserPromptSubmit", strings.NewReader(`{"session_id":"s1","cwd":"/r/foo"}`), p); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("start-of-work should not notify, got %d", len(f.calls))
	}
	// Finish the turn: working -> idle.
	if err := handleHook("Stop", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatal(err)
	}
	// Block on the user: idle -> needs-attention.
	if err := handleHook("Notification", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatal(err)
	}
	// Re-fired block: no new notification.
	if err := handleHook("Notification", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatal(err)
	}

	if len(f.calls) != 2 {
		t.Fatalf("expected 2 notifications, got %d: %+v", len(f.calls), f.calls)
	}
	if f.calls[0].Status != StatusIdle || !strings.Contains(f.calls[0].Message, "finished") {
		t.Errorf("first notification should be the finish, got %+v", f.calls[0])
	}
	if f.calls[1].Status != StatusNeedsAttention || !strings.Contains(f.calls[1].Message, "needs you") {
		t.Errorf("second notification should be needs-attention, got %+v", f.calls[1])
	}
}

// TestHandleHookMuteSuppresses pins that a muted location emits no notifications.
func TestHandleHookMuteSuppresses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "")
	stubNoTmuxRecovery(t)
	f := &fakeNotifier{}
	p := onPolicy(f)

	if err := handleHook("UserPromptSubmit", strings.NewReader(`{"session_id":"s1","cwd":"/r/foo"}`), p); err != nil {
		t.Fatal(err)
	}
	// With no tmux, the location key falls back to the session id.
	if err := writeMutes(stateDir(t, dir), map[string]bool{"id:s1": true}); err != nil {
		t.Fatal(err)
	}
	if err := handleHook("Stop", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("muted agent should not notify, got %d", len(f.calls))
	}
}

// TestHandleHookConfigGating pins that each edge's config flag suppresses only that edge.
func TestHandleHookConfigGating(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "")
	stubNoTmuxRecovery(t)
	f := &fakeNotifier{}
	// finished off, needs-attention on.
	p := notifyPolicy{needsAttention: true, finished: false, notifier: f}

	if err := handleHook("UserPromptSubmit", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatal(err)
	}
	if err := handleHook("Stop", strings.NewReader(`{"session_id":"s1"}`), p); err != nil { // finish, gated off
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("finished gated off should not notify, got %d", len(f.calls))
	}
	if err := handleHook("Notification", strings.NewReader(`{"session_id":"s1"}`), p); err != nil { // block, on
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("needs-attention should still notify, got %d", len(f.calls))
	}
}

// TestHandleHookNotifyIsBestEffort pins that a failing notifier neither fails the hook nor loses
// the status write.
func TestHandleHookNotifyIsBestEffort(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "")
	stubNoTmuxRecovery(t)
	p := onPolicy(errNotifier{})

	if err := handleHook("UserPromptSubmit", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatal(err)
	}
	if err := handleHook("Stop", strings.NewReader(`{"session_id":"s1"}`), p); err != nil {
		t.Fatalf("a notifier error must not fail the hook: %v", err)
	}
	rec, err := readRecord(stateDir(t, dir), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusIdle {
		t.Fatalf("status should still be written despite notifier error, got %q", rec.Status)
	}
}

// TestNotifierForSelection pins the auto vs command choice.
func TestNotifierForSelection(t *testing.T) {
	if _, ok := notifierFor("").(autoNotifier); !ok {
		t.Error("empty command should select the auto notifier")
	}
	if _, ok := notifierFor("notify-send x").(commandNotifier); !ok {
		t.Error("a set command should select the command notifier")
	}
}

// TestCommandNotifierEnv pins the environment-variable contract passed to a user command.
func TestCommandNotifierEnv(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env.txt")
	c := commandNotifier{command: `printf '%s|%s|%s|%s|%s' "$BE_AGENT" "$BE_STATUS" "$BE_REPO" "$BE_CWD" "$BE_MESSAGE" > ` + out}
	err := c.Notify(Notification{
		Agent:   "foo",
		Status:  StatusNeedsAttention,
		Repo:    "myrepo",
		CWD:     "/r/foo",
		Message: "foo needs you",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "foo|needs-attention|myrepo|/r/foo|foo needs you"
	if string(data) != want {
		t.Errorf("env contract mismatch:\n got %q\nwant %q", string(data), want)
	}
}
