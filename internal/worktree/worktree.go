// Package worktree manages a grouped-sibling layout that can live anywhere on disk: a
// repository is a plain clone at <path>/<repo>, and its additional git worktrees live as
// grouped siblings under <path>/<repo>.worktrees/<branch-slug>. Recognition is git-native
// — a repository and its worktrees are derived from `git worktree list` and the shared git
// directory (`git rev-parse --git-common-dir`), not from any path convention — so any
// on-disk layout is recognized, including worktrees a user or another tool created.
// Repositories discovered under optional configured roots are surfaced to the picker as
// one session candidate each (opening a repo lands on its primary worktree); the repo's
// linked worktrees are managed as windows inside that session by the agents view, not as
// separate top-level candidates.
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

// Managed is a discovered repository: its name and the primary worktree to open into.
type Managed struct {
	Repo   string // repository folder name (the primary worktree's base name)
	Path   string // absolute path of the primary worktree - where opening the repo lands
	GitDir string // git common dir - the repository's identity, used to name its home session
}

// Worktree is one entry in a repository's git worktree set.
type Worktree struct {
	Path      string // absolute, symlink-resolved worktree path
	Branch    string // short branch name; empty if detached or bare
	Head      string // commit the worktree is checked out at
	IsPrimary bool   // git's primary (main) worktree — the one `git worktree remove` refuses
}

func requireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrGitMissing
	}
	return nil
}

