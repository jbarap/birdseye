package fleet

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jbarap/birdseye/internal/agents"
	"github.com/jbarap/birdseye/internal/providers/dir"
	"github.com/jbarap/birdseye/internal/tmux"
)

type fakeSource struct{ list []agents.Agent }

func (f fakeSource) Agents() ([]agents.Agent, error) { return f.list, nil }

type fakePanes struct{ list []agents.PaneInfo }

func (f fakePanes) ListPanes() ([]agents.PaneInfo, error) { return f.list, nil }

type fakeRepos struct {
	m   map[string]agents.RepoInfo
	wts map[string][]agents.WorktreeInfo
}

func (f fakeRepos) Resolve(dir string) (agents.RepoInfo, bool) {
	info, ok := f.m[dir]
	return info, ok
}

func (f fakeRepos) Worktrees(gitDir string) ([]agents.WorktreeInfo, error) { return f.wts[gitDir], nil }

func repoAt(gitDir, repo, top, name string, primary bool) agents.RepoInfo {
	return agents.RepoInfo{GitDir: gitDir, Repo: repo, TopLevel: top, Worktree: name, IsPrimary: primary}
}

// fakeClient builds a tmux client whose list-sessions reports a fixed session id, so
// records carry a session.id without a live server.
func fakeClient() *tmux.Client {
	return tmux.NewWithRunner(func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "list-sessions" {
			return "proj\t$1\n", nil
		}
		return "", nil
	}, false)
}

// projFleet builds a fleet over a recognized `proj` repo — a primary worktree (base, no
// agent), a `feat` worktree with a live agent, and a windowless `spike` slot — plus a
// separate `misc` session holding an incidental agent in /tmp (addressed by pane id). The
// /tmp pane is not inside any git worktree, so its agent resolves to no repository and
// renders as an incidental (ungrouped) row rather than under `proj`.
func projFleet() *Fleet {
	const gd = "/code/proj/.git"
	src := fakeSource{list: []agents.Agent{
		{SessionID: "feat-agent", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", CWD: "/code/proj.worktrees/feat", Title: "feat", Status: agents.StatusWorking},
		{SessionID: "tmp-agent", TmuxSession: "misc", TmuxWindow: "0", TmuxPane: "%4", Title: "scratch", Status: agents.StatusNeedsAttention},
	}}
	panes := fakePanes{list: []agents.PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "misc", WindowIndex: "0", PaneID: "%4", StartPath: "/tmp"},
	}}
	repos := fakeRepos{
		m: map[string]agents.RepoInfo{
			"/code/proj":                 repoAt(gd, "proj", "/code/proj", "proj", true),
			"/code/proj.worktrees/feat":  repoAt(gd, "proj", "/code/proj.worktrees/feat", "feat", false),
			"/code/proj.worktrees/spike": repoAt(gd, "proj", "/code/proj.worktrees/spike", "spike", false),
		},
		wts: map[string][]agents.WorktreeInfo{
			gd: {
				{Path: "/code/proj", Name: "proj", Branch: "main", IsPrimary: true},
				{Path: "/code/proj.worktrees/feat", Name: "feat", Branch: "feat"},
				{Path: "/code/proj.worktrees/spike", Name: "spike", Branch: "spike"},
			},
		},
	}
	return New(fakeClient(), "claude", src, panes, repos)
}

// TestHandlesAndKinds checks the handle scheme and kind vocabulary round-trip: every
// row gets the address the verbs expect, and Resolve maps it back to the same work.
func TestHandlesAndKinds(t *testing.T) {
	f := projFleet()
	recs, err := f.List()
	if err != nil {
		t.Fatal(err)
	}
	byHandle := map[string]Record{}
	for _, r := range recs {
		byHandle[r.Handle] = r
	}

	if r, ok := byHandle["proj"]; !ok || r.Kind != "base" {
		t.Fatalf("expected `proj` base record, got %+v", r)
	}
	if r, ok := byHandle["proj/feat"]; !ok || r.Kind != "worktree" || r.Status != string(agents.StatusWorking) {
		t.Fatalf("expected `proj/feat` worktree-agent record, got %+v", r)
	}
	if r, ok := byHandle["proj/spike"]; !ok || r.Kind != "slot" || r.Pane != "" {
		t.Fatalf("expected `proj/spike` windowless slot, got %+v", r)
	}
	if r, ok := byHandle["%4"]; !ok || r.Kind != "agent" || r.Worktree != "" {
		t.Fatalf("expected `%%4` incidental agent record, got %+v", r)
	}

	// Reverse: each handle resolves back to the matching target.
	for _, tc := range []struct {
		handle, wantKind, wantDir string
		primary                   bool
	}{
		{"proj", "base", "/code/proj", true},
		{"proj/feat", "worktree", "/code/proj.worktrees/feat", false},
		{"proj/spike", "slot", "/code/proj.worktrees/spike", false},
		{"%4", "agent", "", false},
	} {
		got, err := f.Resolve(tc.handle)
		if err != nil {
			t.Fatalf("resolve %q: %v", tc.handle, err)
		}
		if got.Kind != tc.wantKind || got.Dir != tc.wantDir || got.IsPrimary != tc.primary {
			t.Fatalf("resolve %q = %+v, want kind=%s dir=%s primary=%v", tc.handle, got, tc.wantKind, tc.wantDir, tc.primary)
		}
	}

	if _, err := f.Resolve("proj/nope"); err == nil {
		t.Fatal("resolving an unknown handle should error")
	}
}

