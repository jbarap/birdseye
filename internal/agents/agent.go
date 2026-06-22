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

// RowKind distinguishes the leaf rows the agents view renders. A row may carry a
// live agent, mark a managed repo's default-branch anchor, or mark an empty worktree
// slot (a worktree window with no agent — a spawn target).
type RowKind int

const (
	RowAgent  RowKind = iota // a live agent (status from its hook record)
	RowAnchor                // a managed repo's default-branch checkout (no agent)
	RowSlot                  // a managed worktree with no agent (spawn target)
)

// Row is one leaf the agents view renders. It is the view's unit — a tmux location
// plus display — optionally backed by a live agent. Anchor and slot rows carry no
// agent, which is why the view models rows rather than overloading Agent.
type Row struct {
	Kind RowKind
	// SessionID is the agent's id for RowAgent rows (used to follow selection across
	// refreshes and to forget a deleted agent); a stable synthetic id otherwise.
	SessionID string
	// Tmux location of the row, used for grouping, preview, jump, and lifecycle ops.
	TmuxSession    string
	TmuxWindow     string
	TmuxWindowName string
	TmuxPane       string
	// Dir is the row's filesystem directory (the pane's start path / worktree dir),
	// used to add or remove a worktree. Empty when unknown.
	Dir string
	// Worktree is the worktree's name when the row is a managed worktree (RowSlot, or
	// a RowAgent running in a worktree window); empty for incidental agents and rows
	// outside a managed repo. A non-empty value is what makes delete also remove the
	// worktree.
	Worktree string
	// Managed reports whether the owning session is a recognized managed repo, so its
	// section bar shows the indicator and `n` is available.
	Managed bool
	// Title is the row's display name.
	Title string
	// Status is the agent's status for RowAgent rows; anchor/slot rows render their
	// own marker and ignore this.
	Status Status
	// Updated is when the backing state was last written (RowAgent); used for the
	// most-recent dedup and selection stability.
	Updated time.Time
}

// RowSource yields the rows the agents view renders. The plain agents view uses an
// adapter over a Source; orchestration uses the Workspace reconciler.
type RowSource interface {
	Rows() ([]Row, error)
}

// agentsAsRows adapts a plain Source into a RowSource — every agent a RowAgent — for
// the non-orchestration path and for tests.
type agentsAsRows struct{ src Source }

func (a agentsAsRows) Rows() ([]Row, error) {
	ags, err := a.src.Agents()
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(ags))
	for _, ag := range ags {
		rows = append(rows, agentRow(ag))
	}
	return rows, nil
}

// AgentsAsRows wraps a Source so it can drive the view without orchestration.
func AgentsAsRows(src Source) RowSource { return agentsAsRows{src} }

// agentRow builds a RowAgent Row from an Agent.
func agentRow(a Agent) Row {
	return Row{
		Kind:           RowAgent,
		SessionID:      a.SessionID,
		TmuxSession:    a.TmuxSession,
		TmuxWindow:     a.TmuxWindow,
		TmuxWindowName: a.TmuxWindowName,
		TmuxPane:       a.TmuxPane,
		Title:          a.Title,
		Status:         a.Status,
		Updated:        a.Updated,
	}
}
