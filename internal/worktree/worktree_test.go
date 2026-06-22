package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepoNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/u/birds-eye.git": "birds-eye",
		"https://github.com/u/birds-eye":     "birds-eye",
		"git@github.com:u/foo.git":           "foo",
		"git@github.com:u/foo":               "foo",
		"/local/path/bar/":                   "bar",
	}
	for url, want := range cases {
		if got := RepoNameFromURL(url); got != want {
			t.Errorf("RepoNameFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}

// TestListRecognizesManagedReposByAnchor checks the recognition rule: a managed repo
// is recognized by its anchor — a child directory named after the repo's default
// branch (<repo>/main). A single anchored checkout is valid even with no siblings,
// while a folder of unrelated clones, and a folder holding a project-named clone, must
// NOT be read as worktrees — the regression that turned an ordinary projects directory
// into bogus worktrees.
func TestListRecognizesManagedReposByAnchor(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()

	// A genuine worktree set: alpha/main (anchor) + alpha/feat (linked worktree).
	alphaMain := filepath.Join(root, "alpha", "main")
	gitInit(t, alphaMain, "main")
	gitCommit(t, alphaMain)
	mustGit(t, alphaMain, "worktree", "add", filepath.Join(root, "alpha", "feat"))

	// A single-worktree managed repo: <repo>/main with no siblings is still valid —
	// the default-branch checkout is its own anchor.
	soloMain := filepath.Join(root, "gizmo", "main")
	gitInit(t, soloMain, "main")
	gitCommit(t, soloMain)

	// A folder of unrelated clones side by side — no anchor, not one repo's worktrees.
	for _, n := range []string{"red", "blue"} {
		d := filepath.Join(root, "vendor", n)
		gitInit(t, d, "main")
		gitCommit(t, d)
	}

	// A folder holding a single project-named clone — the dir name (tax) is not the
	// repo's default branch (main), so there is no anchor.
	tax := filepath.Join(root, "docs", "tax")
	gitInit(t, tax, "main")
	gitCommit(t, tax)

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	byRepo := map[string]Managed{}
	for _, m := range got {
		byRepo[m.Repo] = m
	}
	if m, ok := byRepo["alpha"]; !ok || len(m.Dirs) != 2 {
		t.Fatalf("expected alpha with 2 worktrees, got %+v (all=%+v)", byRepo["alpha"], got)
	}
	if m, ok := byRepo["gizmo"]; !ok || len(m.Dirs) != 1 || m.Dirs[0].Name != "main" {
		t.Fatalf("expected gizmo/main (a single anchored worktree), got %+v", byRepo["gizmo"])
	}
	if _, ok := byRepo["vendor"]; ok {
		t.Fatal("a folder of unrelated clones must not be listed as a worktree set")
	}
	if _, ok := byRepo["docs"]; ok {
		t.Fatal("a folder holding a project-named clone must not be listed")
	}
}

func TestListNoRootsIsEmpty(t *testing.T) {
	got, err := List()
	if err != nil || got != nil {
		t.Fatalf("no roots should yield no managed repos, got %+v err=%v", got, err)
	}
}

func TestProviderCandidatesFromRoots(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()
	alphaMain := filepath.Join(root, "alpha", "main")
	gitInit(t, alphaMain, "main")
	gitCommit(t, alphaMain)
	mustGit(t, alphaMain, "worktree", "add", filepath.Join(root, "alpha", "wt1"))

	cands, err := NewProvider([]string{root}).Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}
	var names []string
	for _, c := range cands {
		names = append(names, c.Name)
	}
	if !containsStr(names, "alpha-main") {
		t.Fatalf("expected an alpha-main candidate, got %v", names)
	}
}

func TestProviderNoRootsYieldsNothing(t *testing.T) {
	cands, err := NewProvider(nil).Candidates()
	if err != nil || cands != nil {
		t.Fatalf("no roots should yield no candidates, got %+v err=%v", cands, err)
	}
}

// TestCloneAddDefaultBranch is an end-to-end check over real git: it clones a source
// repo whose default branch is "trunk" (not main), verifies the layout, and exercises
// add's branch semantics (new branch, existing branch, duplicate).
func TestCloneAddDefaultBranch(t *testing.T) {
	requireGitBinary(t)

	// A source repo with default branch "trunk" and an extra branch "existing".
	srcParent := t.TempDir()
	src := filepath.Join(srcParent, "myrepo")
	gitInit(t, src, "trunk")
	gitCommit(t, src)
	mustGit(t, src, "branch", "existing")

	// Clone places it at <parent>/myrepo/trunk.
	parent := t.TempDir()
	dir, existed, err := Clone(parent, src)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	want := filepath.Join(parent, "myrepo", "trunk")
	if dir != want || existed {
		t.Fatalf("clone dir=%q existed=%v, want %q existed=false", dir, existed, want)
	}
	if !isGitWorktree(dir) {
		t.Fatalf("cloned dir %q is not a git worktree", dir)
	}

	// Re-clone is a no-op reporting the existing layout.
	if d2, existed2, err := Clone(parent, src); err != nil || d2 != want || !existed2 {
		t.Fatalf("re-clone = (%q,%v,%v), want (%q,true,nil)", d2, existed2, err, want)
	}

	// defaultBranchOfRepo resolves origin/HEAD set up by clone.
	if got := defaultBranchOfRepo(dir); got != "trunk" {
		t.Fatalf("defaultBranchOfRepo = %q, want trunk", got)
	}

	// containerOf returns <parent>/myrepo and the worktree top level.
	container, top, err := containerOf(dir)
	if err != nil || container != filepath.Join(parent, "myrepo") || top != want {
		t.Fatalf("containerOf = (%q,%q,%v)", container, top, err)
	}

	// add with no branch creates a new branch "feat" off the default branch.
	featDir, err := Add(dir, "feat", "")
	if err != nil {
		t.Fatalf("add feat: %v", err)
	}
	if featDir != filepath.Join(parent, "myrepo", "feat") {
		t.Fatalf("feat worktree at %q", featDir)
	}
	if got := branchOf(t, featDir); got != "feat" {
		t.Fatalf("feat worktree on branch %q, want feat", got)
	}

	// add a name matching an existing branch checks that branch out (no new branch).
	exDir, err := Add(dir, "existing", "")
	if err != nil {
		t.Fatalf("add existing: %v", err)
	}
	if got := branchOf(t, exDir); got != "existing" {
		t.Fatalf("worktree on branch %q, want existing", got)
	}

	// add declines to overwrite an existing worktree dir.
	if _, err := Add(dir, "feat", ""); err == nil {
		t.Fatal("expected duplicate worktree to error")
	}

	// add outside a git worktree errors clearly.
	if _, err := Add(t.TempDir(), "nope", ""); err == nil {
		t.Fatal("expected add outside a repo to error")
	}
}

// TestRemoveCleanAndDirty checks the dirty-guard: a clean worktree removes, a dirty one
// is refused without force and removed with it.
func TestRemoveCleanAndDirty(t *testing.T) {
	requireGitBinary(t)
	parent := t.TempDir()
	src := filepath.Join(parent, "src")
	gitInit(t, src, "main")
	gitCommit(t, src)

	// A clean linked worktree removes without force.
	cleanDir, err := Add(src, "clean", "")
	if err != nil {
		t.Fatal(err)
	}
	if dirty, _ := IsDirty(cleanDir); dirty {
		t.Fatal("fresh worktree should be clean")
	}
	if err := Remove(cleanDir, false); err != nil {
		t.Fatalf("remove clean: %v", err)
	}
	if _, err := os.Stat(cleanDir); !os.IsNotExist(err) {
		t.Fatalf("clean worktree dir should be gone, stat err=%v", err)
	}

	// A dirty worktree is refused without force, removed with it.
	dirtyDir, err := Add(src, "dirty", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirtyDir, "scratch.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := IsDirty(dirtyDir); !dirty {
		t.Fatal("worktree with an untracked file should be dirty")
	}
	if err := Remove(dirtyDir, false); err == nil {
		t.Fatal("removing a dirty worktree without force should fail")
	}
	if err := Remove(dirtyDir, true); err != nil {
		t.Fatalf("force-remove dirty: %v", err)
	}
	if _, err := os.Stat(dirtyDir); !os.IsNotExist(err) {
		t.Fatalf("dirty worktree dir should be gone after force, stat err=%v", err)
	}
}

// --- helpers ---

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func requireGitBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func gitInit(t *testing.T, dir, branch string) {
	t.Helper()
	mustMkdir(t, dir)
	mustGit(t, dir, "init", "-b", branch)
	mustGit(t, dir, "config", "user.email", "test@example.com")
	mustGit(t, dir, "config", "user.name", "Test")
}

func gitCommit(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "-A")
	mustGit(t, dir, "commit", "-m", "init")
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := run(dir, "git", args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func branchOf(t *testing.T, dir string) string {
	t.Helper()
	out, err := output(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("branchOf %q: %v", dir, err)
	}
	return out
}