// TestPrimaryWorktreeWithAgentAddressesAsBase guards the conflation fix end-to-end: a
// recognized agent in the primary worktree must still address as `<repo>` and resolve as a
// primary (base), so `be agents delete` refuses it cleanly instead of `proj/proj` slipping
// through to a git error.
func TestPrimaryWorktreeWithAgentAddressesAsBase(t *testing.T) {
	const gd = "/code/proj/.git"
	src := fakeSource{list: []agents.Agent{
		{SessionID: "base-agent", TmuxSession: "proj", TmuxWindow: "0", TmuxPane: "%1", Title: "proj", Status: agents.StatusWorking},
	}}
	panes := fakePanes{list: []agents.PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
	}}
	repos := fakeRepos{
		m:   map[string]agents.RepoInfo{"/code/proj": repoAt(gd, "proj", "/code/proj", "proj", true)},
		wts: map[string][]agents.WorktreeInfo{gd: {{Path: "/code/proj", Name: "proj", Branch: "main", IsPrimary: true}}},
	}
	f := New(fakeClient(), "claude", src, panes, repos)

	recs, err := f.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Handle != "proj" || recs[0].Kind != "base" {
		t.Fatalf("an agent in the primary worktree should address as the `proj` base, got %+v", recs)
	}

	got, err := f.Resolve("proj")
	if err != nil {
		t.Fatalf("resolve proj: %v", err)
	}
	if !got.IsPrimary {
		t.Fatalf("the resolved base must be IsPrimary, got %+v", got)
	}
	if err := f.Delete(got, false); err == nil {
		t.Fatal("deleting the primary worktree must be refused")
	}
}

// TestOpenNamesWindowAfterWorktree checks the window a re-opened row lands in is named
// after the worktree it holds — the slot by its worktree name, the base by the repo name —
// rather than inheriting tmux's process-derived default.
func TestOpenNamesWindowAfterWorktree(t *testing.T) {
	const gd = "/code/proj/.git"
	var calls []string
	runner := func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if len(args) > 0 && args[0] == "list-sessions" {
			return "proj\t$1\n", nil
		}
		return "", nil
	}
	repos := fakeRepos{m: map[string]agents.RepoInfo{
		"/code/proj":                 repoAt(gd, "proj", "/code/proj", "proj", true),
		"/code/proj.worktrees/spike": repoAt(gd, "proj", "/code/proj.worktrees/spike", "spike", false),
	}}
	f := New(tmux.NewWithRunner(runner, false), "claude", fakeSource{}, fakePanes{}, repos)

	called := func(substr string) bool {
		for _, c := range calls {
			if strings.HasPrefix(c, "new-window") && strings.Contains(c, substr) {
				return true
			}
		}
		return false
	}

	if _, err := f.Open(Target{Handle: "proj/spike", Dir: "/code/proj.worktrees/spike", Session: "proj"}); err != nil {
		t.Fatal(err)
	}
	if !called("-n spike") {
		t.Fatalf("slot window should be named after the worktree, calls=%v", calls)
	}

	if _, err := f.OpenShell(Target{Handle: "proj", Dir: "/code/proj", Session: "proj", IsPrimary: true}); err != nil {
		t.Fatal(err)
	}
	if !called("-n proj") {
		t.Fatalf("base window should be named after the repo, calls=%v", calls)
	}
}

// TestRecordJSONShape pins the stable field names another agent codes against.
func TestRecordJSONShape(t *testing.T) {
	f := projFleet()
	rec, err := f.Status("proj/feat")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{
		`"handle":"proj/feat"`, `"repo":"proj"`, `"worktree":"feat"`, `"branch":"feat"`,
		`"path":"/code/proj.worktrees/feat"`, `"agent_dir":"/code/proj.worktrees/feat"`,
		`"session":{"id":"$1","name":"proj"}`,
		`"window":"1"`, `"pane":"%2"`, `"status":"working"`, `"kind":"worktree"`,
	} {
		if !strings.Contains(s, key) {
			t.Fatalf("record JSON missing %s\ngot %s", key, s)
		}
	}
}

