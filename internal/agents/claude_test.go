package agents

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubNoTmuxRecovery neutralizes the owning-process tmux recovery so a test that sets
// TMUX="" to simulate "not in tmux" is hermetic: without it, resolveTmux would read the real
// /proc environment of whatever process runs the suite and pick up a live pane when the tests
// are run from inside tmux.
func stubNoTmuxRecovery(t *testing.T) {
	t.Helper()
	prev := procTmux
	procTmux = func(int) (string, string) { return "", "" }
	t.Cleanup(func() { procTmux = prev })
}

// TestParseTmuxEnv pins recovery of a Claude daemon's tmux location from its NUL-separated
// environ: the pane and server socket are extracted, and an environ without tmux yields empties.
func TestParseTmuxEnv(t *testing.T) {
	env := []byte("PATH=/usr/bin\x00TMUX=/tmp/tmux-1000/default,11488,10\x00TMUX_PANE=%46\x00HOME=/home/u\x00")
	tmux, pane := parseTmuxEnv(env)
	if tmux != "/tmp/tmux-1000/default,11488,10" {
		t.Errorf("tmux = %q", tmux)
	}
	if pane != "%46" {
		t.Errorf("pane = %q", pane)
	}
	if tmux, pane := parseTmuxEnv([]byte("PATH=/usr/bin\x00HOME=/home/u\x00")); tmux != "" || pane != "" {
		t.Errorf("a tmux-less environ should yield empties, got %q %q", tmux, pane)
	}
}

func TestStatusForEventName(t *testing.T) {
	// Events whose status comes from the name alone (empty payload).
	cases := map[string]Status{
		"Notification":     StatusNeedsAttention,
		"Stop":             StatusIdle,
		"PreToolUse":       StatusWorking,
		"UserPromptSubmit": StatusWorking,
		"Whatever":         StatusWorking,
	}
	for ev, want := range cases {
		if got := StatusFor(ev, hookInput{}); got != want {
			t.Errorf("StatusFor(%q, {}) = %q, want %q", ev, got, want)
		}
	}
}

// TestStatusForNotificationMessage pins the payload-aware split: only Claude's
// idle nudge downgrades to idle; permission prompts and anything unrecognized
// stay needs-attention. The idle string here mirrors Claude's live copy — if
// that wording changes, this is the test that catches it.
func TestStatusForNotificationMessage(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    Status
	}{
		{"idle nudge", "Claude is waiting for your input", StatusIdle},
		{"idle nudge cased", "WAITING FOR YOUR INPUT", StatusIdle},
		{"permission prompt", "Claude needs your permission to use Bash", StatusNeedsAttention},
		{"unrecognized", "Something brand new", StatusNeedsAttention},
		{"empty", "", StatusNeedsAttention},
	}
	for _, c := range cases {
		got := StatusFor("Notification", hookInput{Message: c.message})
		if got != c.want {
			t.Errorf("%s: StatusFor(Notification, %q) = %q, want %q", c.name, c.message, got, c.want)
		}
	}
	// The message only matters for Notification — a Stop with the idle phrase is
	// still idle by the event name, and a non-Notification event ignores it.
	if got := StatusFor("UserPromptSubmit", hookInput{Message: "waiting for your input"}); got != StatusWorking {
		t.Errorf("message should not affect non-Notification events, got %q", got)
	}
}

// alwaysAlive is a liveness stub that keeps every record (treats all pids live).
func alwaysAlive(int) bool { return true }

// TestClaudeTitleLevel pins the title glyph -> level map: any Braille frame is working
// (the spinner animates across the class), the sparkle is idle, and anything else yields
// no signal so the caller falls back to the hook status.
func TestClaudeTitleLevel(t *testing.T) {
	cases := []struct {
		title     string
		want      Status
		haveLevel bool
	}{
		{"⠂ Working on the thing", StatusWorking, true},  // U+2802
		{"⠐ another spinner frame", StatusWorking, true}, // U+2810, a different frame
		{"⣿ full braille", StatusWorking, true},          // U+28FF, top of range
		{"✳ Idle at the prompt", StatusIdle, true},       // U+2733
		{"  ✳ leading space trimmed", StatusIdle, true},
		{"grotto", "", false}, // a plain shell title
		{"", "", false},
		{"~/projects/birdseye", "", false},
	}
	for _, c := range cases {
		got, ok := claudeTitleLevel(c.title)
		if got != c.want || ok != c.haveLevel {
			t.Errorf("claudeTitleLevel(%q) = (%q,%v), want (%q,%v)", c.title, got, ok, c.want, c.haveLevel)
		}
	}
}