// ListWorktrees enumerates the worktrees of the repository containing dir via
// `git worktree list --porcelain`. The first worktree git reports is the primary
// worktree. Paths are absolute and symlink-resolved so they compare equal to the start
// paths the reconciler derives from git.
func ListWorktrees(dir string) ([]Worktree, error) {
	if err := requireGit(); err != nil {
		return nil, err
	}
	out, err := output(dir, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var wts []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			wts = append(wts, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			p := strings.TrimPrefix(line, "worktree ")
			if resolved, err := filepath.EvalSymlinks(p); err == nil {
				p = resolved
			}
			cur = &Worktree{Path: filepath.Clean(p), IsPrimary: len(wts) == 0}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	flush()
	return wts, nil
}

// Add creates a grouped-sibling worktree under <repo>.worktrees/<branch-slug>, resolving
// the repository (and its primary worktree) from git's shared git dir, so it works from
// anywhere inside any worktree of the repo. Branch handling: branch defaults to name when
// empty; the directory name is the filesystem-safe slug of the branch (`feature/login` →
// `feature-login`). A branch that already exists (local or remote-tracking) is checked
// out; otherwise a new branch of that name is created off the repository's default
// branch. It declines to overwrite an existing directory (a slug collision).
//
// On success Add also provisions the new worktree by carrying over the repository's
// .worktreeinclude files (see CopyIncludes), so both callers - the agentless CLI and the
// agent spawn path - get the copy step without duplicating it. Any per-file copy failure is
// returned as a non-fatal warning rather than aborting; the worktree is already usable.
func Add(cwd, name, branch string) (dir string, warnings []string, err error) {
	if err := requireGit(); err != nil {
		return "", nil, err
	}
	primary, _, err := primaryOf(cwd)
	if err != nil {
		return "", nil, err
	}
	if branch == "" {
		branch = name
	}
	slug := slugifyBranch(branch)
	if slug == "" {
		return "", nil, fmt.Errorf("could not derive a worktree directory name from %q", branch)
	}
	container := primary + ".worktrees"
	dir = filepath.Join(container, slug)
	if _, statErr := os.Stat(dir); statErr == nil {
		return "", nil, fmt.Errorf("worktree directory %q already exists at %s", slug, dir)
	}
	var args []string
	if branchExists(primary, branch) {
		// Existing local/remote branch: check it out in the new worktree.
		args = []string{"worktree", "add", dir, branch}
	} else {
		// New branch of the given name, based on the repository's default branch.
		args = []string{"worktree", "add", "-b", branch, dir}
		if def := defaultBranchOfRepo(primary); def != "" {
			args = append(args, def)
		}
	}
	if err := os.MkdirAll(container, 0o755); err != nil {
		return "", nil, err
	}
	if err := run(primary, "git", args...); err != nil {
		return "", nil, err
	}
	return dir, CopyIncludes(primary, dir), nil
}

// Environment variables exported to a worktree setup command, naming the new worktree, its
// repository (the copy source), and the branch checked out, so a setup script can locate the
// source it was provisioned from.
const (
	EnvWorktree = "BIRDSEYE_WORKTREE"
	EnvRepo     = "BIRDSEYE_REPO"
	EnvBranch   = "BIRDSEYE_BRANCH"
)

// SetupEnv returns the setup command's env contract as KEY=VALUE assignments: the new
// worktree's absolute path (also the working directory), the primary worktree's absolute path
// (the copy source), and the branch checked out. The agentless path appends these to the
// process environment; the spawn path uses them as a shell prefix on the chained command.
func SetupEnv(worktreeDir, primary, branch string) []string {
	return []string{
		EnvWorktree + "=" + worktreeDir,
		EnvRepo + "=" + primary,
		EnvBranch + "=" + branch,
	}
}

// CopyIncludes carries a repository's .worktreeinclude files from its primary worktree into a
// newly created worktree. It honors the cross-tool .worktreeinclude convention: a file is
// copied only when it both matches an include pattern and is git-ignored, so tracked files are
// never duplicated. The guardrail is git's: candidates come from `git ls-files --others
// --ignored --exclude-from=.worktreeinclude` (untracked files matching the include patterns),
// then each is confirmed git-ignored with `git check-ignore`. It is best-effort - an absent
// .worktreeinclude or one matching nothing is a no-op, and a per-file failure is collected as a
// non-fatal warning rather than aborting - so it never blocks worktree creation.
func CopyIncludes(primary, dst string) []string {
	if _, err := os.Stat(filepath.Join(primary, ".worktreeinclude")); err != nil {
		return nil // no include file → nothing to carry over
	}
	out, err := output(primary, "git", "ls-files", "--others", "--ignored", "--exclude-from=.worktreeinclude", "-z")
	if err != nil {
		return []string{fmt.Sprintf(".worktreeinclude: listing files failed: %v", err)}
	}
	var warnings []string
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		// Confirm the candidate is git-ignored by the repository's standard ignore rules,
		// not merely matched by an include pattern, so an unignored file is never copied.
		if err := run(primary, "git", "check-ignore", "-q", "--", rel); err != nil {
			continue
		}
		if err := copyFile(filepath.Join(primary, rel), filepath.Join(dst, rel)); err != nil {
			warnings = append(warnings, fmt.Sprintf(".worktreeinclude: %s: %v", rel, err))
		}
	}
	return warnings
}

// copyFile copies a regular file from src to dst, creating parent directories and preserving
// the source's permission bits. Non-regular sources (directories, symlinks, devices) are
// skipped rather than copied, since the include set is meant for plain local files.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// slugifyBranch turns a branch name into a filesystem-safe directory name: path
// separators and any other character outside [A-Za-z0-9._-] become a hyphen, runs of
// hyphens collapse, and leading/trailing hyphens and dots are trimmed. "feature/login" →
// "feature-login".
func slugifyBranch(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	slug := b.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-.")
}

// List returns the managed repositories discovered under the given roots. A repository is
// a git clone found directly under a root, reported once with its primary worktree path.
// Repositories are de-duplicated by their shared git dir, so a linked worktree that
// happens to sit under a root does not produce a second entry.
func List(roots ...string) ([]Managed, error) {
	var out []Managed
	seen := map[string]bool{}
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
			p := filepath.Join(root, e.Name())
			if !isGitWorktree(p) {
				continue
			}
			gitDir, ok := commonDirOf(p)
			if !ok || seen[gitDir] {
				continue
			}
			seen[gitDir] = true
			wts, err := ListWorktrees(p)
			if err != nil || len(wts) == 0 {
				continue
			}
			primary := primaryPath(wts)
			out = append(out, Managed{Repo: filepath.Base(primary), Path: primary, GitDir: gitDir})
		}
	}
	return out, nil
}

// primaryPath returns the primary worktree's path from a worktree set, falling back to
// the first entry.
func primaryPath(wts []Worktree) string {
	for _, w := range wts {
		if w.IsPrimary {
			return w.Path
		}
	}
	if len(wts) > 0 {
		return wts[0].Path
	}
	return ""
}

// commonDirOf returns the absolute, symlink-resolved git common-dir for a worktree
// directory — the identity shared by every worktree of the same repository. ok is
// false when path is not actually a git worktree (git rejects it).
func commonDirOf(path string) (string, bool) {
	out, err := output(path, "git", "rev-parse", "--git-common-dir")
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(path, out)
	}
	if resolved, err := filepath.EvalSymlinks(out); err == nil {
		out = resolved
	}
	return filepath.Clean(out), true
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

