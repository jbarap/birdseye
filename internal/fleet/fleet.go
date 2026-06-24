// Package fleet is the shared operation layer behind both faces of agent
// management: the `be dash` TUI and the headless `be agents` verbs call into it,
// so neither holds private powers. It composes the tmux and worktree primitives
// into the work lifecycle (list, status, spawn, close, delete, jump, send) and
// derives durable `repo/worktree` handles from git and tmux on every call — never
// from persisted state, so an agent birdseye did not spawn is addressable for
// free.
package fleet

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jbarap/birdseye/internal/agents"
	"github.com/jbarap/birdseye/internal/providers/dir"
	"github.com/jbarap/birdseye/internal/tmux"
	"github.com/jbarap/birdseye/internal/worktree"
)

// Fleet performs the work-lifecycle operations over a tmux client and the git
// worktree primitives. It is constructed once per process and used by both the
// dash (via the cli orchestrator adapter) and the headless verbs.
type Fleet struct {
	client  *tmux.Client
	command string
	src     agents.Source
	panes   agents.PaneLister
	repos   agents.RepoResolver
}

// New builds a Fleet. command is the agent command to start in spawned windows
// (e.g. "claude"); src/panes/repos are the same substrate adapters the dash
// reconciler uses, so list/resolve see one observable world.
func New(client *tmux.Client, command string, src agents.Source, panes agents.PaneLister, repos agents.RepoResolver) *Fleet {
	return &Fleet{client: client, command: command, src: src, panes: panes, repos: repos}
}

// Session names a tmux session by both its stable id and its name.
type Session struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Record is one work item in the stable JSON contract the data verbs emit. Field
// names are additive-only: existing names stay stable across releases, new fields
// may appear. Empty optional fields are omitted.
type Record struct {
	Handle   string  `json:"handle"`
	Repo     string  `json:"repo,omitempty"`
	Worktree string  `json:"worktree,omitempty"`
	Branch   string  `json:"branch,omitempty"`
	Path     string  `json:"path,omitempty"`
	Session  Session `json:"session"`
	Window   string  `json:"window,omitempty"`
	Pane     string  `json:"pane,omitempty"`
	Status   string  `json:"status,omitempty"`
	Kind     string  `json:"kind"`
}

// Target is a resolved unit of work: enough to act on it (close its window,
// remove its worktree, jump to or send into its pane) without re-resolving.
type Target struct {
	Handle    string
	Repo      string
	Worktree  string
	Branch    string
	Dir       string
	GitDir    string
	Session   string
	Window    string
	Pane      string
	Status    string
	Kind      string
	IsPrimary bool
}

// rows reconciles the live substrate (agents + tmux + git) into the dash's rows.
// A fresh Workspace per call is correct here: each verb invocation is its own
// process, so there is no stale cache to invalidate.
func (f *Fleet) rows() ([]agents.Row, error) {
	return agents.NewWorkspace(f.src, f.panes, f.repos).Rows()
}

// kindOf maps a reconciled row to the contract's kind vocabulary.
func kindOf(r agents.Row) string {
	switch {
	case r.Kind == agents.RowAnchor:
		return "base"
	case r.Kind == agents.RowSlot:
		return "slot"
	case r.Worktree != "":
		return "worktree" // a live agent running in a managed worktree
	default:
		return "agent" // a live agent with no worktree (incidental / imported)
	}
}

// handleOf derives a row's address: `<repo>` for an anchor, `<repo>/<worktree>`
// for a linked worktree, the tmux pane id for a worktreeless agent.
func handleOf(r agents.Row) string {
	switch {
	case r.Kind == agents.RowAnchor:
		return r.Repo
	case r.Worktree != "" && r.Repo != "":
		return r.Repo + "/" + r.Worktree
	default:
		return r.TmuxPane
	}
}

// records joins reconciled rows with tmux session ids into the stable contract.
func (f *Fleet) records() ([]Record, error) {
	rows, err := f.rows()
	if err != nil {
		return nil, err
	}
	ids, _ := f.client.SessionIDs() // best-effort: empty map without a server
	out := make([]Record, 0, len(rows))
	for _, r := range rows {
		out = append(out, Record{
			Handle:   handleOf(r),
			Repo:     r.Repo,
			Worktree: r.Worktree,
			Branch:   r.Branch,
			Path:     r.Dir,
			Session:  Session{ID: ids[r.TmuxSession], Name: r.TmuxSession},
			Window:   r.TmuxWindow,
			Pane:     r.TmuxPane,
			Status:   statusString(r),
			Kind:     kindOf(r),
		})
	}
	return out, nil
}

// statusString reports an agent row's status; anchor/slot rows have none.
func statusString(r agents.Row) string {
	if r.Kind == agents.RowAgent {
		return string(r.Status)
	}
	return ""
}

// List returns every work record in the active set (the stable JSON contract).
func (f *Fleet) List() ([]Record, error) { return f.records() }

// Status returns the single record addressed by handle.
func (f *Fleet) Status(handle string) (Record, error) {
	t, err := f.Resolve(handle)
	if err != nil {
		return Record{}, err
	}
	ids, _ := f.client.SessionIDs()
	return Record{
		Handle:   t.Handle,
		Repo:     t.Repo,
		Worktree: t.Worktree,
		Branch:   t.Branch,
		Path:     t.Dir,
		Session:  Session{ID: ids[t.Session], Name: t.Session},
		Window:   t.Window,
		Pane:     t.Pane,
		Status:   t.Status,
		Kind:     t.Kind,
	}, nil
}

