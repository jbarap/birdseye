package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAddGroupedSibling exercises the grouped-sibling creation policy and branch
// handling over real git: a worktree lands at <repo>.worktrees/<branch-slug>, the
// container is derived from git (not the checkout's parent), and branch handling covers
// new / existing / slug-collision cases.
func TestAddGroupedSibling(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "myrepo")
	gitInit(t, repo, "trunk")
	gitCommit(t, repo)
	mustGit(t, repo, "branch", "existing")

	// add with no branch creates a new branch off the default branch, in a slug dir.
	featDir, err := Add(repo, "feat", "")
	if err != nil {
		t.Fatalf("add feat: %v", err)
	}
	if want := filepath.Join(root, "myrepo.worktrees", "feat"); featDir != want {
		t.Fatalf("feat worktree at %q, want %q", featDir, want)
	}
	if got := branchOf(t, featDir); got != "feat" {
		t.Fatalf("feat worktree on branch %q, want feat", got)
	}

	// a slashed branch slugifies into the directory name but keeps the branch as given.
	loginDir, err := Add(repo, "feature/login", "")
	if err != nil {
		t.Fatalf("add feature/login: %v", err)
	}
	if want := filepath.Join(root, "myrepo.worktrees", "feature-login"); loginDir != want {
		t.Fatalf("login worktree at %q, want %q", loginDir, want)
	}
	if got := branchOf(t, loginDir); got != "feature/login" {
		t.Fatalf("login worktree on branch %q, want feature/login", got)
	}

	// a name matching an existing branch checks that branch out (no new branch).
	exDir, err := Add(repo, "existing", "")
	if err != nil {
		t.Fatalf("add existing: %v", err)
	}
	if got := branchOf(t, exDir); got != "existing" {
		t.Fatalf("worktree on branch %q, want existing", got)
	}

	// a slug collision is refused rather than overwritten.
	if _, err := Add(repo, "feat", ""); err == nil {
		t.Fatal("expected a slug collision to error")
	}

	// add outside a git worktree errors clearly.
	if _, err := Add(t.TempDir(), "nope", ""); err == nil {
		t.Fatal("expected add outside a repo to error")
	}
}

// TestAddContainerFromGitNotCheckoutParent confirms the container is resolved from the
// repository's shared git dir, so `add` from a worktree placed off in some other
// directory still lands the new worktree under <repo>.worktrees/.
func TestAddContainerFromGitNotCheckoutParent(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "myrepo")
	gitInit(t, repo, "main")
	gitCommit(t, repo)

	// A worktree placed somewhere unrelated to <repo>.worktrees/.
	stray := filepath.Join(t.TempDir(), "stray")
	mustGit(t, repo, "worktree", "add", stray)

	got, err := Add(stray, "x", "")
	if err != nil {
		t.Fatalf("add from stray worktree: %v", err)
	}
	if want := filepath.Join(root, "myrepo.worktrees", "x"); got != want {
		t.Fatalf("add resolved container from the checkout's parent (%q), want git-derived %q", got, want)
	}
}

// TestListGitNative checks that discovery finds git repos directly under a root and
// enumerates each repo's worktrees from git, wherever they live on disk.
func TestListGitNative(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()

	// alpha: a clone with a grouped-sibling worktree.
	alpha := filepath.Join(root, "alpha")
	gitInit(t, alpha, "main")
	gitCommit(t, alpha)
	mustGit(t, alpha, "worktree", "add", filepath.Join(root, "alpha.worktrees", "feat"))

	// solo: a clone with no extra worktrees — still a valid managed repo.
	solo := filepath.Join(root, "solo")
	gitInit(t, solo, "main")
	gitCommit(t, solo)

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	byRepo := map[string]Managed{}
	for _, m := range got {
		byRepo[m.Repo] = m
	}
	if m, ok := byRepo["alpha"]; !ok || len(m.Dirs) != 2 {
		t.Fatalf("expected alpha with 2 worktrees (primary + feat), got %+v (all=%+v)", byRepo["alpha"], got)
	}
	if m, ok := byRepo["solo"]; !ok || len(m.Dirs) != 1 || m.Dirs[0].Name != "solo" {
		t.Fatalf("expected solo as a single-worktree managed repo, got %+v", byRepo["solo"])
	}
}

func TestListNoRootsIsEmpty(t *testing.T) {
	got, err := List()
	if err != nil || got != nil {
		t.Fatalf("no roots should yield no managed repos, got %+v err=%v", got, err)
	}
}

// TestListWorktreesPrimaryFlag checks the porcelain parse marks git's main worktree as
// primary and reports the linked one as non-primary.
func TestListWorktreesPrimaryFlag(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	gitInit(t, repo, "main")
	gitCommit(t, repo)
	mustGit(t, repo, "worktree", "add", filepath.Join(root, "repo.worktrees", "feat"))

	wts, err := ListWorktrees(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %+v", wts)
	}
	if !wts[0].IsPrimary {
		t.Fatalf("first worktree should be primary, got %+v", wts[0])
	}
	for _, w := range wts[1:] {
		if w.IsPrimary {
			t.Fatalf("only the first worktree should be primary, got %+v", w)
		}
	}
}

func TestProviderCandidatesFromRoots(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gitInit(t, alpha, "main")
	gitCommit(t, alpha)
	mustGit(t, alpha, "worktree", "add", filepath.Join(root, "alpha.worktrees", "wt1"))

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
	if !containsStr(names, "alpha-wt1") {
		t.Fatalf("expected an alpha-wt1 candidate, got %v", names)
	}
}

func TestProviderNoRootsYieldsNothing(t *testing.T) {
	cands, err := NewProvider(nil).Candidates()
	if err != nil || cands != nil {
		t.Fatalf("no roots should yield no candidates, got %+v err=%v", cands, err)
	}
}

// TestRemoveCleanAndDirty checks the dirty-guard: a clean worktree removes, a dirty one
// is refused without force and removed with it.
func TestRemoveCleanAndDirty(t *testing.T) {
	requireGitBinary(t)
	root := t.TempDir()
	src := filepath.Join(root, "src")
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
