// Package worktree manages a repo-per-folder layout that can live anywhere on
// disk: a repository is cloned to <parent>/<repo>/<default-branch>, and sibling
// git worktrees live alongside it as <parent>/<repo>/<name>. The <repo>/ container
// is derived from git (the parent of the worktree's top level), so there is no
// mandatory common root. Worktrees discovered under optional configured roots are
// surfaced to the picker as session candidates.
package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrGitMissing is returned when git is required but not on PATH.
var ErrGitMissing = errors.New("git is required but was not found on PATH")

// Managed is a repository container and the worktrees it contains.
type Managed struct {
	Repo string // repository folder name (the <repo>/ container's base name)
	Dirs []Dir  // the default-branch checkout plus sibling worktrees
}

// Dir is one worktree directory.
type Dir struct {
	Name string // e.g. "main", "master", "feature-x"
	Path string // absolute path
}

// RepoNameFromURL derives the repository folder name from a clone URL.
func RepoNameFromURL(url string) string {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	base := url
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		base = url[i+1:]
	}
	return strings.TrimSuffix(base, ".git")
}

func requireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrGitMissing
	}
	return nil
}

// Clone clones url into <parent>/<repo>/<default-branch>, where <default-branch> is
// the repository's actual default branch resolved from the remote's HEAD (e.g. main,
// master, trunk). If that directory already exists it does not re-clone; the returned
// bool reports whether it already existed.
func Clone(parent, url string) (dir string, existed bool, err error) {
	if err := requireGit(); err != nil {
		return "", false, err
	}
	repo := RepoNameFromURL(url)
	if repo == "" {
		return "", false, fmt.Errorf("could not derive repo name from %q", url)
	}
	branch, err := defaultBranchFromRemote(url)
	if err != nil {
		return "", false, err
	}
	dir = filepath.Join(parent, repo, branch)
	if _, statErr := os.Stat(dir); statErr == nil {
		return dir, true, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", false, err
	}
	if err := run("", "git", "clone", url, dir); err != nil {
		return "", false, err
	}
	return dir, false, nil
}

// Add creates a sibling worktree <container>/<name>, inferring the <repo>/ container
// from cwd (the parent of the current worktree's top level), so it works from anywhere
// inside any worktree of the repo. Branch handling: an explicit branch is used as
// given; with no branch, a name matching an existing local/remote branch checks that
// branch out, otherwise a new branch named after the worktree is created off the
// repository's default branch. It declines to overwrite an existing directory.
func Add(cwd, name, branch string) (dir string, err error) {
	if err := requireGit(); err != nil {
		return "", err
	}
	container, top, err := containerOf(cwd)
	if err != nil {
		return "", err
	}
	dir = filepath.Join(container, name)
	if _, statErr := os.Stat(dir); statErr == nil {
		return "", fmt.Errorf("worktree %q already exists at %s", name, dir)
	}
	var args []string
	switch {
	case branch != "":
		// Explicit ref: let git check it out (DWIM-ing a remote branch if needed).
		args = []string{"worktree", "add", dir, branch}
	case branchExists(top, name):
		// Name matches an existing branch: check it out rather than failing.
		args = []string{"worktree", "add", dir, name}
	default:
		// New branch named after the worktree, based on the default branch.
		args = []string{"worktree", "add", "-b", name, dir}
		if def := defaultBranchOfRepo(top); def != "" {
			args = append(args, def)
		}
	}
	if err := run(top, "git", args...); err != nil {
		return "", err
	}
	return dir, nil
}

// List returns the managed repositories discovered under the given roots and their
// worktree dirs. A managed repo is any immediate child of a root that contains at
// least one git worktree (a child with a `.git` entry) — no common root and no
// hardcoded `main` name is assumed.
func List(roots ...string) ([]Managed, error) {
	var out []Managed
	for _, root := range roots {
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			repoDir := filepath.Join(root, e.Name())
			sub, err := os.ReadDir(repoDir)
			if err != nil {
				continue
			}
			var dirs []Dir
			for _, s := range sub {
				if !s.IsDir() {
					continue
				}
				p := filepath.Join(repoDir, s.Name())
				if isGitWorktree(p) {
					dirs = append(dirs, Dir{Name: s.Name(), Path: p})
				}
			}
			if len(dirs) == 0 {
				continue // not a managed repo container
			}
			out = append(out, Managed{Repo: e.Name(), Dirs: dirs})
		}
	}
	return out, nil
}

