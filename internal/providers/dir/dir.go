// Package dir provides create candidates from directories sourced via zoxide
// and/or configured roots. Selecting one starts a session rooted in that
// directory, named after it.
package dir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jbarap/birds-eye/internal/provider"
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
		name := SessionName(filepath.Base(path))
		dir := path
		out = append(out, provider.Candidate{
			Name:  name,
			Label: name + "  (" + path + ")",
			Type:  Type,
			Kind:  provider.KindCreate,
			Dir:   dir,
			Action: func(b provider.Backend) error {
				if err := b.Ensure(name, dir); err != nil {
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
