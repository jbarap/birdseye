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

func TestWorkspaceRecognizesRepo(t *testing.T) {
	// proj: primary clone (no agent → anchor), feat worktree (agent), spike worktree
	// (window, no agent → slot). Every pane is a worktree of the one repo, so the
	// repository is recognized and rendered as one section.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "feat-agent", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", Title: "feat", Status: StatusWorking},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "proj", WindowIndex: "2", PaneID: "%3", StartPath: "/code/proj.worktrees/spike"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{
			"/code/proj":                 repoAt(gd, "proj", "/code/proj", "proj", true),
			"/code/proj.worktrees/feat":  repoAt(gd, "proj", "/code/proj.worktrees/feat", "feat", false),
			"/code/proj.worktrees/spike": repoAt(gd, "proj", "/code/proj.worktrees/spike", "spike", false),
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
		byName[r.Worktree] = r
	}
	if r := byName["proj"]; r.Kind != RowAnchor || r.GitDir != gd {
		t.Fatalf("the primary worktree should be the anchor, got %+v", r)
	}
	if r := byName["feat"]; r.Kind != RowAgent || r.GitDir != gd || r.Dir != "/code/proj.worktrees/feat" {
		t.Fatalf("feat should be a worktree agent row, got %+v", r)
	}
	if r := byName["spike"]; r.Kind != RowSlot || r.Dir != "/code/proj.worktrees/spike" {
		t.Fatalf("spike should be a slot, got %+v", r)
	}
}

func TestWorkspaceStrayPaneDoesNotSuppressRepo(t *testing.T) {
	// A session whose panes are mostly worktrees but include one non-git pane still
	// recognizes the repository: the stray pane never poisons it. The stray pane's agent
	// falls through as an incidental (ungrouped) row; the repo keeps its anchor + agent.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "feat-agent", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", Title: "feat", Status: StatusWorking},
		{SessionID: "tmp-agent", TmuxSession: "proj", TmuxWindow: "2", TmuxPane: "%3", Title: "scratch", Status: StatusIdle},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "proj", WindowIndex: "2", PaneID: "%3", StartPath: "/tmp"}, // not a worktree
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
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	counts := rowsByKind(rows)
	if counts[RowAnchor] != 1 {
		t.Fatalf("the repository should still be recognized (1 anchor) despite a stray pane, got %+v rows=%+v", counts, rows)
	}
	byName := map[string]Row{}
	for _, r := range rows {
		byName[r.Worktree] = r
	}
	if r := byName["feat"]; r.Kind != RowAgent || r.GitDir != gd {
		t.Fatalf("the feat worktree agent should remain under the repo, got %+v", r)
	}
	// The /tmp pane is not a worktree of the repo, so it yields no worktree row; its agent
	// surfaces as an incidental, ungrouped row (no GitDir).
	var incidental *Row
	for i := range rows {
		if rows[i].SessionID == "tmp-agent" {
			incidental = &rows[i]
		}
	}
	if incidental == nil || incidental.GitDir != "" || incidental.Worktree != "" {
		t.Fatalf("the stray-pane agent should be an incidental ungrouped row, got %+v", incidental)
	}
}

func TestWorkspacePanesSpanTwoReposYieldTwoSections(t *testing.T) {
	// A session with panes resolving to two different repositories recognizes both: each
	// repository is an independent section, anchored by its own pane.
	const gdA, gdB = "/code/a/.git", "/code/b/.git"
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{
		{Session: "mix", WindowIndex: "0", PaneID: "%1", StartPath: "/code/a"},
		{Session: "mix", WindowIndex: "1", PaneID: "%2", StartPath: "/code/b"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{
			"/code/a": repoAt(gdA, "a", "/code/a", "a", true),
			"/code/b": repoAt(gdB, "b", "/code/b", "b", true),
		},
		wts: map[string][]WorktreeInfo{
			gdA: {{Path: "/code/a", Name: "a", IsPrimary: true}},
			gdB: {{Path: "/code/b", Name: "b", IsPrimary: true}},
		},
	}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if counts := rowsByKind(rows); counts[RowAnchor] != 2 {
		t.Fatalf("a two-repo session should yield two sections (2 anchors), got %+v rows=%+v", counts, rows)
	}
	gitDirs := map[string]bool{}
	for _, r := range rows {
		gitDirs[r.GitDir] = true
	}
	if !gitDirs[gdA] || !gitDirs[gdB] {
		t.Fatalf("both repositories should be recognized, got gitDirs=%v", gitDirs)
	}
}

func TestWorkspaceAgentCwdAnchorsRepo(t *testing.T) {
	// A pane born outside the repo (a tmuxp auto-cd workflow) but whose agent reports a
	// working directory inside it: the repository is recognized from the agent's cwd.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "feat-agent", TmuxSession: "work", TmuxWindow: "0", TmuxPane: "%1",
			CWD: "/code/proj.worktrees/feat", Title: "feat", Status: StatusWorking},
	}}
	panes := fakePanes{list: []PaneInfo{
		// The pane started in the user's home, not a worktree.
		{Session: "work", WindowIndex: "0", PaneID: "%1", StartPath: "/home/u"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{
			"/code/proj.worktrees/feat": repoAt(gd, "proj", "/code/proj.worktrees/feat", "feat", false),
		},
		wts: map[string][]WorktreeInfo{gd: {
			{Path: "/code/proj", Name: "proj", IsPrimary: true},
			{Path: "/code/proj.worktrees/feat", Name: "feat"},
		}},
	}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Row{}
	for _, r := range rows {
		byName[r.Worktree] = r
	}
	if r := byName["feat"]; r.Kind != RowAgent || r.GitDir != gd || r.TmuxSession != "work" {
		t.Fatalf("the agent's cwd should anchor the repo and place its row, got %+v", r)
	}
	if r := byName["proj"]; r.Kind != RowAnchor {
		t.Fatalf("the repo's primary worktree should render as the anchor, got %+v", r)
	}
}

func TestWorkspaceNoLivePresenceNotShown(t *testing.T) {
	// A repository with worktrees on disk but no pane and no agent resolving into it is
	// not shown: recognition does not scan the filesystem for repositories.
	src := &fakeSource{list: nil}
	panes := fakePanes{list: []PaneInfo{{Session: "elsewhere", WindowIndex: "0", PaneID: "%1", StartPath: "/somewhere"}}}
	repos := &fakeRepos{m: map[string]RepoInfo{}} // nothing resolves
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("a repository with no live presence should produce no rows, got %+v", rows)
	}
}