// IsDirty reports whether the worktree at dir has uncommitted changes or untracked
// files — the signal that removing it needs confirmation.
func IsDirty(dir string) (bool, error) {
	out, err := output(dir, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Remove removes the worktree at dir via `git worktree remove`. With force=false git
// declines a worktree that has uncommitted changes; force=true passes --force. The
// command runs from the repository's main worktree (resolved from the common git dir)
// so removing a linked worktree — even the caller's own — succeeds.
func Remove(dir string, force bool) error {
	if err := requireGit(); err != nil {
		return err
	}
	commonDir, err := output(dir, "git", "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	mainWorktree := filepath.Dir(commonDir)
	args := []string{"worktree", "remove", dir}
	if force {
		args = append(args, "--force")
	}
	return run(mainWorktree, "git", args...)
}

// Info describes the managed-repo context of a directory.
type Info struct {
	Container     string // the <repo>/ container (parent of the worktree top level)
	Repo          string // the container's base name
	DefaultBranch string // the repo's default branch (e.g. main, master)
	TopLevel      string // the directory's worktree top level
	Worktree      string // the worktree's name (base of TopLevel)
}

// Resolve returns the managed-repo context of dir, or ok=false when dir is not inside
// a git worktree. It is the git-backed classifier the agents view's reconciler uses.
func Resolve(dir string) (Info, bool) {
	container, top, err := containerOf(dir)
	if err != nil {
		return Info{}, false
	}
	return Info{
		Container:     container,
		Repo:          filepath.Base(container),
		DefaultBranch: defaultBranchOfRepo(top),
		TopLevel:      top,
		Worktree:      filepath.Base(top),
	}, true
}

// containerOf returns the <repo>/ container and the current worktree's top level for
// a starting directory: the container is the parent of `git rev-parse --show-toplevel`.
// It errors clearly when start is not inside a git worktree.
func containerOf(start string) (container, top string, err error) {
	top, err = output(start, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("not inside a git worktree; run `be worktree add` from within a managed repo")
	}
	return filepath.Dir(top), top, nil
}

// defaultBranchFromRemote resolves a clone URL's default branch from the remote's
// HEAD via `git ls-remote --symref` (no clone needed).
func defaultBranchFromRemote(url string) (string, error) {
	out, err := output("", "git", "ls-remote", "--symref", url, "HEAD")
	if err != nil {
		return "", err
	}
	// A symref line looks like: "ref: refs/heads/main\tHEAD".
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ref:" {
			return strings.TrimPrefix(fields[1], "refs/heads/"), nil
		}
	}
	return "", fmt.Errorf("could not determine default branch for %q", url)
}

// defaultBranchOfRepo resolves an existing local repo's default branch from
// origin/HEAD, falling back to the currently checked-out branch when no remote HEAD
// is known. Returns "" only when neither can be determined.
func defaultBranchOfRepo(dir string) string {
	if out, err := output(dir, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(out, "origin/")
	}
	if out, err := output(dir, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil && out != "HEAD" {
		return out
	}
	return ""
}

// branchExists reports whether name is an existing local or remote-tracking branch.
func branchExists(dir, name string) bool {
	for _, ref := range []string{"refs/heads/" + name, "refs/remotes/origin/" + name} {
		if err := run(dir, "git", "show-ref", "--verify", "--quiet", ref); err == nil {
			return true
		}
	}
	return false
}

// isGitWorktree reports whether path is a git worktree (its top level holds a `.git`
// directory for the primary checkout, or a `.git` file for a linked worktree).
func isGitWorktree(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return nil
}

// output runs a command in dir and returns its trimmed stdout, wrapping stderr in the
// error on failure.
func output(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}
