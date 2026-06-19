package agents

import (
	"os"
	"os/exec"
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

// alwaysAlive is a liveness stub that keeps every record (treats all pids live).
func alwaysAlive(int) bool { return true }

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("the current process should report alive")
	}
	if processAlive(0) || processAlive(-1) {
		t.Fatal("a non-positive pid is never alive")
	}
	// A reaped child's pid is no longer running.
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot run 'true' to obtain a dead pid: %v", err)
	}
	if processAlive(cmd.Process.Pid) {
		t.Errorf("reaped pid %d should not be alive", cmd.Process.Pid)
	}
}

func TestWalkToClaudeFindsSessionProcess(t *testing.T) {
	// Synthetic tree: be(10) -> sh(11, ephemeral) -> claude(12) -> zsh(13) -> tmux(14).
	// The hook starts the walk at its parent (the shell) and must land on claude.
	tree := map[int]struct {
		ppid int
		comm string
	}{
		11: {12, "sh"},
		12: {13, "/usr/local/bin/claude"}, // full path: basename still matches
		13: {14, "zsh"},
		14: {1, "tmux: server"},
	}
	parent := func(pid int) (int, string) { return tree[pid].ppid, tree[pid].comm }

	got, ok := walkToClaude(11, parent)
	if !ok || got != 12 {
		t.Fatalf("walk should find claude pid 12, got pid=%d ok=%v", got, ok)
	}

	// No Claude in the chain: report not-found so the caller can fall back.
	plain := map[int]struct {
		ppid int
		comm string
	}{11: {13, "sh"}, 13: {1, "zsh"}}
	if _, ok := walkToClaude(11, func(p int) (int, string) { return plain[p].ppid, plain[p].comm }); ok {
		t.Fatal("walk should not find a claude ancestor when none exists")
	}
}

func TestAgentsReclaimsDeadProcesses(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, r := range []record{
		{SessionID: "live", PID: 100, TmuxSession: "s", TmuxPane: "%1", Title: "live", Status: StatusWorking, Updated: now},
		// Pane still open (a leftover shell), but the Claude process is gone.
		{SessionID: "dead", PID: 200, TmuxSession: "s", TmuxPane: "%2", Title: "dead", Status: StatusIdle, Updated: now},
		// Legacy record with no pid: cannot be confirmed live, so reclaimed.
		{SessionID: "legacy", PID: 0, Title: "legacy", Status: StatusDone, Updated: now},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	s := &ClaudeSource{dir: dir, alive: func(pid int) bool { return pid == 100 }}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SessionID != "live" {
		t.Fatalf("only the live-process agent should remain, got %+v", got)
	}
	for _, id := range []string{"dead", "legacy"} {
		if _, err := os.Stat(filepath.Join(dir, sanitize(id)+".json")); !os.IsNotExist(err) {
			t.Fatalf("%s state file should be deleted, stat err=%v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, sanitize("live")+".json")); err != nil {
		t.Fatalf("live record must remain: %v", err)
	}
}

func TestAgentsSortsNeedsAttentionFirst(t *testing.T) {
	dir := t.TempDir()
	for _, r := range []record{
		{SessionID: "1", PID: 1, Title: "idle-one", Status: StatusIdle, Updated: time.Now()},
		{SessionID: "2", PID: 1, Title: "attn-one", Status: StatusNeedsAttention, Updated: time.Now()},
		{SessionID: "3", PID: 1, Title: "work-one", Status: StatusWorking, Updated: time.Now()},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	s := &ClaudeSource{dir: dir, alive: alwaysAlive}
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

	s := &ClaudeSource{dir: dir, alive: alwaysAlive}
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

func TestHandleHookWritesState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "") // not in tmux for the test

	payload := `{"session_id":"sess-123","cwd":"/home/u/projects/foo"}`
	if err := HandleHook("Notification", strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}

	s, err := NewClaudeSource()
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
