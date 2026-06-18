package agents

import (
	"os"
	"path/filepath"
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

func TestAgentsDedupsSharedLocation(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, r := range []record{
		// Two Claude sessions that ran in the same pane (a restart): keep newest.
		{SessionID: "old", TmuxSession: "arewa", TmuxWindow: "2", TmuxPane: "%5", Title: "arewa", Status: StatusDone, Updated: now.Add(-time.Hour)},
		{SessionID: "new", TmuxSession: "arewa", TmuxWindow: "2", TmuxPane: "%5", Title: "arewa", Status: StatusWorking, Updated: now},
		// A pre-upgrade record (no pane) in the same window is superseded.
		{SessionID: "legacy", TmuxSession: "arewa", TmuxWindow: "2", Title: "arewa", Status: StatusDone, Updated: now.Add(-2 * time.Hour)},
		// A genuinely separate pane in the same window survives.
		{SessionID: "other", TmuxSession: "arewa", TmuxWindow: "2", TmuxPane: "%6", Title: "arewa", Status: StatusIdle, Updated: now},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	s := &ClaudeSource{dir: dir, staleAfter: 0, now: time.Now}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 agents (one per pane), got %d: %+v", len(got), got)
	}
	byPane := map[string]Agent{}
	for _, a := range got {
		byPane[a.TmuxPane] = a
	}
	if byPane["%5"].SessionID != "new" || byPane["%5"].Status != StatusWorking {
		t.Fatalf("pane %%5 should keep the newest record, got %+v", byPane["%5"])
	}
	if _, ok := byPane["%6"]; !ok {
		t.Fatalf("separate pane %%6 should survive dedup")
	}
}

func TestAgentsPrunesExpiredRecords(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	recs := map[string]record{
		"keep-done":   {SessionID: "keep-done", Title: "kd", Status: StatusDone, Updated: now.Add(-time.Hour)},
		"old-done":    {SessionID: "old-done", Title: "od", Status: StatusDone, Updated: now.Add(-48 * time.Hour)},
		"crashed":     {SessionID: "crashed", Title: "cr", Status: StatusWorking, Updated: now.Add(-48 * time.Hour)},
		"fresh-stale": {SessionID: "fresh-stale", Title: "fs", Status: StatusWorking, Updated: now.Add(-time.Hour)},
	}
	for _, r := range recs {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	s := &ClaudeSource{
		dir:         dir,
		staleAfter:  5 * time.Minute,
		forgetDone:  24 * time.Hour,
		forgetStale: 24 * time.Hour,
		now:         func() time.Time { return now },
	}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}

	live := map[string]bool{}
	for _, a := range got {
		live[a.SessionID] = true
	}
	if !live["keep-done"] || !live["fresh-stale"] {
		t.Fatalf("recent records should remain, got %v", live)
	}
	if live["old-done"] || live["crashed"] {
		t.Fatalf("aged-out records should be gone from the list, got %v", live)
	}
	// The state files must actually be deleted, not just hidden.
	for _, id := range []string{"old-done", "crashed"} {
		if _, err := os.Stat(filepath.Join(dir, sanitize(id)+".json")); !os.IsNotExist(err) {
			t.Fatalf("expected %s state file to be pruned, stat err=%v", id, err)
		}
	}
	for _, id := range []string{"keep-done", "fresh-stale"} {
		if _, err := os.Stat(filepath.Join(dir, sanitize(id)+".json")); err != nil {
			t.Fatalf("expected %s state file to remain: %v", id, err)
		}
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

	s, err := NewClaudeSource(time.Hour, 0, 0)
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
