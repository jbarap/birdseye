package agents

import (
	"strings"
	"testing"
)

// nsRow builds a repo-tagged agent row for the namespace tests: its Dir is what membership
// resolves against, GitDir is its repo identity, IsPrimary marks the repo's home worktree.
func nsRow(id, repo, gitdir, dir string, primary bool, status Status) Row {
	return Row{
		Kind: RowAgent, SessionID: id, Title: id, TmuxSession: id,
		Status: status, Repo: repo, GitDir: gitdir, Dir: dir, IsPrimary: primary,
	}
}

var testNamespaces = []Namespace{
	{Name: "personal", Roots: []string{"/home/u/personal"}},
	{Name: "work", Roots: []string{"/home/u/work"}},
}

func TestMatchNamespaceLongestRootWins(t *testing.T) {
	nss := []Namespace{
		{Name: "work", Roots: []string{"/home/u/work"}},
		{Name: "clients", Roots: []string{"/home/u/work/clients"}},
	}
	cases := []struct {
		path, want string
	}{
		{"/home/u/work/api", "work"},
		{"/home/u/work/clients/acme", "clients"}, // nested root is more specific
		{"/home/u/personal/blog", ""},            // under no root
		{"/home/u/worktree", ""},                 // segment-aware: not under /home/u/work
	}
	for _, c := range cases {
		if got := matchNamespace(c.path, nss); got != c.want {
			t.Errorf("matchNamespace(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestNamespaceMembershipPrefersPrimaryPath(t *testing.T) {
	rows := []Row{
		// A linked worktree outside the root, seen before the primary; the primary under the
		// root must still decide the repo's namespace.
		nsRow("w1", "api", "g2", "/tmp/api-feature", false, StatusWorking),
		nsRow("w2", "api", "g2", "/home/u/work/api", true, StatusIdle),
		nsRow("p1", "blog", "g1", "/home/u/personal/blog", true, StatusNeedsAttention),
		nsRow("x", "", "", "/somewhere", false, StatusIdle), // incidental: no repo
	}
	m := namespaceMembership(rows, testNamespaces)
	if m["g2"] != "work" {
		t.Errorf("g2 mapped to %q, want work (primary path under work root)", m["g2"])
	}
	if m["g1"] != "personal" {
		t.Errorf("g1 mapped to %q, want personal", m["g1"])
	}
	if _, ok := m[""]; ok {
		t.Errorf("incidental row (empty GitDir) should not appear in membership")
	}
}

// nsModel builds a model over fixed namespace-tagged rows with the given active tab key
// ("" = All, a configured name, or an automatic workspace key).
func nsModel(t *testing.T, activeTab string, rows []Row) model {
	t.Helper()
	m, err := newModel(fixedRows{rows}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.namespaces = testNamespaces
	m.width, m.height = 120, 40
	m.activeTab = activeTab
	m.setRows(m.allRows)
	return m
}

func testRows() []Row {
	return []Row{
		nsRow("p1", "blog", "g1", "/home/u/personal/blog", true, StatusNeedsAttention),
		nsRow("w1", "api", "g2", "/home/u/work/api", true, StatusWorking),
		nsRow("x", "", "", "/tmp/scratch", false, StatusIdle), // incidental
	}
}

func TestNamespaceFilterAppliesToBothLenses(t *testing.T) {
	rows := testRows()

	all := nsModel(t, "", rows)
	if len(all.rows) != 3 {
		t.Fatalf("All should pass every row, got %d", len(all.rows))
	}

	personal := nsModel(t, "personal", rows)
	if got := sessionIDs(personal.rows); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("personal Projects lens = %v, want [p1]", got)
	}
	if got := sessionIDs(personal.agentRows); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("personal Agents lens = %v, want [p1] (both lenses filtered consistently)", got)
	}

	work := nsModel(t, "work", rows)
	if got := sessionIDs(work.rows); len(got) != 1 || got[0] != "w1" {
		t.Fatalf("work Projects lens = %v, want [w1]", got)
	}
}

func TestIncidentalAgentsOnlyUnderAll(t *testing.T) {
	rows := testRows()
	all := nsModel(t, "", rows)
	if !containsID(all.rows, "x") {
		t.Fatalf("All should include the incidental agent x")
	}
	for _, active := range []string{"personal", "work"} {
		m := nsModel(t, active, rows)
		if containsID(m.rows, "x") {
			t.Fatalf("namespace %q should not include incidental agent x", active)
		}
	}
}

// TestAutoWorkspaceGroupsUnmatchedRepos checks that a repository under no configured root forms a
// parent-derived workspace tab (labelled by its parent dir), that siblings share it, and that
// filtering to it keeps only its repos.
func TestAutoWorkspaceGroupsUnmatchedRepos(t *testing.T) {
	rows := []Row{
		nsRow("p1", "blog", "g1", "/home/u/personal/blog", true, StatusIdle),
		// two unmatched repos sharing a parent (~/experiments) → one auto workspace
		nsRow("e1", "foo", "g3", "/home/u/experiments/foo", true, StatusNeedsAttention),
		nsRow("e2", "bar", "g4", "/home/u/experiments/bar", true, StatusWorking),
	}
	all := nsModel(t, "", rows)

	// The tab list is All, personal, work (configured), then the "experiments" auto tab.
	labels := tabLabelsOf(all.tabList())
	if got := strings.Join(labels, ","); got != "All,personal,work,experiments" {
		t.Fatalf("tab labels = %q, want All,personal,work,experiments", got)
	}
	// The experiments auto tab is urgent (e1 needs attention) and keyed by parent path.
	var expKey string
	for _, ti := range all.tabList() {
		if ti.label == "experiments" {
			expKey = ti.key
			if !ti.urgent {
				t.Fatalf("experiments tab should carry the urgency dot (e1 needs attention)")
			}
		}
	}
	if expKey == "" {
		t.Fatalf("no experiments auto tab found")
	}

	exp := nsModel(t, expKey, rows)
	if got := sessionIDs(exp.rows); len(got) != 2 || !containsID(exp.rows, "e1") || !containsID(exp.rows, "e2") {
		t.Fatalf("experiments tab = %v, want the two sibling repos e1,e2", got)
	}
}

// TestActiveAutoTabFallsBackToAllWhenEmptied checks that when the last repo of the active auto
// workspace leaves, the next refresh drops the user back to All.
func TestActiveAutoTabFallsBackToAllWhenEmptied(t *testing.T) {
	with := []Row{
		nsRow("p1", "blog", "g1", "/home/u/personal/blog", true, StatusIdle),
		nsRow("e1", "foo", "g3", "/home/u/experiments/foo", true, StatusWorking),
	}
	m := nsModel(t, "", with)
	// Land on the experiments auto tab.
	var expKey string
	for _, ti := range m.tabList() {
		if ti.label == "experiments" {
			expKey = ti.key
		}
	}
	m.activeTab = expKey
	m.setRows(with)
	if m.activeTab != expKey {
		t.Fatalf("precondition: should be on experiments tab")
	}
	// The experiments repo leaves; only personal remains.
	m.setRows([]Row{nsRow("p1", "blog", "g1", "/home/u/personal/blog", true, StatusIdle)})
	if m.activeTab != "" {
		t.Fatalf("an emptied auto tab should fall back to All, got %q", m.activeTab)
	}
}

func TestSwitchNamespaceWraps(t *testing.T) {
	m := nsModel(t, "", testRows())
	m.switchNamespace(-1) // All -> last (work; no auto tabs in testRows)
	if m.activeTab != "work" {
		t.Fatalf("prev from All should wrap to work, got %q", m.activeTab)
	}
	m.switchNamespace(1) // work -> All
	if m.activeTab != "" {
		t.Fatalf("next from work should wrap to All, got %q", m.activeTab)
	}
}

func TestTabBarUrgencyDotAndActiveMarker(t *testing.T) {
	// Active tab = work; personal holds a needs-attention agent, so its (non-active) tab dots.
	m := nsModel(t, "work", testRows())
	bar := m.tabBar()
	if !strings.Contains(bar, "[work]") {
		t.Fatalf("active tab should be bracketed: %q", bar)
	}
	dot := statusGlyph[StatusNeedsAttention]
	if !strings.Contains(stripANSI(bar), "personal"+dot) {
		t.Fatalf("personal (non-active, has needs-attention) should carry the urgency dot: %q", stripANSI(bar))
	}
}

func TestTabBarInertWithoutNamespaces(t *testing.T) {
	m, err := newModel(fixedRows{testRows()}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 120, 40
	if m.tabBarLines() != 0 || m.tabBar() != "" {
		t.Fatalf("with no namespaces the tab bar must be absent (inert)")
	}
}

func tabLabelsOf(tabs []tabInfo) []string {
	out := make([]string, len(tabs))
	for i, t := range tabs {
		out[i] = t.label
	}
	return out
}

func sessionIDs(rows []Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.SessionID
	}
	return out
}

func containsID(rows []Row, id string) bool {
	for _, r := range rows {
		if r.SessionID == id {
			return true
		}
	}
	return false
}
