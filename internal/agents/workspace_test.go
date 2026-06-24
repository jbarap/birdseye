package agents

import "testing"

type fakePanes struct{ list []PaneInfo }

func (f fakePanes) ListPanes() ([]PaneInfo, error) { return f.list, nil }

// fakeRepos resolves start paths and worktree sets from fixed maps and counts calls (to
// prove caching).
type fakeRepos struct {
	m      map[string]RepoInfo       // start path -> repo classification
	wts    map[string][]WorktreeInfo // git dir -> worktree set
	calls  int                       // Resolve calls
	wcalls int                       // Worktrees calls
}

func (f *fakeRepos) Resolve(dir string) (RepoInfo, bool) {
	f.calls++
	info, ok := f.m[dir]
	return info, ok
}

func (f *fakeRepos) Worktrees(gitDir string) ([]WorktreeInfo, error) {
	f.wcalls++
	return f.wts[gitDir], nil
}

// repoAt builds a RepoInfo classification for a single start path.
func repoAt(gitDir, repo, top, name string, primary bool) RepoInfo {
	return RepoInfo{GitDir: gitDir, Repo: repo, TopLevel: top, Worktree: name, IsPrimary: primary}
}

func rowsByKind(rows []Row) map[RowKind]int {
	m := map[RowKind]int{}
	for _, r := range rows {
		m[r.Kind]++
	}
	return m
}

func TestWorkspaceRecognizesManagedRepo(t *testing.T) {
	// proj: primary clone (no agent → anchor), feat worktree (agent), spike worktree
	// (window, no agent → slot), plus an incidental agent in /tmp.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "feat-agent", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", Title: "feat", Status: StatusWorking},
		{SessionID: "tmp-agent", TmuxSession: "proj", TmuxWindow: "3", TmuxPane: "%4", Title: "scratch", Status: StatusNeedsAttention},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "proj", WindowIndex: "2", PaneID: "%3", StartPath: "/code/proj.worktrees/spike"},
		{Session: "proj", WindowIndex: "3", PaneID: "%4", StartPath: "/tmp"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{
			"/code/proj":                 repoAt(gd, "proj", "/code/proj", "proj", true),
			"/code/proj.worktrees/feat":  repoAt(gd, "proj", "/code/proj.worktrees/feat", "feat", false),
			"/code/proj.worktrees/spike": repoAt(gd, "proj", "/code/proj.worktrees/spike", "spike", false),
			// /tmp is not a worktree → not present.
		},
		wts: map[string][]WorktreeInfo{
			gd: {
				{Path: "/code/proj", Name: "proj", IsPrimary: true},
				{Path: "/code/proj.worktrees/feat", Name: "feat"},
				{Path: "/code/proj.worktrees/spike", Name: "spike"},
			},
		},
	}

	w := NewWorkspace(src, panes, repos)
	rows, err := w.Rows()
	if err != nil {
		t.Fatal(err)
	}
	counts := rowsByKind(rows)
	if counts[RowAnchor] != 1 || counts[RowSlot] != 1 {
		t.Fatalf("want 1 anchor + 1 slot, got %+v rows=%+v", counts, rows)
	}
	byName := map[string]Row{}
	for _, r := range rows {
		key := r.Worktree
		if r.Kind == RowAgent && r.Worktree == "" {
			key = "incidental:" + r.SessionID
		}
		byName[key] = r
	}
	if r := byName["proj"]; r.Kind != RowAnchor || !r.Managed {
		t.Fatalf("the primary worktree should be the anchor, got %+v", r)
	}
	if r := byName["feat"]; r.Kind != RowAgent || !r.Managed || r.Dir != "/code/proj.worktrees/feat" {
		t.Fatalf("feat should be a managed worktree agent row, got %+v", r)
	}
	if r := byName["spike"]; r.Kind != RowSlot || r.Dir != "/code/proj.worktrees/spike" {
		t.Fatalf("spike should be a slot, got %+v", r)
	}
	inc := byName["incidental:tmp-agent"]
	if inc.Kind != RowAgent || inc.Worktree != "" || !inc.Managed {
		t.Fatalf("tmp agent should be an incidental managed row, got %+v", inc)
	}
}