// TestOpenRoutesToHome pins the write-domain rule: re-opening a worktree routes the new
// window into the repository's deterministic home session, never into the row's prior
// (possibly user-made) session, even when the Target names one.
func TestOpenRoutesToHome(t *testing.T) {
	const gd = "/code/proj/.git"
	var calls []string
	runner := func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}
	repos := fakeRepos{m: map[string]agents.RepoInfo{
		"/code/proj.worktrees/spike": repoAt(gd, "proj", "/code/proj.worktrees/spike", "spike", false),
	}}
	f := New(tmux.NewWithRunner(runner, false), "claude", fakeSource{}, fakePanes{}, repos)

	// The Target carries a user session; Open must ignore it and use the home session.
	if _, err := f.Open(Target{Handle: "proj/spike", Dir: "/code/proj.worktrees/spike", Session: "my-user-session"}); err != nil {
		t.Fatal(err)
	}
	home := dir.HomeSession(gd)
	routed := false
	for _, c := range calls {
		if strings.HasPrefix(c, "new-window") && strings.Contains(c, "-t "+home+":") {
			routed = true
		}
		if strings.Contains(c, "my-user-session") {
			t.Fatalf("Open must not touch the user session, call=%q", c)
		}
	}
	if !routed {
		t.Fatalf("the new window should be created in the home session %q, calls=%v", home, calls)
	}
}

// TestDeleteRefusedWithCoTenant pins the sole-occupant guard at the fleet boundary: when a
// worktree hosts two agents, Delete refuses (removing nothing) so the worktree is never
// removed out from under a co-located agent.
func TestDeleteRefusedWithCoTenant(t *testing.T) {
	const gd = "/code/proj/.git"
	src := fakeSource{list: []agents.Agent{
		{SessionID: "a1", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", CWD: "/code/proj.worktrees/feat", Title: "a1", Status: agents.StatusWorking},
		{SessionID: "a2", TmuxSession: "proj", TmuxWindow: "2", TmuxPane: "%3", CWD: "/code/proj.worktrees/feat", Title: "a2", Status: agents.StatusIdle},
	}}
	panes := fakePanes{list: []agents.PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "proj", WindowIndex: "2", PaneID: "%3", StartPath: "/code/proj.worktrees/feat"},
	}}
	repos := fakeRepos{
		m: map[string]agents.RepoInfo{
			"/code/proj":                repoAt(gd, "proj", "/code/proj", "proj", true),
			"/code/proj.worktrees/feat": repoAt(gd, "proj", "/code/proj.worktrees/feat", "feat", false),
		},
		wts: map[string][]agents.WorktreeInfo{gd: {
			{Path: "/code/proj", Name: "proj", Branch: "main", IsPrimary: true},
			{Path: "/code/proj.worktrees/feat", Name: "feat", Branch: "feat"},
		}},
	}
	f := New(fakeClient(), "claude", src, panes, repos)

	t1 := Target{Handle: "proj/feat", Repo: "proj", Worktree: "feat", Dir: "/code/proj.worktrees/feat", GitDir: gd, Pane: "%2"}
	err := f.Delete(t1, false)
	if err == nil || !strings.Contains(err.Error(), "shares its worktree") {
		t.Fatalf("delete with a co-tenant should be refused, got err=%v", err)
	}
}

// TestAmbiguousHandleRefused checks that two repos sharing a basename make a handle
// ambiguous, and the verb refuses with the disambiguating paths rather than guessing.
func TestAmbiguousHandleRefused(t *testing.T) {
	src := fakeSource{}
	panes := fakePanes{list: []agents.PaneInfo{
		{Session: "a", WindowIndex: "0", PaneID: "%1", StartPath: "/work/a/proj"},
		{Session: "b", WindowIndex: "0", PaneID: "%2", StartPath: "/other/b/proj"},
	}}
	repos := fakeRepos{
		m: map[string]agents.RepoInfo{
			"/work/a/proj":  repoAt("/work/a/proj/.git", "proj", "/work/a/proj", "proj", true),
			"/other/b/proj": repoAt("/other/b/proj/.git", "proj", "/other/b/proj", "proj", true),
		},
		wts: map[string][]agents.WorktreeInfo{
			"/work/a/proj/.git":  {{Path: "/work/a/proj", Name: "proj", IsPrimary: true}},
			"/other/b/proj/.git": {{Path: "/other/b/proj", Name: "proj", IsPrimary: true}},
		},
	}
	f := New(fakeClient(), "claude", src, panes, repos)
	_, err := f.Resolve("proj")
	if err == nil {
		t.Fatal("expected an ambiguous-handle error")
	}
	if !strings.Contains(err.Error(), "/work/a/proj") || !strings.Contains(err.Error(), "/other/b/proj") {
		t.Fatalf("ambiguity error should list both paths, got: %v", err)
	}
}
