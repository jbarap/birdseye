package agents

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ErrWorktreeDirty is returned by Orchestrator.Remove when force is false and the
// worktree has uncommitted changes, so the view can prompt for force/cancel.
var ErrWorktreeDirty = errors.New("worktree has uncommitted changes")

// Orchestrator performs the create/delete side effects for managed repos. It is
// optional: when nil, the agents view is read-only (today's behavior).
type Orchestrator interface {
	// Spawn creates a worktree named name in the repo the row belongs to, opens a tmux
	// window (in the row's session) rooted there, and starts the configured agent
	// command.
	Spawn(repo Row, name string) error
	// Remove tears down a row: it kills the row's tmux window and, when the row is a
	// managed worktree, removes the git worktree. With force=false it returns
	// ErrWorktreeDirty rather than removing a dirty worktree.
	Remove(r Row, force bool) error
}

// startNewAgent opens the new-worktree name prompt when the cursor is in a managed
// repo. The action is a no-op without an orchestrator or outside a managed repo.
func (m model) startNewAgent() (tea.Model, tea.Cmd) {
	if m.orch == nil {
		return m, nil
	}
	r, ok := m.currentRow()
	if !ok || !r.Managed || r.Dir == "" {
		m.setError("new agent: place the cursor inside a managed repo")
		return m, nil
	}
	m.mode = modeNewAgent
	m.input = ""
	m.target = r
	m.clearNotice()
	return m, nil
}

// handleNewAgentKey drives the worktree-name text input. Enter submits, esc cancels;
// other navigation is inert while the prompt is open.
func (m model) handleNewAgentKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input = ""
	case tea.KeyEnter:
		name := strings.TrimSpace(m.input)
		m.mode = modeNormal
		m.input = ""
		if name != "" && m.orch != nil {
			if err := m.orch.Spawn(m.target, name); err != nil {
				m.setError("new agent: " + err.Error())
			} else {
				m.clearNotice()
				m.reload()
			}
		}
	case tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.input += " "
	case tea.KeyRunes:
		m.input += string(key.Runes)
	}
	return m, nil
}

// startDelete removes the row under the cursor. A clean worktree (or an incidental
// agent) is removed immediately; a dirty worktree opens a force/cancel confirm. The
// anchor is never removable.
func (m model) startDelete() (tea.Model, tea.Cmd) {
	if m.orch == nil {
		return m, nil
	}
	r, ok := m.currentRow()
	if !ok {
		return m, nil
	}
	if r.Kind == RowAnchor {
		m.setError("the repo anchor cannot be deleted")
		return m, nil
	}
	err := m.orch.Remove(r, false)
	if errors.Is(err, ErrWorktreeDirty) {
		m.mode = modeConfirmDelete
		m.target = r
		return m, nil
	}
	if err != nil {
		m.setError("delete: " + err.Error())
		return m, nil
	}
	m.clearNotice()
	m.reload()
	return m, nil
}

// handleConfirmKey resolves the dirty-worktree confirmation: y forces removal, n/esc
// cancel (the default).
func (m model) handleConfirmKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "Y":
		m.mode = modeNormal
		if m.orch != nil {
			if err := m.orch.Remove(m.target, true); err != nil {
				m.setError("delete: " + err.Error())
			} else {
				m.clearNotice()
				m.reload()
			}
		}
	case "n", "N", "esc", "ctrl+c", "q":
		m.mode = modeNormal
		m.setInfo("delete cancelled")
	}
	return m, nil
}