func TestWorkspaceAnchorIsPrimaryRegardlessOfBranch(t *testing.T) {
	// The primary worktree is the anchor even when its directory name is not the
	// default branch — recognition is git-native, not "basename == default branch".
	const gd = "/code/proj/.git"
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
	}}
	repos := &fakeRepos{
		m:   map[string]RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]WorktreeInfo{gd: {{Path: "/code/proj", Name: "proj", Branch: "release-2.0", IsPrimary: true}}},
	}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != RowAnchor || !rows[0].Managed {
		t.Fatalf("a primary worktree on a feature branch should still be the anchor, got %+v", rows)
	}
}

func TestWorkspaceWindowlessWorktreeIsSlot(t *testing.T) {
	// A worktree in the git set with no open tmux window still surfaces as a slot.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]WorktreeInfo{gd: {
			{Path: "/code/proj", Name: "proj", IsPrimary: true},
			{Path: "/code/proj.worktrees/ghost", Name: "ghost"}, // no window
		}},
	}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	var ghost *Row
	for i := range rows {
		if rows[i].Worktree == "ghost" {
			ghost = &rows[i]
		}
	}
	if ghost == nil || ghost.Kind != RowSlot || ghost.Dir != "/code/proj.worktrees/ghost" {
		t.Fatalf("a windowless worktree should render as a slot, got rows=%+v", rows)
	}
	if ghost.TmuxPane != "" {
		t.Fatalf("a windowless slot should carry no tmux pane, got %q", ghost.TmuxPane)
	}
}

func TestWorkspaceNoAnchorIsPlain(t *testing.T) {
	// A session whose windows are not in any git worktree is not managed.
	src := &fakeSource{list: []Agent{
		{SessionID: "a", TmuxSession: "plain", TmuxWindow: "0", TmuxPane: "%1", Title: "a", Status: StatusIdle},
	}}
	panes := fakePanes{list: []PaneInfo{{Session: "plain", WindowIndex: "0", PaneID: "%1", StartPath: "/somewhere"}}}
	repos := &fakeRepos{m: map[string]RepoInfo{}} // nothing resolves

	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != RowAgent || rows[0].Managed {
		t.Fatalf("a non-managed session should yield one plain agent row, got %+v", rows)
	}
}

func TestWorkspaceSecondPrimaryWindowDoesNotDuplicate(t *testing.T) {
	// Two windows both started in the primary worktree: still one managed repo, one anchor.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "5", PaneID: "%9", StartPath: "/code/proj"},
	}}
	repos := &fakeRepos{
		m:   map[string]RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]WorktreeInfo{gd: {{Path: "/code/proj", Name: "proj", IsPrimary: true}}},
	}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != RowAnchor {
		t.Fatalf("a second primary-worktree window must not duplicate the anchor, got %+v", rows)
	}
}

func TestWorkspaceCachesGitLookups(t *testing.T) {
	const gd = "/code/proj/.git"
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{
			"/code/proj":                repoAt(gd, "proj", "/code/proj", "proj", true),
			"/code/proj.worktrees/feat": repoAt(gd, "proj", "/code/proj.worktrees/feat", "feat", false),
		},
		wts: map[string][]WorktreeInfo{gd: {
			{Path: "/code/proj", Name: "proj", IsPrimary: true},
			{Path: "/code/proj.worktrees/feat", Name: "feat"},
		}},
	}
	w := NewWorkspace(src, panes, repos)
	if _, err := w.Rows(); err != nil {
		t.Fatal(err)
	}
	resolveCalls, wtCalls := repos.calls, repos.wcalls
	if _, err := w.Rows(); err != nil {
		t.Fatal(err)
	}
	if repos.calls != resolveCalls || repos.wcalls != wtCalls {
		t.Fatalf("second Rows() should hit the caches; Resolve %d→%d, Worktrees %d→%d",
			resolveCalls, repos.calls, wtCalls, repos.wcalls)
	}
	// After a lifecycle action the caches are dropped and git is re-read.
	w.Invalidate()
	if _, err := w.Rows(); err != nil {
		t.Fatal(err)
	}
	if repos.calls == resolveCalls || repos.wcalls == wtCalls {
		t.Fatalf("Invalidate should force a git re-read; Resolve %d→%d, Worktrees %d→%d",
			resolveCalls, repos.calls, wtCalls, repos.wcalls)
	}
}
