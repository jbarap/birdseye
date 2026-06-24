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
	GitDir    string // git common dir — the repository's identity across all its worktrees
	Repo      string // repository name (the primary worktree's base name)
	TopLevel  string // this directory's worktree top level
	Worktree  string // the worktree's name (base of TopLevel)
	IsPrimary bool   // whether this worktree is git's primary worktree (the anchor)
}

// WorktreeInfo is one worktree in a repository's git worktree set.
type WorktreeInfo struct {
	Path      string
	Name      string // base of Path
	Branch    string
	IsPrimary bool
}

// RepoResolver resolves git facts for the reconciler without this package importing git.
// worktree.Resolve / worktree.ListWorktrees are adapted to this in cli.
type RepoResolver interface {
	// Resolve classifies a directory; ok=false when it is not inside a git worktree.
	Resolve(dir string) (RepoInfo, bool)
	// Worktrees enumerates the full git worktree set for the repository identified by
	// gitDir (its shared git common dir), so worktrees with no open tmux window are
	// still surfaced.
	Worktrees(gitDir string) ([]WorktreeInfo, error)
}

// Invalidator is implemented by a RowSource whose Rows() are backed by cached git facts.
// The view calls Invalidate after a lifecycle action (spawn/remove) so the next refresh
// re-reads git rather than serving a stale worktree set.
type Invalidator interface{ Invalidate() }

// Workspace reconciles live agents, the tmux structure, and git facts into the rows
// the view renders. It recognizes managed repos and emits anchor/slot rows alongside
// agent rows. Nothing is persisted: Rows() re-derives each call. Two caches keep the
// per-tick path off git: a per-start-path cache memoizes repo classification (including
// negative results), and a per-repo cache memoizes the git worktree set; both are
// dropped by Invalidate after a lifecycle action.
type Workspace struct {
	src     Source
	panes   PaneLister
	repos   RepoResolver
	cache   map[string]*RepoInfo      // start path -> repo (nil = not a worktree)
	wtCache map[string][]WorktreeInfo // git common dir -> worktree set
}

// NewWorkspace builds a Workspace over an agent source, a tmux pane lister, and a git
// repo resolver. It satisfies RowSource.
func NewWorkspace(src Source, panes PaneLister, repos RepoResolver) *Workspace {
	return &Workspace{
		src:     src,
		panes:   panes,
		repos:   repos,
		cache:   map[string]*RepoInfo{},
		wtCache: map[string][]WorktreeInfo{},
	}
}

// Invalidate drops the cached git facts so the next Rows() re-reads git. Called by the
// view after a spawn or remove changes the on-disk worktree set.
func (w *Workspace) Invalidate() {
	w.cache = map[string]*RepoInfo{}
	w.wtCache = map[string][]WorktreeInfo{}
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

// worktrees resolves (and memoizes) the git worktree set for a repository.
func (w *Workspace) worktrees(gitDir string) []WorktreeInfo {
	if v, ok := w.wtCache[gitDir]; ok {
		return v
	}
	list, err := w.repos.Worktrees(gitDir)
	if err != nil {
		list = nil
	}
	w.wtCache[gitDir] = list
	return list
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
		repo, ok := w.managedRepoOf(bySession[sess])
		if !ok {
			continue // not a managed repo; its agents fall through to the plain pass
		}
		rows = append(rows, w.managedRows(sess, bySession[sess], repo, byPane, claimed)...)
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

// managedRepoOf decides whether a session is a managed repo and, if so, which repository
// it is. A session is managed iff at least one of its windows started inside a git
// worktree. When more than one repo is present, the primary-worktree window wins; failing
// that, the lowest-window-index worktree window. ok=false means the session is plain.
func (w *Workspace) managedRepoOf(panes []PaneInfo) (RepoInfo, bool) {
	var chosen RepoInfo
	chosenWin := ""
	found := false
	for _, p := range panes {
		info := w.lookup(p.StartPath)
		if info == nil {
			continue
		}
		switch {
		case !found:
			chosen, chosenWin, found = *info, p.WindowIndex, true
		case info.IsPrimary && !chosen.IsPrimary:
			chosen, chosenWin = *info, p.WindowIndex
		case info.IsPrimary == chosen.IsPrimary && lessWindow(p.WindowIndex, chosenWin):
			chosen, chosenWin = *info, p.WindowIndex
		}
	}
	return chosen, found
}

// managedRows builds the anchor/worktree/slot rows for one managed session. It enumerates
// the repository's full git worktree set (not just open windows), so a worktree with no
// window still renders as a slot. Each worktree yields one row: an agent row if an agent
// runs in its window, the `⌂ base` anchor row for the primary worktree, else an `◌ slot`
// spawn target. Agents adopted as worktree rows are recorded in claimed.
func (w *Workspace) managedRows(sess string, panes []PaneInfo, repo RepoInfo, byPane map[string]Agent, claimed map[string]bool) []Row {
	// Index this repo's open windows by their worktree path, preferring a pane that has
	// a live agent so the row carries the agent.
	type windowed struct {
		pane PaneInfo
		has  bool
	}
	byPath := map[string]windowed{}
	for _, p := range panes {
		info := w.lookup(p.StartPath)
		if info == nil || info.GitDir != repo.GitDir {
			continue
		}
		_, has := byPane[p.PaneID]
		if e, ok := byPath[info.TopLevel]; !ok || (has && !e.has) {
			byPath[info.TopLevel] = windowed{pane: p, has: has}
		}
	}

	wts := w.worktrees(repo.GitDir)
	var out []Row
	for _, wt := range wts {
		win, hasWindow := byPath[wt.Path]
		agent, hasAgent := Agent{}, false
		if hasWindow {
			agent, hasAgent = byPane[win.pane.PaneID]
		}
		switch {
		case hasAgent:
			r := agentRow(agent)
			r.Dir, r.Worktree, r.Managed = wt.Path, wt.Name, true
			r.Repo, r.Branch, r.GitDir = repo.Repo, wt.Branch, repo.GitDir
			out = append(out, r)
			claimed[agent.SessionID] = true
		case wt.IsPrimary:
			r := Row{
				Kind: RowAnchor, SessionID: "anchor:" + sess,
				TmuxSession: sess, Dir: wt.Path, Worktree: wt.Name, Managed: true,
				Repo: repo.Repo, Branch: wt.Branch, GitDir: repo.GitDir,
			}
			if hasWindow {
				r.TmuxWindow, r.TmuxWindowName, r.TmuxPane = win.pane.WindowIndex, win.pane.WindowName, win.pane.PaneID
			}
			out = append(out, r)
		default:
			r := Row{
				Kind: RowSlot, SessionID: "slot:" + sess + ":" + wt.Name,
				TmuxSession: sess, Dir: wt.Path, Worktree: wt.Name, Managed: true,
				Repo: repo.Repo, Branch: wt.Branch, GitDir: repo.GitDir,
			}
			if hasWindow {
				r.TmuxWindow, r.TmuxWindowName, r.TmuxPane = win.pane.WindowIndex, win.pane.WindowName, win.pane.PaneID
			}
			out = append(out, r)
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
