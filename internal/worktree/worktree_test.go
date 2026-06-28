package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jbarap/birdseye/internal/providers/dir"
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
	featDir, _, err := Add(repo, "feat", "")
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
	loginDir, _, err := Add(repo, "feature/login", "")
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
	exDir, _, err := Add(repo, "existing", "")
	if err != nil {
		t.Fatalf("add existing: %v", err)
	}
	if got := branchOf(t, exDir); got != "existing" {
		t.Fatalf("worktree on branch %q, want existing", got)
	}

	// a slug collision is refused rather than overwritten.
	if _, _, err := Add(repo, "feat", ""); err == nil {
		t.Fatal("expected a slug collision to error")
	}

	// add outside a git worktree errors clearly.
	if _, _, err := Add(t.TempDir(), "nope", ""); err == nil {
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

	got, _, err := Add(stray, "x", "")
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

	// alpha: a clone with a grouped-sibling worktree — still reported once, at its primary.
	alpha := filepath.Join(root, "alpha")
	gitInit(t, alpha, "main")
	gitCommit(t, alpha)
	mustGit(t, alpha, "worktree", "add", filepath.Join(root, "alpha.worktrees", "feat"))

	// solo: a clone with no extra worktrees.
	solo := filepath.Join(root, "solo")
	gitInit(t, solo, "main")
	gitCommit(t, solo)

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected one entry per repo (alpha, solo), got %+v", got)
	}
	byRepo := map[string]Managed{}
	for _, m := range got {
		byRepo[m.Repo] = m
	}
	if m, ok := byRepo["alpha"]; !ok || filepath.Base(m.Path) != "alpha" {
		t.Fatalf("expected alpha reported once at its primary worktree (base alpha, not feat), got %+v", byRepo["alpha"])
	}
	if m, ok := byRepo["solo"]; !ok || filepath.Base(m.Path) != "solo" {
		t.Fatalf("expected solo reported once at its primary worktree, got %+v", byRepo["solo"])
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
	if len(cands) != 1 {
		t.Fatalf("expected one candidate per repo, got %d", len(cands))
	}
	// The candidate is named by the repository's home session (<repo>-<hash>), the same name
	// the dash's `n` and `be agents spawn` use, so opening here and spawning land in one session.
	gitDir, ok := commonDirOf(alpha)
	if !ok {
		t.Fatal("alpha should be a git worktree")
	}
	if want := dir.HomeSession(gitDir); cands[0].Name != want || cands[0].Label != "alpha" {
		t.Fatalf("expected alpha candidate named %q with label alpha, got %+v", want, cands[0])
	}
	if cands[0].Type != Type {
		t.Fatalf("expected candidate type %q, got %q", Type, cands[0].Type)
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
	cleanDir, _, err := Add(src, "clean", "")
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
	dirtyDir, _, err := Add(src, "dirty", "")
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

func writeAt(t *testing.T, base, rel, content string) {
	t.Helper()
	p := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// TestCopyIncludes pins the .worktreeinclude guardrail: a file is carried into the new
// worktree only when it both matches an include pattern and is git-ignored. A matched
// git-ignored file (including a nested one) is copied preserving its relative path; an
// unmatched git-ignored file and a tracked file are never copied.
func TestCopyIncludes(t *testing.T) {
	requireGitBinary(t)
	primary := t.TempDir()
	gitInit(t, primary, "main")
	gitCommit(t, primary)
	writeAt(t, primary, ".gitignore", ".env\nlocal/\nbuild/\n")
	writeAt(t, primary, ".worktreeinclude", ".env\nlocal/\ntracked.txt\n")
	writeAt(t, primary, ".env", "SECRET=1\n")
	writeAt(t, primary, "local/cfg.json", "{}\n")
	writeAt(t, primary, "build/out.o", "junk\n")
	writeAt(t, primary, "tracked.txt", "tracked\n")
	mustGit(t, primary, "add", "tracked.txt")
	mustGit(t, primary, "commit", "-m", "add tracked")

	dst := t.TempDir()
	if w := CopyIncludes(primary, dst); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}

	if got, err := os.ReadFile(filepath.Join(dst, ".env")); err != nil || string(got) != "SECRET=1\n" {
		t.Errorf("matched git-ignored .env should be copied verbatim, got %q err %v", got, err)
	}
	if !exists(filepath.Join(dst, "local", "cfg.json")) {
		t.Error("nested local/cfg.json should be copied preserving its relative path")
	}
	if exists(filepath.Join(dst, "build", "out.o")) {
		t.Error("an unmatched git-ignored file must not be copied")
	}
	if exists(filepath.Join(dst, "tracked.txt")) {
		t.Error("a tracked file must never be copied even when an include pattern matches it")
	}
}

// TestCopyIncludesAbsentIsNoop pins that a repository without a .worktreeinclude provisions
// nothing and reports no warning.
func TestCopyIncludesAbsentIsNoop(t *testing.T) {
	requireGitBinary(t)
	primary := t.TempDir()
	gitInit(t, primary, "main")
	gitCommit(t, primary)
	if w := CopyIncludes(primary, t.TempDir()); w != nil {
		t.Fatalf("expected a no-op with no .worktreeinclude, got %v", w)
	}
}

// TestSetupEnv pins the setup command's env contract: the new worktree, the copy-source
// repository, and the branch, as KEY=VALUE assignments.
func TestSetupEnv(t *testing.T) {
	got := SetupEnv("/wt", "/repo", "feat")
	want := []string{"BIRDSEYE_WORKTREE=/wt", "BIRDSEYE_REPO=/repo", "BIRDSEYE_BRANCH=feat"}
	if len(got) != len(want) {
		t.Fatalf("SetupEnv = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SetupEnv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