func TestWorkspaceTwoAgentsOneWorktreeTwoRows(t *testing.T) {
	// Two agents in the same worktree render as two rows (the worktree label repeats),
	// rather than folding into one. Row identity is one row per agent.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "a1", TmuxSession: "be-proj-x", TmuxWindow: "1", TmuxPane: "%2", CWD: "/code/proj.worktrees/feat", Title: "a1", Status: StatusWorking},
		{SessionID: "a2", TmuxSession: "be-proj-x", TmuxWindow: "2", TmuxPane: "%3", CWD: "/code/proj.worktrees/feat", Title: "a2", Status: StatusIdle},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "be-proj-x", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "be-proj-x", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "be-proj-x", WindowIndex: "2", PaneID: "%3", StartPath: "/code/proj.worktrees/feat"},
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
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	feat := 0
	for _, r := range rows {
		if r.Worktree == "feat" {
			if r.Kind != RowAgent {
				t.Fatalf("a feat row should be a live agent, got %+v", r)
			}
			feat++
		}
	}
	if feat != 2 {
		t.Fatalf("two agents in one worktree should render as two rows, got %d (rows=%+v)", feat, rows)
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
	if len(rows) != 1 || rows[0].Kind != RowAnchor {
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
	// An agent whose pane is not in any git worktree (and reports no repo cwd) is an
	// incidental, ungrouped row.
	src := &fakeSource{list: []Agent{
		{SessionID: "a", TmuxSession: "plain", TmuxWindow: "0", TmuxPane: "%1", Title: "a", Status: StatusIdle},
	}}
	panes := fakePanes{list: []PaneInfo{{Session: "plain", WindowIndex: "0", PaneID: "%1", StartPath: "/somewhere"}}}
	repos := &fakeRepos{m: map[string]RepoInfo{}} // nothing resolves

	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != RowAgent || rows[0].GitDir != "" {
		t.Fatalf("a non-repo agent should yield one incidental ungrouped row, got %+v", rows)
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

// TestWorkspacePrimaryWorktreeCarriesIsPrimary pins the first-class primary fact: the base
// row is IsPrimary, a slot is not, and — critically — a recognized agent running in the
// primary worktree is a RowAgent that is *still* IsPrimary. Kind follows agent presence;
// IsPrimary does not, so the lifecycle guards stay correct in that case.
func TestWorkspacePrimaryWorktreeCarriesIsPrimary(t *testing.T) {
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		// An agent the user started in the primary worktree's own window (recognition).
		{SessionID: "base-agent", TmuxSession: "proj", TmuxWindow: "0", TmuxPane: "%1", Title: "proj", Status: StatusWorking},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/spike"},
	}}
	repos := &fakeRepos{
		m: map[string]RepoInfo{
			"/code/proj":                 repoAt(gd, "proj", "/code/proj", "proj", true),
			"/code/proj.worktrees/spike": repoAt(gd, "proj", "/code/proj.worktrees/spike", "spike", false),
		},
		wts: map[string][]WorktreeInfo{gd: {
			{Path: "/code/proj", Name: "proj", IsPrimary: true},
			{Path: "/code/proj.worktrees/spike", Name: "spike"},
		}},
	}
	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Row{}
	for _, r := range rows {
		byName[r.Worktree] = r
	}
	if r := byName["proj"]; r.Kind != RowAgent || !r.IsPrimary {
		t.Fatalf("an agent in the primary worktree should be a RowAgent that is still IsPrimary, got %+v", r)
	}
	if r := byName["spike"]; r.Kind != RowSlot || r.IsPrimary {
		t.Fatalf("a slot must not be IsPrimary, got %+v", r)
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
