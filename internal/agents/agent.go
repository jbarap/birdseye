// Package agents provides the Agent abstraction behind `be agents`, plus a Claude
// Code implementation with a layered status model: hooks supply identity, existence
// (process liveness), needs-attention, and done, while the working/idle level is read
// fresh from the agent pane's current OSC title so it reflects the terminal now and
// self-heals when no transition hook fires (an interrupt, a crash, an in-terminal
// answer). The title is a deliberate state channel, not scraped TUI layout.
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
	// CWD is the agent's working directory as reported by its hook. It is the
	// durable signal the reconciler resolves to a repository for recognition and
	// grouping (an agent's worktree does not flicker the way a wandering shell's
	// path would). Empty when the hook reported no directory.
	CWD string
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
// live agent, mark a managed repo's primary-worktree anchor, or mark an empty worktree
// slot (a worktree with no agent — a spawn target, whether or not it has a window).
type RowKind int

const (
	RowAgent  RowKind = iota // a live agent (status from its hook record)
	RowAnchor                // a managed repo's primary worktree (no agent)
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
	// AgentDir is the live agent's own working directory as reported by its hook,
	// for RowAgent rows. It is distinct from Dir (the worktree on disk): the
	// reconciler resolves it to a repository for recognition, and it is surfaced in
	// the headless JSON contract. Empty for anchor/slot rows and agents that
	// reported no directory.
	AgentDir string
	// Worktree is the worktree's name when the row is a managed worktree (RowSlot, or
	// a RowAgent running in a worktree window); empty for incidental agents and rows
	// outside a managed repo. A non-empty value is what makes delete also remove the
	// worktree.
	Worktree string
	// Repo is the repository name for managed rows (the primary worktree's basename);
	// empty for incidental agents. With Worktree it forms the durable `repo/worktree`
	// handle the headless verbs address work by.
	Repo string
	// Branch is the worktree's checked-out branch for managed rows; empty otherwise.
	Branch string
	// IsPrimary reports whether this row's worktree is git's primary worktree (the repo
	// base), independent of whether an agent runs in it. It is the first-class fact the
	// lifecycle guards key on — git refuses to remove a primary worktree, so it can never
	// be deleted and Enter opens a shell rather than spawning. Kind (anchor vs agent)
	// follows agent presence and so cannot carry this on its own: a recognized agent in
	// the base is a RowAgent that is still IsPrimary.
	IsPrimary bool
	// GitDir is the repository's shared git common dir — its identity across all its
	// worktrees. It is both the handle-ambiguity guard (two repos sharing a basename)
	// and the view's grouping key: every row of one repository shares a GitDir, so the
	// view sections by it. Empty for incidental agents (rendered ungrouped).
	GitDir string
	// Title is the row's display name.
	Title string
	// Status is the agent's status for RowAgent rows; anchor/slot rows render their
	// own marker and ignore this.
	Status Status
	// Updated is when the backing state was last written (RowAgent); used for the
	// most-recent dedup and selection stability.
	Updated time.Time
}

// isManagedWorktree reports whether the row is a removable managed worktree (not the
// primary): deleting it also runs `git worktree remove`. An incidental agent — a window
// with no worktree — is only closed, and the primary worktree is never removable. This
// mirrors the orchestrator's own delete logic.
func (r Row) isManagedWorktree() bool {
	return r.Worktree != "" && !r.IsPrimary && r.Dir != ""
}

// hasWindow reports whether the row has a live tmux window or pane of its own.
func (r Row) hasWindow() bool { return r.TmuxWindow != "" || r.TmuxPane != "" }

// isWindowlessStructural reports whether the row is a base or slot with no live window —
// a worktree on disk with no terminal attached. Capturing its session would surface an
// unrelated window, so it previews nothing and reads as a dormant state, not a failure.
func (r Row) isWindowlessStructural() bool {
	return !r.hasWindow() && (r.Kind == RowAnchor || r.Kind == RowSlot)
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
		Dir:            a.CWD,
		AgentDir:       a.CWD,
		Title:          a.Title,
		Status:         a.Status,
		Updated:        a.Updated,
	}
}
