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
	// Open starts an agent in an existing windowless worktree (a slot): it opens a tmux
	// window rooted at the slot's worktree and starts the configured agent command,
	// adding no worktree (the directory already exists). It brings a dormant slot — a
	// worktree whose window was closed with `dd` — back to life.
	Open(slot Row) error
	// OpenShell opens a plain (agentless) tmux window rooted at the row's worktree, adding
	// no worktree. It re-gives the repo base a window after its window was closed with
	// `dd` — the base is agentless by design, so it gets a shell, not an agent.
	OpenShell(r Row) error
	// Close tears down a row's tmux window only, leaving any managed worktree on disk
	// (it then renders as a slot). It has no filesystem effect; the headless analog is
	// `be agents close`.
	Close(r Row) error
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
	if !ok || r.GitDir == "" || r.Dir == "" {
		m.setError("new agent: place the cursor inside a recognized repo")
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

// wake brings a windowless worktree row (a base or slot — see Row.isWindowlessStructural)
// back to life and attaches to it. The base is agentless by design so it gets a plain
// shell; a slot is a spawn target so it gets an agent. wake then reloads and selects the
// now-windowed row. Resolving the attach target from the reloaded rows — rather than
// trusting open to return coordinates — means it is the same reconciled row the rest of
// the view uses. If the window can't be found after opening, it reports the action and
// stays put rather than attaching to nothing.
func (m model) wake(r Row) (tea.Model, tea.Cmd) {
	if m.orch == nil {
		return m, nil
	}
	open := m.orch.Open // a slot is a spawn target → an agent
	if r.IsPrimary {
		open = m.orch.OpenShell // the base is agentless → a plain shell
	}
	if err := open(r); err != nil {
		m.setError("open: " + err.Error())
		return m, nil
	}
	m.reloadAfterAction()
	if w, ok := m.windowedRowForWorktree(r); ok {
		m.chosen = &w
		return m, tea.Quit
	}
	m.setInfo("opened a window")
	return m, nil
}

// windowedRowForWorktree finds the now-windowed row for a just-opened worktree, identified
// by its directory, so Enter can attach to the window wake created. The worktree directory
// is the stable per-worktree identity (unique across a repo's slots and its base), so it
// survives the row's kind changing from slot/base to windowed - and survives wake routing
// the new window into the repository's home session rather than the row's prior session.
func (m model) windowedRowForWorktree(src Row) (Row, bool) {
	for _, r := range m.rows {
		if src.Dir != "" && r.Dir == src.Dir && r.hasWindow() {
			return r, true
		}
	}
	return Row{}, false
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
	m.reloadAfterAction()
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

// startClose closes the row's tmux window with no confirmation and no filesystem
// effect — the safe teardown (`dd`). Under the sibling-worktree layout a managed
// worktree persists when its window closes and drops to a slot, so the action is
// reversible and needs no gate. A windowless slot has nothing to close (a no-op). The
// anchor's window closes like any other (ending the session if it was the last).
func (m model) startClose() (tea.Model, tea.Cmd) {
	if m.orch == nil {
		return m, nil
	}
	r, ok := m.currentRow()
	if !ok {
		return m, nil
	}
	if !r.hasWindow() {
		// Nothing to close. Say so rather than no-op silently, so the keystroke never
		// looks like it was swallowed — and phrase it for what the row actually is: the
		// repo base (which `dD` cannot remove) reads differently from an empty slot.
		m.setInfo(nothingToCloseMsg(r))
		return m, nil
	}
	if err := m.orch.Close(r); err != nil {
		m.setError("close: " + err.Error())
		return m, nil
	}
	m.clearNotice()
	m.reloadAfterAction()
	return m, nil
}

// nothingToCloseMsg phrases the windowless-close feedback for what the row is. A managed
// worktree slot can be torn down with `dD`, so its hint points there; the repo base has no
// window and cannot be removed (`dD` is blocked on the anchor), so it must not suggest one.
func nothingToCloseMsg(r Row) string {
	switch {
	case r.Kind == RowAgent && !r.hasWindow():
		return "detached agent — no window to close"
	case r.IsPrimary:
		return "nothing to close — the repo base has no window"
	case r.isManagedWorktree():
		return "nothing to close — empty slot (dD removes the worktree)"
	default:
		return "nothing to close — no window here"
	}
}

// startDelete opens a confirmation before any delete — a deliberate gate so an
// accidental `dD` never removes a worktree. The primary worktree is never removable (git
// refuses to remove it) — keyed on IsPrimary so a recognized agent in the base is refused
// here, cleanly, rather than failing later in git. Removal happens in handleConfirmKey.
func (m model) startDelete() (tea.Model, tea.Cmd) {
	if m.orch == nil {
		return m, nil
	}
	r, ok := m.currentRow()
	if !ok {
		return m, nil
	}
	if r.IsPrimary {
		m.setError("the repo base cannot be deleted — dd closes its window")
		return m, nil
	}
	// A detached agent (background/daemon or headless) has no window to close, so it cannot be
	// cleared out of its worktree first; removing the worktree would pull it out from under a
	// live agent. Refuse while it runs — the liveness GC drops it when the process exits.
	if r.Kind == RowAgent && !r.hasWindow() {
		m.setError("detached agent — no window to close; its worktree can't be removed while it runs")
		return m, nil
	}
	// Sole-occupant guard: removing a worktree is permitted only when the target is the
	// only row of that worktree. With a co-tenant (e.g. a second agent in the same
	// worktree), refuse here — before the confirmation — so a worktree is never removed
	// out from under a co-located agent. The user must close the others first.
	if r.isManagedWorktree() && m.worktreeOccupants(r) > 1 {
		m.setError("this worktree has other agents — close them first (dd), then dD removes it")
		return m, nil
	}
	m.mode = modeConfirmDelete
	m.target = r
	m.confirmChoice = 0 // default to cancel, the safe choice
	return m, nil
}

// worktreeOccupants counts the rows that share the target's worktree (its directory within
// the same repository). It is the sole-occupant test for delete: a count above one means a
// co-tenant agent is present, so the worktree must not be removed yet. A row with no
// worktree directory has no occupancy to share and counts as none.
func (m model) worktreeOccupants(r Row) int {
	if r.Dir == "" {
		return 0
	}
	n := 0
	for _, x := range m.rows {
		if x.Dir == r.Dir && x.GitDir == r.GitDir {
			n++
		}
	}
	return n
}

// handleConfirmKey resolves the delete confirmation. The two buttons (cancel / confirm)
// are selectable: ←/→/h/l/tab move between them, ⏎ activates the highlighted one. The
// y/n shortcuts still fire directly so muscle memory survives. A dirty worktree declines
// the unforced remove and escalates to the force confirmation rather than discarding
// changes silently.
func (m model) handleConfirmKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "left", "right", "h", "l", "tab":
		m.confirmChoice ^= 1
		return m, nil
	case "enter":
		if m.confirmChoice == 0 {
			return m.cancelConfirm()
		}
		fallthrough
	case "y", "Y":
		return m.runDelete(false)
	case "n", "N", "esc", "ctrl+c", "q":
		return m.cancelConfirm()
	}
	return m, nil
}