// TestReconcilePrecedence walks the status precedence table: needs-attention is a latch
// cleared only by a working level, and otherwise the live level overrides working/idle with
// a fallback to the hook status when there is no signal.
func TestReconcilePrecedence(t *testing.T) {
	cases := []struct {
		name      string
		hook      Status
		level     Status
		haveLevel bool
		want      Status
	}{
		{"attention cleared by working level", StatusNeedsAttention, StatusWorking, true, StatusWorking},
		{"attention held by idle level", StatusNeedsAttention, StatusIdle, true, StatusNeedsAttention},
		{"attention held with no level", StatusNeedsAttention, "", false, StatusNeedsAttention},
		{"stale working corrected to idle", StatusWorking, StatusIdle, true, StatusIdle},
		{"working stays working", StatusWorking, StatusWorking, true, StatusWorking},
		{"idle promoted by working level", StatusIdle, StatusWorking, true, StatusWorking},
		{"working with no level falls back", StatusWorking, "", false, StatusWorking},
		{"unknown resolved by level", StatusUnknown, StatusWorking, true, StatusWorking},
		{"unknown with no level stays unknown", StatusUnknown, "", false, StatusUnknown},
	}
	for _, c := range cases {
		if got := reconcile(c.hook, c.level, c.haveLevel); got != c.want {
			t.Errorf("%s: reconcile(%q,%q,%v) = %q, want %q", c.name, c.hook, c.level, c.haveLevel, got, c.want)
		}
	}
}

// TestAgentsCorrectsLevelFromTitle exercises the end-to-end read path: a stale hook status
// is corrected by the live pane title, the attention latch is held or cleared by the
// title, a pane with no title keeps its hook status, and the title source is read once per
// refresh (batched), not once per agent.
func TestAgentsCorrectsLevelFromTitle(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, r := range []record{
		{SessionID: "stale-work", PID: 1, TmuxSession: "s", TmuxPane: "%1", Title: "a", Status: StatusWorking, Updated: now},
		{SessionID: "still-work", PID: 1, TmuxSession: "s", TmuxPane: "%2", Title: "b", Status: StatusWorking, Updated: now},
		{SessionID: "attn-moved", PID: 1, TmuxSession: "s", TmuxPane: "%3", Title: "c", Status: StatusNeedsAttention, Updated: now},
		{SessionID: "attn-held", PID: 1, TmuxSession: "s", TmuxPane: "%4", Title: "d", Status: StatusNeedsAttention, Updated: now},
		{SessionID: "no-title", PID: 1, TmuxSession: "s", TmuxPane: "%9", Title: "e", Status: StatusWorking, Updated: now},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	calls := 0
	titles := map[string]string{
		"%1": "✳ done now",        // working record, idle title -> idle
		"%2": "⠂ still going",     // working record, spinner title -> working
		"%3": "⠂ resumed",         // attention record, spinner title -> working (latch cleared)
		"%4": "✳ awaiting answer", // attention record, idle title -> needs-attention (held)
		// %9 has no entry -> no signal -> keep hook status
	}
	s := &ClaudeSource{
		dir:      dir,
		recDir:   dir,
		alive:    alwaysAlive,
		detector: titleLevelDetector{},
		titles:   func() (map[string]string, error) { calls++; return titles, nil },
	}

	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("title source should be read once per refresh, got %d calls", calls)
	}
	byID := map[string]Status{}
	for _, a := range got {
		byID[a.SessionID] = a.Status
	}
	want := map[string]Status{
		"stale-work": StatusIdle,
		"still-work": StatusWorking,
		"attn-moved": StatusWorking,
		"attn-held":  StatusNeedsAttention,
		"no-title":   StatusWorking,
	}
	for id, w := range want {
		if byID[id] != w {
			t.Errorf("%s: status = %q, want %q", id, byID[id], w)
		}
	}
}

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
		{SessionID: "legacy", PID: 0, Title: "legacy", Status: StatusIdle, Updated: now},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	s := &ClaudeSource{dir: dir, recDir: dir, alive: func(pid int) bool { return pid == 100 }}
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

