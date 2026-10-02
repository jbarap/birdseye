package agents

import (
	"strings"
	"testing"
	"time"
)

// screenOf builds a capture whose last lines are the given chrome, padded with a content line wide
// enough that the status line is nowhere near the pane width. Real captures carry long transcript
// lines above the chrome; without one, every status line would look like it might be truncated.
func screenOf(chrome ...string) string {
	lines := []string{strings.Repeat("x", 200)}
	return strings.Join(append(lines, chrome...), "\n")
}

// TestChromeBackgroundReadsRealStatusLines pins the parser against chrome captured verbatim from
// live Claude panes. These strings are the contract: if Claude rewords them, this test fails and
// chrome.go is the one file to update - which is the point of keeping real samples rather than
// invented ones.
func TestChromeBackgroundReadsRealStatusLines(t *testing.T) {
	cases := []struct {
		name    string
		chrome  []string
		running bool
		ok      bool
	}{
		{
			// A session with a live monitor counts it on the status line.
			name:    "counted monitor",
			chrome:  []string{"  ⏵⏵ auto mode on · PR #72 · 1 monitor · ← for agents"},
			running: true, ok: true,
		},
		{
			// "← for agents" is a bare hint carrying no count, and appears whether or not
			// anything runs. Reading it as work would pin a claim forever.
			name:    "agents hint without a count",
			chrome:  []string{"  ⏵⏵ auto mode on (shift+tab to cycle) · PR #62 · ← for agents"},
			running: false, ok: true,
		},
		{
			// The collapsed agent count does name work.
			name:    "collapsed agent count",
			chrome:  []string{"  ⏵⏵ auto mode on (shift+tab to cycle) · ← 1 agent"},
			running: true, ok: true,
		},
		{
			// The expanded panel lists agents instead of counting them, so the status line
			// alone shows nothing. Missing this retires a claim whose subagent is still live.
			name: "expanded agent panel",
			chrome: []string{
				"  ⏵⏵ auto mode on (shift+tab to cycle) · PR #14 · ← for agents",
				"  ● main",
				"  ◯ general-purpose  Reading import_job in admin.py        4m 42s · ↓ 150.1k tokens",
			},
			running: true, ok: true,
		},
		{
			// The plain idle session: this is what disproves a stale claim.
			name:    "no work named",
			chrome:  []string{"  ⏵⏵ auto mode on (shift+tab to cycle)"},
			running: false, ok: true,
		},
		{
			// A count of zero is not work.
			name:    "zero count",
			chrome:  []string{"  ⏵⏵ auto mode on · 0 monitors · ← for agents"},
			running: false, ok: true,
		},
		{
			// "PR #72" is a number beside a word, and must never read as a count of work.
			name:    "pr number is not a count",
			chrome:  []string{"  ⏵⏵ auto mode on · PR #72"},
			running: false, ok: true,
		},
	}
	for _, c := range cases {
		running, ok := chromeBackground(screenOf(c.chrome...))
		if running != c.running || ok != c.ok {
			t.Errorf("%s: got running=%v ok=%v, want running=%v ok=%v", c.name, running, ok, c.running, c.ok)
		}
	}
}

// TestChromeBackgroundDeclinesWhenUnsure pins the failure direction. Every case the parser cannot
// settle must yield ok=false, which leaves the claim exactly as the hooks stated it. A wrong
// "nothing is running" hides live work, which is the bug this whole mechanism exists to prevent.
func TestChromeBackgroundDeclinesWhenUnsure(t *testing.T) {
	cases := []struct {
		name   string
		screen string
	}{
		{"empty capture", ""},
		{"no chrome at all", "just a shell prompt\n$ "},
		{
			// A status line filling the pane may have lost its trailing segments, and a dropped
			// "· 1 monitor" would read as no work at all. Capture does not report the pane width,
			// so the widest line stands in for it: a status line that reaches it is not trusted.
			// A screen holding nothing but chrome therefore yields no signal at all, which is the
			// safe direction - it leaves the claim exactly as the hooks stated it.
			name:   "possibly truncated",
			screen: "short\n⏵⏵ auto mode on · PR #72 · 1 monit",
		},
	}
	for _, c := range cases {
		if running, ok := chromeBackground(c.screen); ok {
			t.Errorf("%s: got running=%v ok=true, want ok=false so the claim stands", c.name, running)
		}
	}
}

// TestChromeStatusLineTakesTheLiveFrame pins which status line wins. A capture can hold chrome from
// an earlier frame in its scrollback; the live one is the last.
func TestChromeStatusLineTakesTheLiveFrame(t *testing.T) {
	screen := screenOf(
		"  ⏵⏵ auto mode on · 2 monitors · ← for agents",
		"  some later output",
		"  ⏵⏵ auto mode on (shift+tab to cycle)",
	)
	if running, ok := chromeBackground(screen); !ok || running {
		t.Errorf("the last status line is the live one: got running=%v ok=%v, want running=false ok=true", running, ok)
	}
}

