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

// muterSource is an agent Source that also records mute writes, to prove the Muter seam is
// forwarded through the row adapters rather than dropped.
type muterSource struct {
	last string
	on   bool
}

func (m *muterSource) Agents() ([]Agent, error) { return nil, nil }
func (m *muterSource) SetMuted(key string, muted bool) error {
	m.last, m.on = key, muted
	return nil
}

// TestMuterDelegation pins that the view's Muter seam reaches the underlying source through
// both row adapters. Without it mute is silently a no-op in the real dash (which wraps the
// source in a Workspace) even though the direct toggle test passes - the gap that the live
// smoke test caught.
func TestMuterDelegation(t *testing.T) {
	cases := []struct {
		name string
		wrap func(Source) Muter
	}{
		{"workspace", func(s Source) Muter { return NewWorkspace(s, fakePanes{}, &fakeRepos{}) }},
		{"agents-as-rows", func(s Source) Muter { return AgentsAsRows(s).(Muter) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := &muterSource{}
			if err := tc.wrap(src).SetMuted("pane:%9", true); err != nil {
				t.Fatal(err)
			}
			if src.last != "pane:%9" || !src.on {
				t.Fatalf("%s should forward SetMuted to the source, got last=%q on=%v", tc.name, src.last, src.on)
			}
		})
	}
}

func rowsByKind(rows []Row) map[RowKind]int {
	m := map[RowKind]int{}
	for _, r := range rows {
		m[r.Kind]++
	}
	return m
}

// TestWorkspaceDirRowForNonGitSession pins that an open non-git session with no agent is
// surfaced as exactly one jump-only RowDir, carrying its representative directory.
func TestWorkspaceDirRowForNonGitSession(t *testing.T) {
	src := &fakeSource{}
	panes := fakePanes{list: []PaneInfo{
		{Session: "data", WindowIndex: "0", PaneID: "%1", WindowName: "shell", StartPath: "/media/ssd/datasets/court/v3"},
		{Session: "data", WindowIndex: "1", PaneID: "%2", StartPath: "/media/ssd/datasets/court/v3/sub"},
	}}
	repos := &fakeRepos{m: map[string]RepoInfo{}} // nothing resolves to a repo

	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	counts := rowsByKind(rows)
	if counts[RowDir] != 1 {
		t.Fatalf("want exactly 1 dir row, got %+v rows=%+v", counts, rows)
	}
	var r Row
	for _, row := range rows {
		if row.Kind == RowDir {
			r = row
		}
	}
	if r.TmuxSession != "data" || r.SessionID != "dir:data" {
		t.Fatalf("dir row identity wrong: %+v", r)
	}
	if r.Dir != "/media/ssd/datasets/court/v3" || r.TmuxPane != "%1" {
		t.Fatalf("dir row should carry the lowest-window pane, got Dir=%q Pane=%q", r.Dir, r.TmuxPane)
	}
	if r.Title != "v3" {
		t.Fatalf("dir row title should be the directory basename, got %q", r.Title)
	}
	if r.Worktree != "" || r.GitDir != "" || r.isManagedWorktree() {
		t.Fatalf("dir row must carry no worktree/git identity, got %+v", r)
	}
}

// TestWorkspaceNoDirRowWhenRepresented pins the subordinate-fallback gate: a session earns a
// dir row only when it has no repo pane and hosts no agent, so it never double-surfaces a
// session already shown as a repo section or an agent row.
func TestWorkspaceNoDirRowWhenRepresented(t *testing.T) {
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		// An incidental (non-git) agent occupies the "work" session.
		{SessionID: "a1", TmuxSession: "work", TmuxWindow: "0", TmuxPane: "%9", Title: "scratch", Status: StatusIdle},
	}}
	panes := fakePanes{list: []PaneInfo{
		// "mix": one repo pane + one non-git pane → represented by the repo section.
		{Session: "mix", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "mix", WindowIndex: "1", PaneID: "%2", StartPath: "/tmp/scratch"},
		// "work": non-git pane hosting an agent → represented by the agent row.
		{Session: "work", WindowIndex: "0", PaneID: "%9", StartPath: "/home/u/notes"},
	}}
	repos := &fakeRepos{
		m:   map[string]RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]WorktreeInfo{gd: {{Path: "/code/proj", Name: "proj", IsPrimary: true}}},
	}

	rows, err := NewWorkspace(src, panes, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if n := rowsByKind(rows)[RowDir]; n != 0 {
		t.Fatalf("no session should earn a dir row here, got %d: rows=%+v", n, rows)
	}
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
	// No repository is materialized: no anchor/slot rows and no row carries a git identity.
	// The stray open session surfaces as a jump-only dir row - a separate, intended behavior.
	counts := rowsByKind(rows)
	if counts[RowAnchor] != 0 || counts[RowSlot] != 0 {
		t.Fatalf("a repository with no live presence should produce no repo rows, got %+v", rows)
	}
	for _, r := range rows {
		if r.GitDir != "" {
			t.Fatalf("no row should carry a git identity, got %+v", r)
		}
	}
}

