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
	m.wsEnabled = true
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

// TestDirRowFilesUnderWorkspaceByPath checks that a non-git directory session under a configured
// workspace root appears in that workspace (not only under All), while one under no root forms an
// automatic parent-derived tab rather than being confined to All.
func TestDirRowFilesUnderWorkspaceByPath(t *testing.T) {
	dirRow := func(id, dir string) Row {
		return Row{Kind: RowDir, SessionID: "dir:" + id, Title: id, TmuxSession: id, Dir: dir}
	}
	rows := []Row{
		nsRow("w1", "api", "g2", "/home/u/work/api", true, StatusWorking),
		dirRow("data", "/home/u/work/datasets"),     // under the work root
		dirRow("scratch", "/tmp/scratch/experiment"), // under no configured root
	}

	work := nsModel(t, "work", rows)
	if !containsID(work.rows, "dir:data") {
		t.Fatalf("a dir under the work root should appear in the work workspace, got %v", sessionIDs(work.rows))
	}
	if containsID(work.rows, "dir:scratch") {
		t.Fatalf("the unmatched dir should not appear in work, got %v", sessionIDs(work.rows))
	}

	// The unmatched dir forms an automatic tab (keyed by its parent), so it is reachable off All.
	m := nsModel(t, "", rows)
	var autoKey string
	for _, tab := range m.tabList() {
		if tab.label == "scratch" {
			autoKey = tab.key
		}
	}
	if autoKey == "" {
		t.Fatalf("the unmatched dir should form an automatic 'scratch' tab, tabs=%+v", m.tabList())
	}
	auto := nsModel(t, autoKey, rows)
	if !containsID(auto.rows, "dir:scratch") {
		t.Fatalf("the automatic tab should hold the unmatched dir, got %v", sessionIDs(auto.rows))
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

// TestDisabledIsInertEvenWithNamespaces checks the master switch: with the feature disabled the
// dash renders no tab bar, forms no automatic workspaces, and does not filter, even when namespaces
// are declared and unmatched repositories are present.
func TestDisabledIsInertEvenWithNamespaces(t *testing.T) {
	rows := []Row{
		nsRow("p1", "blog", "g1", "/home/u/personal/blog", true, StatusIdle),
		nsRow("e1", "foo", "g9", "/home/u/experiments/foo", true, StatusIdle),
	}
	m, err := newModel(fixedRows{rows}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.wsEnabled = false
	m.namespaces = testNamespaces
	m.width, m.height = 120, 40
	m.setRows(m.allRows)
	if m.tabBarLines() != 0 || m.tabBar() != "" {
		t.Fatalf("disabled feature must render no tab bar, got lines=%d bar=%q", m.tabBarLines(), m.tabBar())
	}
	if got := tabLabelsOf(m.tabList()); len(got) != 1 || got[0] != "All" {
		t.Fatalf("disabled feature must expose only All, got %v", got)
	}
	if m.tabAssign != nil {
		t.Fatalf("disabled feature must assign no repo tabs, got %v", m.tabAssign)
	}
}

// TestEnabledFormsAutoTabsWithNoConfig checks that with the feature on and zero declared namespaces,
// repositories under distinct parent directories each form an automatic parent-derived tab, so
// grouping appears with no configuration.
func TestEnabledFormsAutoTabsWithNoConfig(t *testing.T) {
	rows := []Row{
		nsRow("a1", "blog", "g1", "/home/u/personal/blog", true, StatusIdle),
		nsRow("a2", "api", "g2", "/home/u/work/api", true, StatusIdle),
	}
	m, err := newModel(fixedRows{rows}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.wsEnabled = true // no m.namespaces
	m.width, m.height = 120, 40
	m.setRows(m.allRows)
	labels := tabLabelsOf(m.tabList())
	want := []string{"All", "personal", "work"} // auto tabs labelled by parent dir, sorted
	if strings.Join(labels, ",") != strings.Join(want, ",") {
		t.Fatalf("tab labels = %v, want %v", labels, want)
	}
	if m.tabBarLines() != 1 {
		t.Fatalf("more than one tab must show the bar, got %d lines", m.tabBarLines())
	}
}

// TestEnabledHidesBarWhenAllIsOnlyTab checks that enabling the feature adds no chrome when there is
// nothing to filter: with no recognized repository (only an incidental agent), the sole tab is All,
// so no bar renders. Two repos under one parent still form one auto tab (All + parent = two tabs, a
// real choice) - the no-bar case is strictly "no repository forms a second tab".
func TestEnabledHidesBarWhenAllIsOnlyTab(t *testing.T) {
	// A repository under a single parent still yields a second tab, so the bar shows.
	twoRepos := []Row{
		nsRow("a1", "blog", "g1", "/home/u/personal/blog", true, StatusIdle),
		nsRow("a2", "diary", "g2", "/home/u/personal/diary", true, StatusIdle),
	}
	m, err := newModel(fixedRows{twoRepos}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.wsEnabled = true // no m.namespaces; both repos share one parent -> one auto tab
	m.width, m.height = 120, 40
	m.setRows(m.allRows)
	if got := tabLabelsOf(m.tabList()); len(got) != 2 {
		t.Fatalf("one parent should yield All + one auto tab, got %v", got)
	}
	if m.tabBarLines() != 1 {
		t.Fatalf("All + one auto tab is a real choice, bar should show, got %d lines", m.tabBarLines())
	}

	// No recognized repository: only an incidental agent maps to no tab, so All is the only tab.
	incidental := []Row{nsRow("x", "", "", "/somewhere", false, StatusIdle)}
	m2, err := newModel(fixedRows{incidental}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m2.wsEnabled = true
	m2.width, m2.height = 120, 40
	m2.setRows(m2.allRows)
	if got := tabLabelsOf(m2.tabList()); len(got) != 1 || got[0] != "All" {
		t.Fatalf("no repository should yield only All, got %v", got)
	}
	if m2.tabBarLines() != 0 || m2.tabBar() != "" {
		t.Fatalf("All-only must render no bar, got lines=%d bar=%q", m2.tabBarLines(), m2.tabBar())
	}
}

// TestDashStatePersistRoundTrip checks that the active-tab key survives a write/read cycle for
// every kind of key: All (""), a configured name, and an automatic workspace key (which embeds a
// NUL byte), and that a missing file reads as the zero state (All).
func TestDashStatePersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if got, err := readDashState(dir); err != nil || got.ActiveWorkspace != "" {
		t.Fatalf("missing file should read as All: got %+v err %v", got, err)
	}
	for _, key := range []string{"", "work", autoPrefix + "/home/u/experiments"} {
		if err := writeDashState(dir, dashState{ActiveWorkspace: key}); err != nil {
			t.Fatalf("writeDashState(%q): %v", key, err)
		}
		got, err := readDashState(dir)
		if err != nil {
			t.Fatalf("readDashState after writing %q: %v", key, err)
		}
		if got.ActiveWorkspace != key {
			t.Fatalf("round-trip mismatch: wrote %q, read %q", key, got.ActiveWorkspace)
		}
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