// TestEffectiveTasksChromeDisprovesClaim pins how the screen and the claim combine. The chrome may
// only ever remove work, and only when it is legible: it cannot invent work the claim never stated,
// and an unreadable screen must leave the claim untouched at any age.
func TestEffectiveTasksChromeDisprovesClaim(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tasks := []BackgroundTask{{Type: "monitor"}}
	fresh := &claim{At: now.Add(-time.Minute), Tasks: tasks}

	noWork := screenOf("  ⏵⏵ auto mode on (shift+tab to cycle)")
	work := screenOf("  ⏵⏵ auto mode on · 1 monitor · ← for agents")

	// The reported bug: an idle row naming a monitor whose pane says nothing is running.
	if got := effectiveTasks(fresh, StatusIdle, noWork, now); got != nil {
		t.Errorf("chrome naming no work should retire the claim, got %+v", got)
	}
	// A working session is not exempt: a claim is stale the moment its work ends.
	if got := effectiveTasks(fresh, StatusWorking, noWork, now); got != nil {
		t.Errorf("chrome disproves a claim regardless of status, got %+v", got)
	}
	// Chrome confirming work keeps the claim, which is what supplies the names.
	if got := effectiveTasks(fresh, StatusIdle, work, now); len(got) != 1 {
		t.Errorf("chrome naming work should keep the claim, got %+v", got)
	}
	// No screen is no signal: behavior falls back to the claim alone.
	if got := effectiveTasks(fresh, StatusIdle, "", now); len(got) != 1 {
		t.Errorf("an unreadable screen must leave the claim standing, got %+v", got)
	}
	// Chrome never adds: a session with no claim names nothing, however busy its pane looks.
	if got := effectiveTasks(nil, StatusIdle, work, now); got != nil {
		t.Errorf("chrome must not invent work the claim never stated, got %+v", got)
	}
}

// TestAgentsRetiresClaimDisprovedByChrome is the regression guard for the reported bug: an idle row
// naming a monitor that had already finished, held by a claim no event was ever going to replace.
//
// It also pins the sampling that makes the fix reachable at all. Every phantom claim sits on an
// idle row, so back when only working panes were sampled there was no screen to disprove anything;
// the claim-carrying panes must be captured too or this silently degrades to the old behavior.
func TestAgentsRetiresClaimDisprovedByChrome(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	bg := []BackgroundTask{{Type: "monitor"}}
	for _, r := range []record{
		{SessionID: "phantom", PID: 1, TmuxSession: "s", TmuxWindow: "1", TmuxPane: "%1", Title: "a", Status: StatusIdle, Updated: now,
			Background: &claim{At: now.Add(-time.Minute), Tasks: bg}},
		{SessionID: "live", PID: 1, TmuxSession: "s", TmuxWindow: "2", TmuxPane: "%2", Title: "b", Status: StatusIdle, Updated: now,
			Background: &claim{At: now.Add(-time.Minute), Tasks: bg}},
		{SessionID: "unseen", PID: 1, TmuxSession: "s", TmuxWindow: "3", TmuxPane: "%3", Title: "c", Status: StatusIdle, Updated: now,
			Background: &claim{At: now.Add(-time.Minute), Tasks: bg}},
	} {
		if err := writeRecord(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	var asked []string
	s := &ClaudeSource{
		dir:    dir,
		recDir: dir,
		alive:  alwaysAlive,
		dets:   defaultDetectors(),
		now:    func() time.Time { return now },
		screens: func(ids []string) (map[string]string, error) {
			asked = ids
			return map[string]string{
				"%1": screenOf("  ⏵⏵ auto mode on (shift+tab to cycle)"),
				"%2": screenOf("  ⏵⏵ auto mode on · 1 monitor · ← for agents"),
			}, nil
		},
	}
	got, err := s.Agents()
	if err != nil {
		t.Fatal(err)
	}
	work := map[string][]BackgroundTask{}
	for _, a := range got {
		work[a.SessionID] = a.Background
	}
	if len(work["phantom"]) != 0 {
		t.Errorf("a claim its own pane says is not running should be retired, got %+v", work["phantom"])
	}
	if len(work["live"]) != 1 {
		t.Errorf("a claim its pane confirms should stand, got %+v", work["live"])
	}
	if len(work["unseen"]) != 1 {
		t.Errorf("an uncaptured pane is no signal, so the claim should stand, got %+v", work["unseen"])
	}
	if len(asked) != 3 {
		t.Errorf("all three claim-carrying panes must be sampled though none is working, asked %v", asked)
	}
}