// RemoveRefusedError reports that the `git worktree remove` command itself declined,
// carrying git's own reason verbatim so a caller can quote it. It is distinct from the
// pre-flight guards callers apply (a primary worktree, a co-tenant agent), which never
// reach git.
//
// Every reason git declines an unforced remove for is resolved by --force, including ones
// no dirtiness probe predicts: a worktree containing submodules is refused outright
// ("working trees containing submodules cannot be moved or removed") even when perfectly
// clean. So a refusal with Forced=false is an invitation to retry with force, not a dead
// end. Forced=true is the terminal case.
type RemoveRefusedError struct {
	Dir    string
	Forced bool   // whether --force was already passed on the attempt that failed
	Reason string // git's message, stripped of its "fatal: " prefix
}

func (e *RemoveRefusedError) Error() string {
	return fmt.Sprintf("git worktree remove %s: %s", e.Dir, e.Reason)
}

// Remove removes the worktree at dir via `git worktree remove`. With force=false git
// declines a worktree that has uncommitted changes (and some it will never remove unforced
// — see RemoveRefusedError); force=true passes --force. A declined removal returns a
// *RemoveRefusedError. The command runs from the repository's primary worktree
// (filepath.Dir(gitDir)) so removing a linked worktree — even the caller's own — succeeds.
// git itself refuses to remove a primary worktree, which is what keeps a repo's anchor
// non-deletable.
//
// The primary is derived from gitDir (the repository's common git dir, which the caller has
// already resolved) rather than by running git inside dir, so Remove still works when dir has
// been deleted on disk — a prunable worktree git still lists. Resolving the primary from the
// to-be-removed directory would fail there with a confusing "not inside a git worktree".
func Remove(dir, gitDir string, force bool) error {
	if err := requireGit(); err != nil {
		return err
	}
	primary := filepath.Dir(gitDir)
	args := []string{"worktree", "remove", dir}
	if force {
		args = append(args, "--force")
	}
	stderr, err := runStderr(primary, "git", args...)
	if err != nil {
		return &RemoveRefusedError{Dir: dir, Forced: force, Reason: gitReason(stderr, err)}
	}
	return nil
}

// gitReason condenses a failed git command's stderr into one human line: the last non-empty
// line (git prints its fatal last, after any warnings) with the "fatal: "/"error: " prefix
// stripped, falling back to the exec error when git printed nothing.
func gitReason(stderr string, err error) string {
	var reason string
	for _, line := range strings.Split(stderr, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			reason = s
		}
	}
	if reason == "" {
		return err.Error()
	}
	return strings.TrimPrefix(strings.TrimPrefix(reason, "fatal: "), "error: ")
}

// Info describes the managed-repo context of a directory (git-derived).
type Info struct {
	GitDir    string // git common dir — the repository's stable identity across worktrees
	Repo      string // the repository's name (the primary worktree's base name)
	TopLevel  string // this directory's worktree top level
	Worktree  string // the worktree's name (base of TopLevel)
	IsPrimary bool   // whether this worktree is git's primary worktree (the anchor)
}

// Resolve returns the managed-repo context of dir, or ok=false when dir is not inside a
// git worktree. It is the git-backed classifier the agents view's reconciler uses.
func Resolve(dir string) (Info, bool) {
	top, err := output(dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return Info{}, false
	}
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		top = resolved
	}
	top = filepath.Clean(top)
	primary, gitDir, err := primaryOf(dir)
	if err != nil {
		return Info{}, false
	}
	return Info{
		GitDir:    gitDir,
		Repo:      filepath.Base(primary),
		TopLevel:  top,
		Worktree:  filepath.Base(top),
		IsPrimary: top == primary,
	}, true
}

// primaryOf resolves the repository identity for a starting directory: its git common
// dir (shared across all worktrees) and its primary worktree (the parent of the common
// dir — the worktree git refuses to remove). It errors clearly when start is not inside a
// git worktree.
func primaryOf(start string) (primary, gitDir string, err error) {
	gd, ok := commonDirOf(start)
	if !ok {
		return "", "", fmt.Errorf("not inside a git worktree; run `be worktree add` from within a managed repo")
	}
	return filepath.Dir(gd), gd, nil
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
	stderr, err := runStderr(dir, name, args...)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return nil
}

// runStderr runs a command in dir and returns its raw stderr alongside the exec error, for
// callers that classify a failure by what the command said rather than just reporting it.
func runStderr(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return errBuf.String(), err
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
