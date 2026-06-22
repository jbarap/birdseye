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

// TestListFindsManagedRepos exercises the git-free recognition: a container is managed
// when it holds at least one git worktree child (a `.git` entry), regardless of the
// default-branch name, across multiple roots.
func TestListFindsManagedRepos(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	// "alpha" with master + wt1 (both worktrees); "loose" has only a plain dir (ignored).
	mustWorktreeDir(t, filepath.Join(rootA, "alpha", "master"))
	mustWorktreeDir(t, filepath.Join(rootA, "alpha", "wt1"))
	mustMkdir(t, filepath.Join(rootA, "loose", "stuff"))
	// A repo under a second root, with a non-"main" default branch dir.
	mustWorktreeDir(t, filepath.Join(rootB, "beta", "trunk"))

	got, err := List(rootA, rootB)
	if err != nil {
		t.Fatal(err)
	}
	byRepo := map[string]Managed{}
	for _, m := range got {
		byRepo[m.Repo] = m
	}
	if _, ok := byRepo["loose"]; ok {
		t.Fatal("a container with no git worktree child must not be listed")
	}
	if m, ok := byRepo["alpha"]; !ok || len(m.Dirs) != 2 {
		t.Fatalf("expected alpha with 2 worktree dirs, got %+v", byRepo["alpha"])
	}
	if m, ok := byRepo["beta"]; !ok || len(m.Dirs) != 1 || m.Dirs[0].Name != "trunk" {
		t.Fatalf("expected beta/trunk worktree, got %+v", byRepo["beta"])
	}
}

func TestListNoRootsIsEmpty(t *testing.T) {
	got, err := List()
	if err != nil || got != nil {
		t.Fatalf("no roots should yield no managed repos, got %+v err=%v", got, err)
	}
}

func TestProviderCandidatesFromRoots(t *testing.T) {
	root := t.TempDir()
	mustWorktreeDir(t, filepath.Join(root, "alpha", "main"))
	mustWorktreeDir(t, filepath.Join(root, "alpha", "wt1"))

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

// --- helpers ---

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

// mustWorktreeDir creates a directory marked as a git worktree (a `.git` entry),
// without standing up a real repo — enough for List's git-free recognition.
func mustWorktreeDir(t *testing.T, p string) {
	t.Helper()
	mustMkdir(t, p)
	if err := os.WriteFile(filepath.Join(p, ".git"), []byte("gitdir: /dev/null\n"), 0o644); err != nil {
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
