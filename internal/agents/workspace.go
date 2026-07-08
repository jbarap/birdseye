package agents

import "github.com/jbarap/birdseye/internal/providers/dir"

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

// SetMuted forwards the view's mute write to the underlying agent source when it persists
// mute (a ClaudeSource), so the Muter seam reaches the store through the reconciler. A
// source that is not a Muter makes mute a no-op.
func (w *Workspace) SetMuted(locKey string, muted bool) error {
	if mu, ok := w.src.(Muter); ok {
		return mu.SetMuted(locKey, muted)
	}
	return nil
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
// classification into the leaf rows the view renders. Recognition is repo-first: a
// repository (keyed by its git-common-dir) lights up when any pane's start path or any
// active agent's working directory resolves into it, regardless of which tmux session
// the evidence lives in. There is no whole-session purity gate — a stray non-git pane
// never suppresses a repository, and panes spanning two repositories yield two sections.
func (w *Workspace) Rows() ([]Row, error) {
	all, err := w.src.Agents()
	if err != nil {
		return nil, err
	}
	// The dash and headless verbs act on an agent through its tmux pane (jump, preview,
	// send, close-window). An agent whose hook ran outside tmux has no pane, so it is not
	// something this view can target or place in the tmux structure - it would otherwise
	// render as an un-jumpable row (e.g. background/headless Claude sessions that share a
	// repo's cwd, appearing as phantom agent rows in that repo). Drop it before recognition
	// and bucketing so it neither lights up a repo on its own nor produces a dead row. The
	// record stays on disk; the process-liveness GC still owns its removal.
	agentList := make([]Agent, 0, len(all))
	for _, a := range all {
		if a.TmuxPane != "" {
			agentList = append(agentList, a)
		}
	}
	panes, _ := w.panes.ListPanes() // best-effort: no tmux → every agent is a plain row

	byPane := map[string]Agent{}     // pane id -> agent
	paneStart := map[string]string{} // pane id -> start path
	for _, p := range panes {
		paneStart[p.PaneID] = p.StartPath
	}
	for _, a := range agentList {
		if a.TmuxPane != "" {
			byPane[a.TmuxPane] = a
		}
	}

	// Recognition: collect the repositories any pane or agent resolves into, keyed by
	// git-common-dir. Panes come first (the stable spine), then agents (covering an
	// auto-cd workflow where a pane is born outside the repo and the agent moves in).
	repoInfo := map[string]RepoInfo{}
	var repoOrder []string
	recognize := func(info *RepoInfo) {
		if info == nil {
			return
		}
		if _, ok := repoInfo[info.GitDir]; !ok {
			repoInfo[info.GitDir] = *info
			repoOrder = append(repoOrder, info.GitDir)
		}
	}
	for _, p := range panes {
		recognize(w.lookup(p.StartPath))
	}
	for _, a := range agentList {
		recognize(w.lookup(a.CWD))
	}

	claimed := map[string]bool{} // agent SessionIDs already rendered under a repo
	var rows []Row
	for _, gd := range repoOrder {
		rows = append(rows, w.repoRows(repoInfo[gd], agentList, panes, byPane, paneStart, claimed)...)
	}

	// Incidental agents: a live agent whose neither working directory nor pane start
	// path resolves into any repository (no git context). Rendered flat and ungrouped.
	for _, a := range agentList {
		if !claimed[a.SessionID] {
			rows = append(rows, agentRow(a))
		}
	}
	return rows, nil
}

// agentWorktree resolves the worktree an agent occupies: its recorded working directory
// when that is inside a git worktree (the durable signal, correct for an auto-cd
// workflow), else its pane's start path. Returns nil when neither resolves into a repo.
func (w *Workspace) agentWorktree(a Agent, paneStart map[string]string) *RepoInfo {
	if info := w.lookup(a.CWD); info != nil {
		return info
	}
	if sp := paneStart[a.TmuxPane]; sp != "" {
		return w.lookup(sp)
	}
	return nil
}

// repoRows builds the rows for one recognized repository, aggregating presence across
// every tmux session. It enumerates the repository's full git worktree set (not just
// open windows), so a worktree with no window still renders. Row identity is repo-first:
// one row per active agent (a worktree hosting two agents yields two rows, the worktree
// label repeating) plus one row per agentless worktree — the `⌂ base` primary or an
// `◌ slot` spawn target. Agents adopted as worktree rows are recorded in claimed.
func (w *Workspace) repoRows(repo RepoInfo, agentList []Agent, panes []PaneInfo, byPane map[string]Agent, paneStart map[string]string, claimed map[string]bool) []Row {
	home := dir.HomeSession(repo.GitDir)

	// Agents belonging to this repository, bucketed by the worktree they occupy.
	agentsByWt := map[string][]Agent{}
	for _, a := range agentList {
		if claimed[a.SessionID] {
			continue
		}
		info := w.agentWorktree(a, paneStart)
		if info == nil || info.GitDir != repo.GitDir {
			continue
		}
		agentsByWt[info.TopLevel] = append(agentsByWt[info.TopLevel], a)
	}

	// Panes of this repository, bucketed by worktree, for windows on agentless
	// worktrees and for the deterministic base/slot pane choice.
	panesByWt := map[string][]PaneInfo{}
	for _, p := range panes {
		info := w.lookup(p.StartPath)
		if info == nil || info.GitDir != repo.GitDir {
			continue
		}
		panesByWt[info.TopLevel] = append(panesByWt[info.TopLevel], p)
	}

	wts := w.worktrees(repo.GitDir)
	var out []Row
	for _, wt := range wts {
		if ags := agentsByWt[wt.Path]; len(ags) > 0 {
			for _, a := range ags {
				r := agentRow(a)
				r.Dir, r.Worktree = wt.Path, wt.Name
				r.Repo, r.Branch, r.GitDir = repo.Repo, wt.Branch, repo.GitDir
				r.IsPrimary = wt.IsPrimary // a recognized agent in the base is still primary
				out = append(out, r)
				claimed[a.SessionID] = true
			}
			continue
		}
		// No agent in this worktree: a base anchor (primary) or a slot spawn target,
		// carrying a representative pane when a window exists for it.
		r := Row{
			Dir: wt.Path, Worktree: wt.Name,
			Repo: repo.Repo, Branch: wt.Branch, GitDir: repo.GitDir, IsPrimary: wt.IsPrimary,
		}
		if wt.IsPrimary {
			r.Kind, r.SessionID = RowAnchor, "anchor:"+repo.GitDir
		} else {
			r.Kind, r.SessionID = RowSlot, "slot:"+repo.GitDir+":"+wt.Name
		}
		if p, ok := representativePane(panesByWt[wt.Path], byPane, home); ok {
			r.TmuxSession = p.Session
			r.TmuxWindow, r.TmuxWindowName, r.TmuxPane = p.WindowIndex, p.WindowName, p.PaneID
		}
		out = append(out, r)
	}
	return out
}

// representativePane picks the pane a base/slot row resolves to when its worktree has
// windows in more than one session. The choice is deterministic: prefer an agent-bearing
// pane, then a pane in the repository's `be-` home session, then the first in stable
// enumeration order. ok=false when the worktree has no open window at all.
func representativePane(panes []PaneInfo, byPane map[string]Agent, home string) (PaneInfo, bool) {
	if len(panes) == 0 {
		return PaneInfo{}, false
	}
	for _, p := range panes {
		if _, ok := byPane[p.PaneID]; ok {
			return p, true
		}
	}
	if home != "" {
		for _, p := range panes {
			if p.Session == home {
				return p, true
			}
		}
	}
	return panes[0], true
}
