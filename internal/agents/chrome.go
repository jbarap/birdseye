package agents

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Claude Code's status line names the background work a session is running right now ("· 1
// monitor", "· ← 2 agents"), and its agent panel lists each running agent on its own line. That is
// the only level-triggered account of this work that exists: hooks state a claim at a turn end,
// but nothing fires when a monitor or a backgrounded shell ends, so a claim can never be retired
// from the hook stream alone. Reading the screen closes that gap the same way pane stillness
// closes the stale-"working" gap - as disproof, never as assertion.
//
// It is a scrape of an undocumented UI, so every uncertainty resolves to "no signal" and leaves
// the claim standing. Should the chrome be reworded, this file is the one place to update, and
// until it is, behavior degrades to the hook-only claim rather than to a wrong answer.

// chromeMode are the glyphs that begin Claude's permission-mode status line, which is the line
// carrying the background-work segments. Matching the mode glyph rather than any wording keeps
// this off the copy that changes most often.
const chromeMode = "⏵⏸"

// chromeAgent prefixes each running agent in the expanded agent panel below the status line. The
// panel's own session line uses a filled circle instead, so only this glyph counts as work.
const chromeAgent = '◯'

// chromeWorkKinds are the nouns Claude counts background work with. A segment is only read as work
// when it is exactly a number and one of these, so "PR #72" and free text can never be miscounted.
var chromeWorkKinds = map[string]bool{
	"monitor": true, "monitors": true,
	"shell": true, "shells": true,
	"task": true, "tasks": true,
	"agent": true, "agents": true,
}

// chromeBackground reports whether a captured screen shows background work running. ok is false
// whenever the screen does not settle the question - no status line found, or a line that may have
// been truncated - and an unsettled screen must leave the claim untouched.
//
// running is true on any hint of work at all, including a count this cannot attribute to a kind,
// because the expensive error is hiding work that is live. Only a status line that is legible,
// complete, and names nothing yields running=false.
func chromeBackground(screen string) (running, ok bool) {
	lines := strings.Split(screen, "\n")
	status, idx := chromeStatusLine(lines)
	if idx < 0 {
		return false, false
	}
	// A line filling the pane may have lost its trailing segments to truncation, and a dropped
	// "· 1 monitor" would read as no work at all. Width is unknown here, so the widest line
	// observed stands in for it: a status line that long is not trusted to be complete.
	if utf8.RuneCountInString(status) >= chromeWidth(lines) {
		return false, false
	}
	if chromeCounts(status) {
		return true, true
	}
	// An expanded panel lists the running agents under the status line instead of counting them
	// on it, so the absence of a count is not yet an absence of work.
	for _, l := range lines[idx+1:] {
		if r, _ := utf8.DecodeRuneInString(strings.TrimLeft(l, " ")); r == chromeAgent {
			return true, true
		}
	}
	return false, true
}

// chromeStatusLine returns the last permission-mode line on the screen and its index. The last is
// the live one: earlier matches are scrollback of previous frames.
func chromeStatusLine(lines []string) (string, int) {
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimLeft(lines[i], " ")
		if r, _ := utf8.DecodeRuneInString(t); strings.ContainsRune(chromeMode, r) {
			return t, i
		}
	}
	return "", -1
}

// chromeWidth is the widest line on the screen, standing in for the pane width that capture does
// not report. Trailing blanks are trimmed by capture, so this is a lower bound - which is the safe
// direction: underestimating the width only declines to read a status line that might be fine.
func chromeWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		if n := utf8.RuneCountInString(l); n > w {
			w = n
		}
	}
	return w
}

// chromeCounts reports whether the status line carries a segment counting background work. The
// leading arrow of the collapsed agent hint ("← 2 agents") is dropped before matching; the
// countless form of that hint ("← for agents") names no work and is ignored, since it appears
// whether or not anything is running.
func chromeCounts(status string) bool {
	for _, seg := range strings.Split(status, "·") {
		fields := strings.Fields(strings.TrimLeft(strings.TrimSpace(seg), "←⟵ "))
		if len(fields) != 2 || !chromeWorkKinds[strings.ToLower(fields[1])] {
			continue
		}
		if n, err := strconv.Atoi(fields[0]); err == nil && n > 0 {
			return true
		}
	}
	return false
}