func TestWorkspaceTwoAgentsOneWorktreeTwoRows(t *testing.T) {
	// Two agents in the same worktree render as two rows (the worktree label repeats),
	// rather than folding into one. Row identity is one row per agent.
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "a1", TmuxSession: "proj-x", TmuxWindow: "1", TmuxPane: "%2", CWD: "/code/proj.worktrees/feat", Title: "a1", Status: StatusWorking},
		{SessionID: "a2", TmuxSession: "proj-x", TmuxWindow: "2", TmuxPane: "%3", CWD: "/code/proj.worktrees/feat", Title: "a2", Status: StatusIdle},
	}}
	panes := fakePanes{list: []PaneInfo{
		{Session: "proj-x", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj-x", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "proj-x", WindowIndex: "2", PaneID: "%3", StartPath: "/code/proj.worktrees/feat"},
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

// TestWorkspaceDetachedAgentsAreRows pins that agents whose hook ran outside tmux (no pane -
// background/daemon or headless sessions) are surfaced as agent rows grouped under their repo.
// They are real, running agents, so hiding them would misreport the fleet; the view marks them
// detached and gates the pane-dependent verbs. Each occupies the base worktree it runs in.
func TestWorkspaceDetachedAgentsAreRows(t *testing.T) {
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		// Two detached sessions in the primary worktree's cwd, no tmux location.
		{SessionID: "h1", CWD: "/code/proj", Title: "proj", Status: StatusWorking},
		{SessionID: "h2", CWD: "/code/proj", Title: "proj", Status: StatusWorking},
	}}
	repos := &fakeRepos{
		m:   map[string]RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]WorktreeInfo{gd: {{Path: "/code/proj", Name: "proj", IsPrimary: true}}},
	}
	rows, err := NewWorkspace(src, fakePanes{}, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("both detached agents should render as rows, got %+v", rows)
	}
	for _, r := range rows {
		if r.Kind != RowAgent {
			t.Errorf("detached agent should be an agent row, got kind %v", r.Kind)
		}
		if r.hasWindow() {
			t.Errorf("a detached agent has no window, got pane=%q window=%q", r.TmuxPane, r.TmuxWindow)
		}
		if !r.IsPrimary || r.Worktree != "proj" {
			t.Errorf("detached agent should occupy the base worktree, got %+v", r)
		}
	}
}

// TestWorkspaceDetachedOnlyRepoRenders pins that a repo whose only evidence is a detached
// agent's cwd (no pane anywhere) still renders: surfacing a background agent in its own repo
// is the point, so liveness alone lights the repo up.
func TestWorkspaceDetachedOnlyRepoRenders(t *testing.T) {
	const gd = "/code/proj/.git"
	src := &fakeSource{list: []Agent{
		{SessionID: "h1", CWD: "/code/proj", Title: "proj", Status: StatusWorking},
	}}
	repos := &fakeRepos{
		m:   map[string]RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]WorktreeInfo{gd: {{Path: "/code/proj", Name: "proj", IsPrimary: true}}},
	}
	rows, err := NewWorkspace(src, fakePanes{}, repos).Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != RowAgent || rows[0].SessionID != "h1" {
		t.Fatalf("a repo known only through a detached agent should render its agent row, got %+v", rows)
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
