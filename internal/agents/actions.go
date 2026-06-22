package agents

import (
	"errors"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ErrWorktreeDirty is returned by Orchestrator.Remove when force is false and the
// worktree has uncommitted changes, so the view can prompt for force/cancel.
var ErrWorktreeDirty = errors.New("worktree has uncommitted changes")

// Orchestrator performs the create/delete side effects for managed repos. It is
// optional: when nil, the agents view is read-only (today's behavior).
type Orchestrator interface {
	// Spawn creates a worktree directory named name in the repo the row belongs to,
	// checking out branch (created off the default branch when new; empty defaults to
	// the worktree name), opens a tmux window (in the row's session) rooted there, and
	// starts the configured agent command.
	Spawn(repo Row, branch, name string) error
	// Remove tears down a row: it kills the row's tmux window and, when the row is a
	// managed worktree, removes the git worktree. With force=false it returns
	// ErrWorktreeDirty rather than removing a dirty worktree.
	Remove(r Row, force bool) error
	// NewSession runs the session picker and materializes the chosen session *without*
	// attaching, returning its tmux session name (empty when cancelled). It takes over
	// the terminal (fzf), so the view runs it through tea.Exec; not attaching keeps the
	// user in the agents view to keep orchestrating.
	NewSession() (string, error)
}

// pickerExec runs NewSession while Bubble Tea has released the terminal (via tea.Exec),
// so the picker's fzf can take the screen and restore cleanly afterward. It captures the
// chosen session name and any error for the completion message. The std streams Bubble Tea
// hands it are ignored: fzf manages its own /dev/tty.
type pickerExec struct {
	run  func() (string, error)
	name string
	err  error
}

func (p *pickerExec) Run() error          { p.name, p.err = p.run(); return p.err }
func (p *pickerExec) SetStdin(io.Reader)  {}
func (p *pickerExec) SetStdout(io.Writer) {}
func (p *pickerExec) SetStderr(io.Writer) {}

// sessionCreatedMsg reports the outcome of the new-session picker back into Update.
type sessionCreatedMsg struct {
	name string
	err  error
}

// startNewSession hands the terminal to the session picker (the same fuzzy list as
// `be sessions`) and materializes the chosen session without attaching, so the user stays
// in the agents view with the new session now listed. A no-op without an orchestrator.
func (m model) startNewSession() (tea.Model, tea.Cmd) {
	if m.orch == nil {
		return m, nil
	}
	m.clearNotice()
	pe := &pickerExec{run: m.orch.NewSession}
	return m, tea.Exec(pe, func(error) tea.Msg {
		return sessionCreatedMsg{name: pe.name, err: pe.err}
	})
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
	m.branchInput = ""
	m.worktreeInput = ""
	m.worktreeEdited = false
	m.focusField = fieldBranch
	m.target = r
	m.clearNotice()
	return m, nil
}

// handleNewAgentKey drives the two-field new-agent form. Enter submits with the current
// values, tab/shift-tab move between the branch and worktree fields, and esc cancels. The
// worktree field mirrors a slugified branch until the user edits it directly, after which
// it is left alone.
func (m model) handleNewAgentKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.cancelNewAgent()
	case tea.KeyEnter:
		return m.submitNewAgent()
	case tea.KeyTab, tea.KeyShiftTab:
		m.focusField = (m.focusField + 1) % numFormFields
	case tea.KeyBackspace:
		m.editField(func(s string) string {
			if r := []rune(s); len(r) > 0 {
				return string(r[:len(r)-1])
			}
			return s
		})
	case tea.KeyRunes:
		runes := string(key.Runes)
		m.editField(func(s string) string { return s + runes })
	}
	return m, nil
}

// editField applies edit to the focused field's buffer and keeps the two fields in sync:
// editing the branch re-derives the worktree slug (until the worktree was hand-edited),
// while editing the worktree marks it hand-edited so it stops following the branch.
func (m *model) editField(edit func(string) string) {
	switch m.focusField {
	case fieldBranch:
		m.branchInput = edit(m.branchInput)
		if !m.worktreeEdited {
			m.worktreeInput = worktreeSlug(m.branchInput)
		}
	case fieldWorktree:
		m.worktreeInput = edit(m.worktreeInput)
		m.worktreeEdited = true
	}
}

// cancelNewAgent closes the form and clears its buffers.
func (m *model) cancelNewAgent() {
	m.mode = modeNormal
	m.branchInput, m.worktreeInput, m.worktreeEdited, m.focusField = "", "", false, fieldBranch
}

// submitNewAgent spawns an agent from the form's branch and worktree values. The worktree
// directory defaults to the branch slug when left blank; an empty branch defers to Spawn,
// which names the branch after the worktree. Nothing happens when both are blank.
func (m model) submitNewAgent() (tea.Model, tea.Cmd) {
	branch := strings.TrimSpace(m.branchInput)
	name := strings.TrimSpace(m.worktreeInput)
	if name == "" {
		name = worktreeSlug(branch)
	}
	m.cancelNewAgent()
	if name == "" || m.orch == nil {
		return m, nil
	}
	if err := m.orch.Spawn(m.target, branch, name); err != nil {
		m.setError("new agent: " + err.Error())
		return m, nil
	}
	m.clearNotice()
	m.reload()
	return m, nil
}

// worktreeSlug turns a branch name into a filesystem-friendly worktree directory name:
// path separators and any other character outside [A-Za-z0-9._-] become a hyphen, runs of
// hyphens collapse, and leading/trailing hyphens and dots are trimmed. "feature/login" →
// "feature-login"; "fix/JIRA-123_bug" → "fix-JIRA-123_bug".
func worktreeSlug(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	slug := b.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-.")
}

// startDelete opens a confirmation before any delete — a deliberate gate so an
// accidental `dd` never tears down a window or worktree. The anchor is never removable.
// The actual removal happens in handleConfirmKey on "y".
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
	m.mode = modeConfirmDelete
	m.target = r
	return m, nil
}

// handleConfirmKey resolves the delete confirmation: y removes, n/esc cancel (the
// default). A dirty worktree declines the unforced remove and escalates to the force
// confirmation rather than discarding changes silently.
func (m model) handleConfirmKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "Y":
		err := m.orch.Remove(m.target, false)
		if errors.Is(err, ErrWorktreeDirty) {
			m.mode = modeConfirmForce
			return m, nil
		}
		m.mode = modeNormal
		if err != nil {
			m.setError("delete: " + err.Error())
			return m, nil
		}
		m.clearNotice()
		m.reload()
	case "n", "N", "esc", "ctrl+c", "q":
		m.mode = modeNormal
		m.setInfo("delete cancelled")
	}
	return m, nil
}

// handleConfirmForceKey resolves the dirty-worktree escalation: y forces removal
// (discarding changes), n/esc cancel (the default).
func (m model) handleConfirmForceKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "Y":
		m.mode = modeNormal
		if err := m.orch.Remove(m.target, true); err != nil {
			m.setError("delete: " + err.Error())
		} else {
			m.clearNotice()
			m.reload()
		}
	case "n", "N", "esc", "ctrl+c", "q":
		m.mode = modeNormal
		m.setInfo("delete cancelled")
	}
	return m, nil
}
