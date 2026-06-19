// Package agents provides the Agent abstraction behind `be agents`, plus a
// Claude Code implementation whose status comes from Claude Code hooks rather
// than from scraping panes.
package agents

import "time"

// Status is an agent's coarse state, drawn from a small, well-defined set.
type Status string

const (
	// StatusNeedsAttention: the agent is blocked on the user (permission or
	// input). Surfaced first.
	StatusNeedsAttention Status = "needs-attention"
	// StatusWorking: the agent is actively running.
	StatusWorking Status = "working"
	// StatusIdle: the agent finished its turn and is waiting.
	StatusIdle Status = "idle"
	// StatusDone: the agent's session ended.
	StatusDone Status = "done"
	// StatusUnknown: no fresh state is available (hook missing or stale).
	StatusUnknown Status = "unknown"
)

// rank orders statuses for display: needs-attention first.
func (s Status) rank() int {
	switch s {
	case StatusNeedsAttention:
		return 0
	case StatusWorking:
		return 1
	case StatusIdle:
		return 2
	case StatusDone:
		return 3
	default:
		return 4
	}
}

// Agent is one tracked agent session.
type Agent struct {
	// SessionID is the agent's own identifier (e.g. Claude session id).
	SessionID string
	// TmuxSession and TmuxWindow locate the agent for jump-to-session.
	// TmuxWindow is the window index (stable for targeting); TmuxWindowName is
	// the window's human title (e.g. "nvim"), used for display only.
	TmuxSession    string
	TmuxWindow     string
	TmuxWindowName string
	// TmuxPane is the exact pane the agent runs in (e.g. "%5"). Unlike
	// session:window it pins the right pane when a window is split, so the
	// preview captures the agent's pane rather than whichever is active.
	TmuxPane string
	// Title is a short human label.
	Title string
	// Status is the agent's current state.
	Status Status
	// Updated is when the status was last written.
	Updated time.Time
}

// Source yields the agents it knows about. New agent types implement this
// without changing the `be agents` command.
type Source interface {
	Agents() ([]Agent, error)
}
