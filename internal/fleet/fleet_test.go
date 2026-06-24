package fleet

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jbarap/birds-eye/internal/agents"
	"github.com/jbarap/birds-eye/internal/tmux"
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

// projFleet builds a fleet over a managed `proj` repo: a primary worktree (anchor, no
// agent), a `feat` worktree with a live agent, a windowless `spike` slot, and an
// incidental agent in /tmp.
func projFleet() *Fleet {
	const gd = "/code/proj/.git"
	src := fakeSource{list: []agents.Agent{
		{SessionID: "feat-agent", TmuxSession: "proj", TmuxWindow: "1", TmuxPane: "%2", Title: "feat", Status: agents.StatusWorking},
		{SessionID: "tmp-agent", TmuxSession: "proj", TmuxWindow: "3", TmuxPane: "%4", Title: "scratch", Status: agents.StatusNeedsAttention},
	}}
	panes := fakePanes{list: []agents.PaneInfo{
		{Session: "proj", WindowIndex: "0", PaneID: "%1", StartPath: "/code/proj"},
		{Session: "proj", WindowIndex: "1", PaneID: "%2", StartPath: "/code/proj.worktrees/feat"},
		{Session: "proj", WindowIndex: "3", PaneID: "%4", StartPath: "/tmp"},
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

	if r, ok := byHandle["proj"]; !ok || r.Kind != "anchor" {
		t.Fatalf("expected `proj` anchor record, got %+v", r)
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
		{"proj", "anchor", "/code/proj", true},
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
		`"path":"/code/proj.worktrees/feat"`, `"session":{"id":"$1","name":"proj"}`,
		`"window":"1"`, `"pane":"%2"`, `"status":"working"`, `"kind":"worktree"`,
	} {
		if !strings.Contains(s, key) {
			t.Fatalf("record JSON missing %s\ngot %s", key, s)
		}
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
