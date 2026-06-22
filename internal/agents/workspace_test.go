package agents

import "testing"

type fakePanes struct{ list []PaneInfo }

func (f fakePanes) ListPanes() ([]PaneInfo, error) { return f.list, nil }

// fakeRepos resolves start paths from a fixed map and counts calls (to prove caching).
type fakeRepos struct {
	m     map[string]RepoInfo
	calls int
}

func (f *fakeRepos) Resolve(dir string) (RepoInfo, bool) {
	f.calls++
	info, ok := f.m[dir]
	return info, ok
}

func wt(container, repo, def, name string) RepoInfo {
	return RepoInfo{Container: container, Repo: repo, DefaultBranch: def, TopLevel: container + "/" + name, Worktree: name}
}

// rowByWorktree finds the row for a worktree name (and, for incidental rows, by title).
func rowsByKind(rows []Row) map[RowKind]int {
	m := map[RowKind]int{}
	for _, r := range rows {
		m[r.Kind]++
	}
	return m
}

func TestWorkspaceRecognizesManagedRepo(t *testing.T) {
	// proj: anchor (main, no agent), feat (worktree + agent), spike (worktree, no agent),
	// plus an incidental agent in /tmp. One agent lives in feat's pane, one in /tmp.
	src := &fakeSource{list: []Agent{
		{SessionID: "feat-agent", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", Title: "feat", Status: StatusWorking},
		{SessionID: "tmp-agent", TmuxSession: "proj", TmuxWindow: "3", TmuxPane: "%4", Title: "scratch", Status: StatusNeedsAttention},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj/main"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj/feat"},
		{Session: "proj", WindowIndex: "2", PaneID: "%3", StartPath: "/code/proj/spike"},
		{Session: "proj", WindowIndex: "3", PaneID: "%4", StartPath: "/tmp"},
	}}
	repos := &fakeRepos{m: map[string]RepoInfo{
		"/code/proj/main":  wt("/code/proj", "proj", "main", "main"),
		"/code/proj/feat":  wt("/code/proj", "proj", "main", "feat"),
		"/code/proj/spike": wt("/code/proj", "proj", "main", "spike"),
		// /tmp is not a worktree → not present.
	}}

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
	if r := byName["main"]; r.Kind != RowAnchor || !r.Managed {
		t.Fatalf("main should be the anchor, got %+v", r)
	}
	if r := byName["feat"]; r.Kind != RowAgent || !r.Managed || r.Dir != "/code/proj/feat" {
		t.Fatalf("feat should be a managed worktree agent row, got %+v", r)
	}
	if r := byName["spike"]; r.Kind != RowSlot || r.Dir != "/code/proj/spike" {
		t.Fatalf("spike should be a slot, got %+v", r)
	}
	// The /tmp agent is incidental: a managed-session agent row, but not a worktree.
	inc := byName["incidental:tmp-agent"]
	if inc.Kind != RowAgent || inc.Worktree != "" || !inc.Managed {
		t.Fatalf("tmp agent should be an incidental managed row, got %+v", inc)
	}
}

func TestWorkspaceNoAnchorIsPlain(t *testing.T) {
	// A session whose windows are not in a repo's default-branch checkout is not managed.
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

func TestWorkspaceSecondMainDoesNotDuplicate(t *testing.T) {
	// Two windows both started in <repo>/main: still one managed repo, one anchor row.
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj/main"},
		{Session: "proj", WindowIndex: "5", PaneID: "%9", StartPath: "/code/proj/main"},
	}}
	repos := &fakeRepos{m: map[string]RepoInfo{
		"/code/proj/main": wt("/code/proj", "proj", "main", "main"),
	}}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != RowAnchor {
		t.Fatalf("a second main window must not duplicate the anchor, got %+v", rows)
	}
}

func TestWorkspaceCachesRepoLookups(t *testing.T) {
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj/main"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj/feat"},
	}}
	repos := &fakeRepos{m: map[string]RepoInfo{
		"/code/proj/main": wt("/code/proj", "proj", "main", "main"),
		"/code/proj/feat": wt("/code/proj", "proj", "main", "feat"),
	}}
	w := NewWorkspace(src, panes, repos)
	if _, err := w.Rows(); err != nil {
		t.Fatal(err)
	}
	first := repos.calls
	if _, err := w.Rows(); err != nil {
		t.Fatal(err)
	}
	if repos.calls != first {
		t.Fatalf("second Rows() should hit the cache, resolver calls grew %d → %d", first, repos.calls)
	}
}