// Resolve turns a handle into a Target by re-deriving the active set and matching.
// A pane-id handle (leading "%") matches a worktreeless agent; otherwise the
// handle is `<repo>` or `<repo>/<worktree>`. An ambiguous handle (two repos in the
// active set share a basename) is refused with the disambiguating paths.
func (f *Fleet) Resolve(handle string) (Target, error) {
	rows, err := f.rows()
	if err != nil {
		return Target{}, err
	}
	var matches []agents.Row
	for _, r := range rows {
		if handleOf(r) == handle {
			matches = append(matches, r)
		}
	}
	switch {
	case len(matches) == 0:
		return Target{}, fmt.Errorf("no work matches handle %q", handle)
	case len(matches) == 1:
		return targetFromRow(matches[0]), nil
	}
	// More than one match: ambiguous only if they are different repositories
	// (distinct git dirs). Identical git dir would be a single worktree, so this
	// is the basename-collision case the design refuses rather than guessing.
	byGit := map[string]agents.Row{}
	for _, m := range matches {
		byGit[m.GitDir] = m
	}
	if len(byGit) == 1 {
		return targetFromRow(matches[0]), nil
	}
	var paths []string
	for _, m := range byGit {
		paths = append(paths, m.Dir)
	}
	sort.Strings(paths)
	return Target{}, fmt.Errorf("handle %q is ambiguous; it matches:\n  %s\ndisambiguate by the full path",
		handle, strings.Join(paths, "\n  "))
}

// targetFromRow lifts a reconciled row into an actionable Target.
func targetFromRow(r agents.Row) Target {
	return Target{
		Handle:    handleOf(r),
		Repo:      r.Repo,
		Worktree:  r.Worktree,
		Branch:    r.Branch,
		Dir:       r.Dir,
		GitDir:    r.GitDir,
		Session:   r.TmuxSession,
		Window:    r.TmuxWindow,
		Pane:      r.TmuxPane,
		Status:    statusString(r),
		Kind:      kindOf(r),
		IsPrimary: r.Kind == agents.RowAnchor,
	}
}

// FromRow lets the dash adapter build a Target from a Row it already holds, so a
// dash action and the corresponding verb go through the identical Close/Delete.
func FromRow(r agents.Row) Target { return targetFromRow(r) }

// Spawn creates a unit of work: it resolves repoDir to its repository, ensures
// (reuses) a tmux session, adds a sibling worktree for branch, opens a window
// rooted there, and starts the agent command — seeding prompt when non-empty. It
// returns the new worktree's `repo/worktree` handle. session may be empty, in
// which case it is derived from the repository name; the dash passes the live
// session it is already in so the window lands there.
func (f *Fleet) Spawn(repoDir, session, branch, name, prompt string) (string, error) {
	info, ok := f.repos.Resolve(repoDir)
	if !ok {
		return "", fmt.Errorf("not a git repository: %s", repoDir)
	}
	primary := filepath.Dir(info.GitDir)
	if session == "" {
		session = dir.SessionName(info.Repo)
	}
	if err := f.client.Ensure(session, primary); err != nil {
		return "", err
	}
	wtDir, err := worktree.Add(repoDir, name, branch)
	if err != nil {
		return "", err
	}
	command := f.command
	if prompt != "" {
		command = command + " " + shellQuote(prompt)
	}
	if _, err := f.client.NewWindow(session, wtDir, command); err != nil {
		return "", err
	}
	return info.Repo + "/" + filepath.Base(wtDir), nil
}

// Close tears down a unit of work's tmux window and leaves the worktree on disk
// (≡ the dash close action). A windowless slot has nothing to close.
func (f *Fleet) Close(t Target) error {
	f.closeWindow(t)
	return nil
}

// Delete closes the window and removes the git worktree (≡ the dash delete
// action). A primary worktree is refused (git will not remove it). A dirty
// worktree without force returns agents.ErrWorktreeDirty, before anything is
// killed, so a cancel is non-destructive. An incidental agent (no worktree) is
// only closed.
func (f *Fleet) Delete(t Target, force bool) error {
	if t.IsPrimary {
		return fmt.Errorf("%q is a primary worktree and cannot be deleted", t.Handle)
	}
	isWorktree := t.Worktree != "" && t.Dir != ""
	if isWorktree && !force {
		if dirty, err := worktree.IsDirty(t.Dir); err == nil && dirty {
			return agents.ErrWorktreeDirty
		}
	}
	f.closeWindow(t)
	if isWorktree {
		return worktree.Remove(t.Dir, force)
	}
	return nil
}

// closeWindow kills the target's window, addressing it by the stable pane id when
// known (the window index drifts as tmux renumbers), else by session:window.
func (f *Fleet) closeWindow(t Target) {
	switch {
	case t.Pane != "":
		_ = f.client.KillWindow(t.Pane) // best-effort; may be gone
	case t.Window != "":
		_ = f.client.KillWindow(t.Session + ":" + t.Window)
	}
}

// Jump connects the caller's tmux client to the target's session/window/pane.
func (f *Fleet) Jump(t Target) error {
	if t.Session == "" {
		return fmt.Errorf("%q has no live tmux window to jump to", t.Handle)
	}
	return f.client.ConnectPane(t.Session, t.Window, t.Pane)
}

// Send dispatches input to the target's agent, best-effort. It addresses the pane
// directly when known so a split window still reaches the agent's pane.
func (f *Fleet) Send(t Target, input string) error {
	target := t.Pane
	if target == "" {
		if t.Session == "" {
			return fmt.Errorf("%q has no live tmux pane to send to", t.Handle)
		}
		target = t.Session + ":" + t.Window
	}
	return f.client.SendKeys(target, input)
}

// shellQuote single-quotes s for safe interpolation into the agent command typed
// into the window's shell, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
