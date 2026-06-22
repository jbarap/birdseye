package agents

import "strconv"

// PaneInfo is one tmux pane the reconciler classifies. It mirrors the tmux backend's
// pane enumeration without this package importing tmux (cli adapts the two).
type PaneInfo struct {
	Session     string
	WindowIndex string
	WindowName  string
	PaneID      string
	StartPath   string
}

// PaneLister enumerates the live tmux panes. *tmux.Client is adapted to this in cli.
type PaneLister interface {
	ListPanes() ([]PaneInfo, error)
}

// RepoInfo is the managed-repo context of a directory (git-derived).
type RepoInfo struct {
	Container     string
	Repo          string
	DefaultBranch string
	TopLevel      string
	Worktree      string
}

// RepoResolver resolves the repo a directory belongs to; ok=false when not inside a
// git worktree. worktree.Resolve is adapted to this in cli.
type RepoResolver interface {
	Resolve(dir string) (RepoInfo, bool)
}

// Workspace reconciles live agents, the tmux structure, and git facts into the rows
// the view renders. It recognizes managed repos and emits anchor/slot rows alongside
// agent rows. Nothing is persisted: Rows() re-derives each call. A per-start-path
// cache memoizes git lookups (including negative results) so the per-tick path is a
// map lookup rather than a git call.
type Workspace struct {
	src   Source
	panes PaneLister
	repos RepoResolver
	cache map[string]*RepoInfo // start path -> repo (nil = not a worktree)
}

// NewWorkspace builds a Workspace over an agent source, a tmux pane lister, and a git
// repo resolver. It satisfies RowSource.
func NewWorkspace(src Source, panes PaneLister, repos RepoResolver) *Workspace {
	return &Workspace{src: src, panes: panes, repos: repos, cache: map[string]*RepoInfo{}}
}

// lookup resolves (and memoizes) the repo context of a start path.
func (w *Workspace) lookup(dir string) *RepoInfo {
	if dir == "" {
		return nil
	}
	if v, ok := w.cache[dir]; ok {
		return v
	}
	info, ok := w.repos.Resolve(dir)
	if !ok {
		w.cache[dir] = nil
		return nil
	}
	w.cache[dir] = &info
	return &info
}

// Rows implements RowSource: it joins live agents with the tmux structure and git
// classification into the leaf rows the view renders.
func (w *Workspace) Rows() ([]Row, error) {
	agents, err := w.src.Agents()
	if err != nil {
		return nil, err
	}
	panes, _ := w.panes.ListPanes() // best-effort: no tmux → every agent is a plain row

	byPane := map[string]Agent{}
	for _, a := range agents {
		if a.TmuxPane != "" {
			byPane[a.TmuxPane] = a
		}
	}

	bySession := map[string][]PaneInfo{}
	var sessionOrder []string
	for _, p := range panes {
		if _, ok := bySession[p.Session]; !ok {
			sessionOrder = append(sessionOrder, p.Session)
		}
		bySession[p.Session] = append(bySession[p.Session], p)
	}

	claimed := map[string]bool{} // agent SessionIDs already rendered as a managed row
	var rows []Row

	for _, sess := range sessionOrder {
		anchor, ok := w.anchorOf(bySession[sess])
		if !ok {
			continue // not a managed repo; its agents fall through to the plain pass
		}
		rows = append(rows, w.managedRows(sess, bySession[sess], anchor, byPane, claimed)...)
		// Incidental agents in this managed session (windows that are not worktrees of it).
		for _, a := range agents {
			if a.TmuxSession == sess && !claimed[a.SessionID] {
				r := agentRow(a)
				r.Managed = true
				rows = append(rows, r)
				claimed[a.SessionID] = true
			}
		}
	}

	// Everything left: plain sessions, ungrouped agents, or agents without a pane match.
	for _, a := range agents {
		if !claimed[a.SessionID] {
			rows = append(rows, agentRow(a))
		}
	}
	return rows, nil
}

// anchorOf finds a session's anchor: the lowest-window-index pane that is its repo's
// default-branch checkout. ok=false means the session is not a managed repo.
func (w *Workspace) anchorOf(panes []PaneInfo) (RepoInfo, bool) {
	var anchor RepoInfo
	anchorWin := ""
	found := false
	for _, p := range panes {
		info := w.lookup(p.StartPath)
		if info == nil || info.DefaultBranch == "" || info.Worktree != info.DefaultBranch {
			continue
		}
		if !found || lessWindow(p.WindowIndex, anchorWin) {
			anchor, anchorWin, found = *info, p.WindowIndex, true
		}
	}
	return anchor, found
}

// managedRows builds the anchor/worktree/slot rows for one managed session. Each
// worktree of the anchor's container yields one row (the agent's if one runs there,
// else a slot — or the base marker for the default-branch worktree). Agents adopted as
// worktree rows are recorded in claimed.
func (w *Workspace) managedRows(sess string, panes []PaneInfo, anchor RepoInfo, byPane map[string]Agent, claimed map[string]bool) []Row {
	type entry struct {
		pane  PaneInfo
		info  *RepoInfo
		agent Agent
		has   bool
	}
	byName := map[string]*entry{}
	var names []string
	for _, p := range panes {
		info := w.lookup(p.StartPath)
		if info == nil || info.Container != anchor.Container {
			continue
		}
		a, has := byPane[p.PaneID]
		if e := byName[info.Worktree]; e == nil {
			byName[info.Worktree] = &entry{pane: p, info: info, agent: a, has: has}
			names = append(names, info.Worktree)
		} else if has && !e.has {
			e.pane, e.agent, e.has = p, a, true // prefer the pane that has an agent
		}
	}

	var out []Row
	for _, name := range names {
		e := byName[name]
		switch {
		case e.has:
			r := agentRow(e.agent)
			r.Dir, r.Worktree, r.Managed = e.info.TopLevel, name, true
			out = append(out, r)
			claimed[e.agent.SessionID] = true
		case name == anchor.DefaultBranch:
			out = append(out, Row{
				Kind: RowAnchor, SessionID: "anchor:" + sess,
				TmuxSession: sess, TmuxWindow: e.pane.WindowIndex, TmuxWindowName: e.pane.WindowName,
				TmuxPane: e.pane.PaneID, Dir: e.info.TopLevel, Worktree: name, Managed: true,
			})
		default:
			out = append(out, Row{
				Kind: RowSlot, SessionID: "slot:" + sess + ":" + name,
				TmuxSession: sess, TmuxWindow: e.pane.WindowIndex, TmuxWindowName: e.pane.WindowName,
				TmuxPane: e.pane.PaneID, Dir: e.info.TopLevel, Worktree: name, Managed: true,
			})
		}
	}
	return out
}

// lessWindow compares tmux window indices numerically when possible (so "2" < "10").
func lessWindow(a, b string) bool {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr == nil && berr == nil {
		return ai < bi
	}
	return a < b
}