// handleConfirmForceKey resolves the dirty-worktree escalation, sharing the confirm
// modal's selectable buttons. y/⏎-on-confirm forces removal (discarding changes); n/esc
// cancel (the default).
func (m model) handleConfirmForceKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "left", "right", "h", "l", "tab":
		m.confirmChoice ^= 1
		return m, nil
	case "enter":
		if m.confirmChoice == 0 {
			return m.cancelConfirm()
		}
		fallthrough
	case "y", "Y":
		m.mode = modeNormal
		if err := m.orch.Remove(m.target, true); err != nil {
			m.setError("delete: " + err.Error())
		} else {
			m.clearNotice()
			m.reloadAfterAction()
		}
		return m, nil
	case "n", "N", "esc", "ctrl+c", "q":
		return m.cancelConfirm()
	}
	return m, nil
}

// runDelete probes an unforced remove of the target; a dirty worktree escalates to the
// force confirmation (resetting the button selection to its safe default) instead of
// discarding changes. force is reserved for the escalated path in handleConfirmForceKey.
func (m model) runDelete(force bool) (tea.Model, tea.Cmd) {
	err := m.orch.Remove(m.target, force)
	if errors.Is(err, ErrWorktreeDirty) {
		m.mode = modeConfirmForce
		m.confirmChoice = 0
		return m, nil
	}
	m.mode = modeNormal
	if err != nil {
		m.setError("delete: " + err.Error())
		return m, nil
	}
	m.clearNotice()
	m.reloadAfterAction()
	return m, nil
}

// toggleMute flips the user's mute intent on the selected agent and persists it through the
// source's Muter seam, so it follows the agent's tmux location and survives refreshes. It is
// a no-op on a non-agent row (a header, anchor, or slot) or a source that is not a Muter
// (the read-only and test paths). The reload restamps the flag onto the rows.
func (m model) toggleMute() (tea.Model, tea.Cmd) {
	muter, ok := m.src.(Muter)
	if !ok {
		return m, nil
	}
	r, ok := m.currentRow()
	if !ok || r.Kind != RowAgent {
		return m, nil
	}
	if err := muter.SetMuted(r.muteKey(), !r.Muted); err != nil {
		m.setError("mute: " + err.Error())
		return m, nil
	}
	m.clearNotice()
	m.reloadAfterAction()
	return m, nil
}

// cancelConfirm dismisses either confirm modal without acting, reporting the cancel.
func (m model) cancelConfirm() (tea.Model, tea.Cmd) {
	m.mode = modeNormal
	m.setInfo("delete cancelled")
	return m, nil
}
