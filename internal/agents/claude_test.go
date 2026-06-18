package agents

import (
	"strings"
	"testing"
	"time"
)

func TestStatusForEvent(t *testing.T) {
	cases := map[string]Status{
		"Notification":     StatusNeedsAttention,
		"Stop":             StatusIdle,
		"SessionEnd":       StatusDone,
		"PreToolUse":       StatusWorking,
		"UserPromptSubmit": StatusWorking,
		"Whatever":         StatusWorking,
	}
	for ev, want := range cases {
		if got := StatusForEvent(ev); got != want {
			t.Errorf("StatusForEvent(%q) = %q, want %q", ev, got, want)
		}
	}
}

func TestAgentsMarksStaleAsUnknown(t *testing.T) {
	dir := t.TempDir()
	fresh := record{SessionID: "a", Title: "fresh", Status: StatusWorking, Updated: time.Now()}
	stale := record{SessionID: "b", Title: "stale", Status: StatusWorking, Updated: time.Now().Add(-time.Hour)}
	done := record{SessionID: "c", Title: "done", Status: StatusDone, Updated: time.Now().Add(-time.Hour)}
	for _, r := range []record{fresh, stale, done} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	s := &ClaudeSource{dir: dir, staleAfter: 5 * time.Minute, now: time.Now}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]Status{}
	for _, a := range got {
		byTitle[a.Title] = a.Status
	}
	if byTitle["fresh"] != StatusWorking {
		t.Errorf("fresh should stay working, got %q", byTitle["fresh"])
	}
	if byTitle["stale"] != StatusUnknown {
		t.Errorf("stale should become unknown, got %q", byTitle["stale"])
	}
	if byTitle["done"] != StatusDone {
		t.Errorf("done should stay done despite age, got %q", byTitle["done"])
	}
}

func TestAgentsSortsNeedsAttentionFirst(t *testing.T) {
	dir := t.TempDir()
	for _, r := range []record{
		{SessionID: "1", Title: "idle-one", Status: StatusIdle, Updated: time.Now()},
		{SessionID: "2", Title: "attn-one", Status: StatusNeedsAttention, Updated: time.Now()},
		{SessionID: "3", Title: "work-one", Status: StatusWorking, Updated: time.Now()},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	s := &ClaudeSource{dir: dir, staleAfter: time.Hour, now: time.Now}
	got, _ := s.Agents()
	if len(got) == 0 || got[0].Status != StatusNeedsAttention {
		t.Fatalf("needs-attention should sort first, got %+v", got)
	}
}

func TestHandleHookWritesState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "") // not in tmux for the test

	payload := `{"session_id":"sess-123","cwd":"/home/u/projects/foo"}`
	if err := HandleHook("Notification", strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}

	s, err := NewClaudeSource(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(got))
	}
	if got[0].Status != StatusNeedsAttention {
		t.Errorf("expected needs-attention from Notification, got %q", got[0].Status)
	}
	if got[0].Title != "foo" { // no tmux session, falls back to cwd base
		t.Errorf("expected title 'foo', got %q", got[0].Title)
	}
}
