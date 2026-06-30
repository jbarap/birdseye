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
	"github.com/jbarap/birdseye/internal/config"
	"github.com/jbarap/birdseye/internal/providers/dir"
	"github.com/jbarap/birdseye/internal/tmux"
	"github.com/jbarap/birdseye/internal/worktree"
)

// Fleet performs the work-lifecycle operations over a tmux client and the git
// worktree primitives. It is constructed once per process and used by both the
// dash (via the cli orchestrator adapter) and the headless verbs.
type Fleet struct {
	client *tmux.Client
	cfg    config.Config
	src    agents.Source
	panes  agents.PaneLister
	repos  agents.RepoResolver
}

// New builds a Fleet. cfg is the user-level configuration; for each repository-scoped
// operation it is overlaid with that repository's .birdseye/config.toml (see config.Overlay),
// so the agent command and setup command honor project overrides. src/panes/repos are the same
// substrate adapters the dash reconciler uses, so list/resolve see one observable world.
func New(client *tmux.Client, cfg config.Config, src agents.Source, panes agents.PaneLister, repos agents.RepoResolver) *Fleet {
	return &Fleet{client: client, cfg: cfg, src: src, panes: panes, repos: repos}
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
	AgentDir string  `json:"agent_dir,omitempty"`
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
	AgentDir  string
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

// kindOf maps a reconciled row to the contract's kind vocabulary. The primary worktree is
// the base whether or not an agent runs in it, so it keys on IsPrimary, not Kind.
func kindOf(r agents.Row) string {
	switch {
	case r.IsPrimary:
		return "base"
	case r.Kind == agents.RowSlot:
		return "slot"
	case r.Worktree != "":
		return "worktree" // a live agent running in a managed worktree
	default:
		return "agent" // a live agent with no worktree (incidental / imported)
	}
}

// handleOf derives a row's address: `<repo>` for the primary worktree (base),
// `<repo>/<worktree>` for a linked worktree, the tmux pane id for a worktreeless agent.
// The base keys on IsPrimary so a recognized agent in it still addresses as `<repo>`.
func handleOf(r agents.Row) string {
	switch {
	case r.IsPrimary:
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
			AgentDir: r.AgentDir,
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
		AgentDir: t.AgentDir,
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
		AgentDir:  r.AgentDir,
		GitDir:    r.GitDir,
		Session:   r.TmuxSession,
		Window:    r.TmuxWindow,
		Pane:      r.TmuxPane,
		Status:    statusString(r),
		Kind:      kindOf(r),
		IsPrimary: r.IsPrimary,
	}
}

// FromRow lets the dash adapter build a Target from a Row it already holds, so a
// dash action and the corresponding verb go through the identical Close/Delete.
func FromRow(r agents.Row) Target { return targetFromRow(r) }

// Spawn creates a unit of work: it resolves repoDir to its repository, ensures
// (reuses) the repository's deterministic home session, adds a sibling worktree
// for branch, opens a window rooted there, and starts the agent command - seeding
// prompt when non-empty. It returns the new worktree's `repo/worktree` handle. The home
// is a pure function of the repository's identity (dir.HomeSession), so spawn always
// routes into be's write domain and never injects a window into a user-made session,
// even when the repository's only existing presence is in one.
func (f *Fleet) Spawn(repoDir, branch, name, prompt string) (string, error) {
	info, ok := f.repos.Resolve(repoDir)
	if !ok {
		return "", fmt.Errorf("not a git repository: %s", repoDir)
	}
	primary := filepath.Dir(info.GitDir)
	session := dir.HomeSession(info.GitDir)
	if err := f.client.Ensure(session, primary, info.Repo); err != nil {
		return "", err
	}
	eff, err := config.Overlay(f.cfg, primary)
	if err != nil {
		return "", err
	}
	wtDir, _, err := worktree.Add(repoDir, name, branch)
	if err != nil {
		return "", err
	}
	command := eff.Agents.AgentCommand()
	if prompt != "" {
		command = command + " " + shellQuote(prompt)
	}
	// Provision the new worktree by chaining its setup command into the agent's window
	// (rather than running it inline, which would block the spawn call): the window runs
	// setup and then the agent, so setup output is visible in the agent's pane and a failed
	// setup short-circuits the `&&`, leaving a shell instead of an agent in a broken env.
	command = chainSetup(command, eff.Worktree.Setup, wtDir, primary, branch)
	// Name the agent's window after the worktree it holds.
	if _, err := f.client.NewWindow(session, wtDir, filepath.Base(wtDir), command); err != nil {
		return "", err
	}
	return info.Repo + "/" + filepath.Base(wtDir), nil
}

// Open starts an agent in an existing worktree — a windowless slot left behind when a
// window was closed with `dd`. It is Spawn minus worktree.Add: the directory must already
// exist, so re-opening a slot is identical to a fresh spawn minus the checkout. A primary
// worktree is refused — the base is not a spawn slot (use OpenShell for it).
func (f *Fleet) Open(t Target) (string, error) {
	if t.IsPrimary {
		return "", fmt.Errorf("%q is a primary worktree, not a slot", t.Handle)
	}
	return f.openWindow(t, true)
}

// OpenShell opens a plain (agentless) window in an existing worktree, returning the
// `repo/worktree` handle. It re-gives the repo base a window after `dd` closed it — the
// base is agentless by design, so it gets a shell, not the agent command. Unlike Open it
// allows a primary worktree.
func (f *Fleet) OpenShell(t Target) (string, error) {
	return f.openWindow(t, false)
}

// openWindow resolves the worktree's repository, ensures (reuses) the repository's home
// session, opens a window rooted at the worktree, and returns the `repo/worktree` handle.
// When agent is true the window starts the repository's effective agent command (honoring its
// .birdseye/config.toml override); when false it leaves a plain shell. The directory must
// already exist; it adds no worktree (re-waking a slot or base does not re-provision). Like
// Spawn it routes into be's write domain (dir.HomeSession), so it never injects a window into a
// user-made session.
func (f *Fleet) openWindow(t Target, agent bool) (string, error) {
	if t.Dir == "" {
		return "", fmt.Errorf("no worktree to open")
	}
	info, ok := f.repos.Resolve(t.Dir)
	if !ok {
		return "", fmt.Errorf("not a git repository: %s", t.Dir)
	}
	command := ""
	if agent {
		eff, err := config.Overlay(f.cfg, filepath.Dir(info.GitDir))
		if err != nil {
			return "", err
		}
		command = eff.Agents.AgentCommand()
	}
	session := dir.HomeSession(info.GitDir)
	if err := f.client.Ensure(session, filepath.Dir(info.GitDir), info.Repo); err != nil {
		return "", err
	}
	// Name the window after the worktree it holds (the repo name for the base).
	if _, err := f.client.NewWindow(session, t.Dir, filepath.Base(t.Dir), command); err != nil {
		return "", err
	}
	return info.Repo + "/" + filepath.Base(t.Dir), nil
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
	// Sole-occupant guard: a worktree is removed only when the target is its only row.
	// A co-tenant (e.g. a second agent in the same worktree) blocks removal — close the
	// others first — so a worktree is never removed out from under a co-located agent.
	if isWorktree {
		if n, err := f.worktreeOccupants(t); err == nil && n > 1 {
			return fmt.Errorf("%q shares its worktree with %d other agent(s); close them first", t.Handle, n-1)
		}
	}
	if isWorktree && !force {
		if dirty, err := worktree.IsDirty(t.Dir); err == nil && dirty {
			return agents.ErrWorktreeDirty
		}
	}
	f.closeWindow(t)
	if isWorktree {
		return worktree.Remove(t.Dir, t.GitDir, force)
	}
	return nil
}

// worktreeOccupants counts the reconciled rows sharing the target's worktree (its
// directory within the same repository) — the sole-occupant test for Delete. A count
// above one means a co-tenant agent is present and the worktree must not be removed yet.
func (f *Fleet) worktreeOccupants(t Target) (int, error) {
	rows, err := f.rows()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if r.Dir == t.Dir && r.GitDir == t.GitDir {
			n++
		}
	}
	return n, nil
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

// chainSetup prefixes setup ahead of agentCommand so the spawned window runs
// `<env> <setup> && <agent>`: setup runs first with the env contract, and the `&&` means the
// agent starts only if setup succeeds. An empty setup returns the agent command unchanged. The
// env assignments precede the first command of the chain, so they apply to the setup command
// only - the same contract the agentless path exports.
func chainSetup(agentCommand, setup, wtDir, primary, branch string) string {
	if strings.TrimSpace(setup) == "" {
		return agentCommand
	}
	var prefix strings.Builder
	for _, kv := range worktree.SetupEnv(wtDir, primary, branch) {
		eq := strings.IndexByte(kv, '=')
		prefix.WriteString(kv[:eq+1] + shellQuote(kv[eq+1:]) + " ")
	}
	return prefix.String() + setup + " && " + agentCommand
}
