// Package worktree manages a repo-per-folder layout: a repository is cloned to
// <root>/<repo>/main, and sibling git worktrees live alongside it as
// <root>/<repo>/<name>. The worktrees are surfaced to the picker as session
// candidates.
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

// Managed is a repository under the managed root and the worktrees it contains.
type Managed struct {
	Repo string // repository folder name
	Dirs []Dir  // "main" plus sibling worktrees
}

// Dir is one worktree directory.
type Dir struct {
	Name string // e.g. "main", "wt1"
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

// Clone clones url into <root>/<repo>/main. If that directory already exists it
// does not re-clone; the returned bool reports whether it already existed.
func Clone(root, url string) (mainDir string, existed bool, err error) {
	if err := requireGit(); err != nil {
		return "", false, err
	}
	repo := RepoNameFromURL(url)
	if repo == "" {
		return "", false, fmt.Errorf("could not derive repo name from %q", url)
	}
	mainDir = filepath.Join(root, repo, "main")
	if _, statErr := os.Stat(mainDir); statErr == nil {
		return mainDir, true, nil
	}
	if err := os.MkdirAll(filepath.Dir(mainDir), 0o755); err != nil {
		return "", false, err
	}
	if err := run("", "git", "clone", url, mainDir); err != nil {
		return "", false, err
	}
	return mainDir, false, nil
}

// Add creates a sibling worktree <root>/<repo>/<name>. branch may be empty (let
// git decide) or an existing ref. It declines to overwrite an existing
// worktree directory.
func Add(root, repo, name, branch string) (dir string, err error) {
	if err := requireGit(); err != nil {
		return "", err
	}
	mainDir := filepath.Join(root, repo, "main")
	if _, statErr := os.Stat(mainDir); statErr != nil {
		return "", fmt.Errorf("no managed repo at %s; clone it first", mainDir)
	}
	dir = filepath.Join(root, repo, name)
	if _, statErr := os.Stat(dir); statErr == nil {
		return "", fmt.Errorf("worktree %q already exists at %s", name, dir)
	}
	args := []string{"worktree", "add", dir}
	if branch != "" {
		args = append(args, branch)
	}
	if err := run(mainDir, "git", args...); err != nil {
		return "", err
	}
	return dir, nil
}

// List returns the managed repositories under root and their worktree dirs. A
// managed repo is any immediate child of root that contains a "main" checkout.
func List(root string) ([]Managed, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Managed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoDir := filepath.Join(root, e.Name())
		if _, statErr := os.Stat(filepath.Join(repoDir, "main")); statErr != nil {
			continue // not a managed repo
		}
		sub, err := os.ReadDir(repoDir)
		if err != nil {
			continue
		}
		m := Managed{Repo: e.Name()}
		for _, s := range sub {
			if s.IsDir() {
				m.Dirs = append(m.Dirs, Dir{Name: s.Name(), Path: filepath.Join(repoDir, s.Name())})
			}
		}
		out = append(out, m)
	}
	return out, nil
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
