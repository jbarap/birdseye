// Package dir provides create candidates from directories sourced via zoxide
// and/or configured roots. Selecting one starts a session rooted in that
// directory, named after it.
package dir

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jbarap/birdseye/internal/provider"
)

// Type is the provider's source tag.
const Type = "dir"

// Provider lists directories as create candidates.
type Provider struct {
	useZoxide bool
	roots     []string
	// zoxideDirs returns directories known to zoxide (most-used first).
	zoxideDirs func() ([]string, error)
}

// New returns a Provider. useZoxide enables zoxide as a source when available;
// roots are directories whose immediate children become candidates.
func New(useZoxide bool, roots []string) *Provider {
	return &Provider{
		useZoxide:  useZoxide,
		roots:      roots,
		zoxideDirs: queryZoxide,
	}
}

// Type returns the provider's source tag.
func (p *Provider) Type() string { return Type }

// Candidates returns create candidates for the discovered directories. When no
// source yields anything it returns no candidates without erroring.
func (p *Provider) Candidates() ([]provider.Candidate, error) {
	seen := map[string]bool{}
	var out []provider.Candidate

	add := func(path string) {
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return
		}
		seen[path] = true
		base := filepath.Base(path)
		name := SessionName(base)
		dir := path
		out = append(out, provider.Candidate{
			Name:  name,
			Label: name + "  (" + path + ")",
			Type:  Type,
			Kind:  provider.KindCreate,
			Dir:   dir,
			Action: func(b provider.Backend) error {
				// Name the initial window after the directory, not the running shell.
				if err := b.Ensure(name, dir, base); err != nil {
					return err
				}
				return b.Connect(name)
			},
		})
	}

	if p.useZoxide {
		dirs, err := p.zoxideDirs()
		if err == nil {
			for _, d := range dirs {
				add(d)
			}
		}
	}
	for _, root := range p.roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	return out, nil
}

// SessionName sanitizes a directory base name into a valid tmux session name.
// tmux forbids '.' and ':' in session names, so they are replaced with '_'.
func SessionName(base string) string {
	r := strings.NewReplacer(".", "_", ":", "_", " ", "_")
	return r.Replace(base)
}

// HomeSession is the deterministic tmux session name be uses as a repository's
// agent home - its write domain. It is a pure function of the repository's identity
// (its git-common-dir): the repository basename is the friendly part, and a short hash
// of the git-common-dir is always appended. The hash does double duty - it disambiguates
// two repositories that share a basename, and it marks the session as be-derived without a
// noisy prefix. Determinism is load-bearing - a repository's home always resolves to the
// same name regardless of what else is running - so the disambiguator is derived from the
// git-common-dir alone, never from the live session set.
func HomeSession(gitCommonDir string) string {
	clean := filepath.Clean(gitCommonDir)
	base := SessionName(filepath.Base(filepath.Dir(clean)))
	sum := sha256.Sum256([]byte(clean))
	return base + "-" + hex.EncodeToString(sum[:])[:6]
}

func queryZoxide() ([]string, error) {
	if _, err := exec.LookPath("zoxide"); err != nil {
		return nil, err
	}
	out, err := exec.Command("zoxide", "query", "-l").Output()
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs, nil
}