// TestMutePersistsSnoozeAndReclaims pins the mute store's three rules: a mute persists and
// stamps the agent (sorting it last), a needs-attention agent is force-unmuted with its key
// pruned (the snooze "wake me on something new" rule), and a dead location's mute is
// reclaimed by the same liveness GC that drops the agent.
func TestMutePersistsSnoozeAndReclaims(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, r := range []record{
		{SessionID: "work", PID: 1, TmuxSession: "s", TmuxPane: "%1", Title: "work", Status: StatusWorking, Updated: now},
		{SessionID: "block", PID: 1, TmuxSession: "s", TmuxPane: "%2", Title: "block", Status: StatusNeedsAttention, Updated: now},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	s := &ClaudeSource{dir: dir, recDir: dir, alive: alwaysAlive}

	workKey := locationKey("%1", "s", "", "work")
	if err := s.SetMuted(workKey, true); err != nil {
		t.Fatal(err)
	}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Agent{}
	for _, a := range got {
		byID[a.SessionID] = a
	}
	if !byID["work"].Muted {
		t.Fatalf("the working agent should be muted after SetMuted")
	}
	if got[len(got)-1].SessionID != "work" {
		t.Fatalf("a muted agent should sort last, got order %+v", got)
	}

	// Snooze: muting a needs-attention agent is overridden — the block wins and its key is
	// pruned from the store, while the working agent's mute persists.
	blockKey := locationKey("%2", "s", "", "block")
	if err := s.SetMuted(blockKey, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Agents(); err != nil {
		t.Fatal(err)
	}
	mutes, _ := readMutes(dir)
	if mutes[blockKey] {
		t.Fatalf("auto-unmute should prune the needs-attention key, store=%v", mutes)
	}
	if !mutes[workKey] {
		t.Fatalf("the working agent's mute should persist, store=%v", mutes)
	}

	// Reclamation: the working agent's process dies → its record and mute are both reclaimed.
	s.alive = func(int) bool { return false }
	if _, err := s.Agents(); err != nil {
		t.Fatal(err)
	}
	if mutes, _ := readMutes(dir); len(mutes) != 0 {
		t.Fatalf("a dead location's mute should be reclaimed, store=%v", mutes)
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
	s := &ClaudeSource{dir: dir, recDir: dir, alive: alwaysAlive}
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
		{SessionID: "old", TmuxSession: "arewa", TmuxWindow: "2", TmuxPane: "%5", Title: "arewa", Status: StatusIdle, Updated: now.Add(-time.Hour)},
		{SessionID: "new", TmuxSession: "arewa", TmuxWindow: "2", TmuxPane: "%5", Title: "arewa", Status: StatusWorking, Updated: now},
		// A pre-upgrade record (no pane) in the same window is superseded.
		{SessionID: "legacy", TmuxSession: "arewa", TmuxWindow: "2", Title: "arewa", Status: StatusIdle, Updated: now.Add(-2 * time.Hour)},
		// A genuinely separate pane in the same window survives.
		{SessionID: "other", TmuxSession: "arewa", TmuxWindow: "2", TmuxPane: "%6", Title: "arewa", Status: StatusIdle, Updated: now},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}

	s := &ClaudeSource{dir: dir, recDir: dir, alive: alwaysAlive}
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
	stubNoTmuxRecovery(t)

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
	// The working directory must survive on the record, not only feed the title:
	// the reconciler resolves it to a repository for recognition.
	if got[0].CWD != "/home/u/projects/foo" {
		t.Errorf("expected agent cwd persisted, got %q", got[0].CWD)
	}
}

// TestHandleHookPersistsCWD pins that the hook's cwd survives onto the stored
// record across events and is exposed as the agent's working directory, and that
// an event without a cwd leaves a previously recorded one intact.
func TestHandleHookPersistsCWD(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("TMUX", "")
	stubNoTmuxRecovery(t)

	if err := HandleHook("SessionStart", strings.NewReader(`{"session_id":"s1","cwd":"/repo/wt"}`)); err != nil {
		t.Fatal(err)
	}
	// A later event with no cwd (an unexpected partial payload) must not erase it.
	if err := HandleHook("Stop", strings.NewReader(`{"session_id":"s1"}`)); err != nil {
		t.Fatal(err)
	}

	rec, err := readRecord(filepath.Join(dir, "birdseye", "agents"), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.CWD != "/repo/wt" {
		t.Fatalf("record should retain cwd across events, got %q", rec.CWD)
	}

	// The Row built from the agent carries the working directory for the contract.
	row := agentRow(Agent{SessionID: "s1", CWD: "/repo/wt"})
	if row.AgentDir != "/repo/wt" {
		t.Errorf("row should carry agent working directory, got %q", row.AgentDir)
	}
}

// TestMigrateLegacyMutes moves a mutes.json written under the old agents/ subdir up to the state
// root, removes the stale copy, and never clobbers an existing root file with a legacy one.
func TestMigrateLegacyMutes(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "agents")
	if err := writeMutes(legacy, map[string]bool{"id:s1": true}); err != nil {
		t.Fatal(err)
	}

	migrateLegacyMutes(root)
	if got, err := readMutes(root); err != nil || !got["id:s1"] {
		t.Fatalf("mute should be migrated to the root, got %v err %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(legacy, muteFile)); !os.IsNotExist(err) {
		t.Fatalf("legacy mutes.json should be removed after migration")
	}

	// A stale legacy file must not overwrite the migrated root file.
	if err := writeMutes(legacy, map[string]bool{"id:s2": true}); err != nil {
		t.Fatal(err)
	}
	migrateLegacyMutes(root)
	if got, _ := readMutes(root); got["id:s2"] || !got["id:s1"] {
		t.Fatalf("existing root mutes must win over a stale legacy file, got %v", got)
	}
}
