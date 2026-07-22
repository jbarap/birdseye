// Package dir provides create candidates from directories sourced via zoxide
// and/or configured roots. Selecting one starts a session rooted in that
// directory, named after it.
package dir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jbarap/birdseye/internal/provider"
	"github.com/jbarap/birdseye/internal/sessname"
)

// Type is the provider's source tag.
const Type = "dir"

// Provider lists directories as create candidates.
type Provider struct {
	useZoxide bool
	limit     int
	roots     []string
	// zoxideDirs returns directories known to zoxide (most-used first).
	zoxideDirs func() ([]string, error)
}

// New returns a Provider. useZoxide enables zoxide as a source when available;
// limit caps how many of zoxide's directories are offered, keeping its most-used
// first (zoxide sorts by frecency), with 0 meaning no cap; roots are directories
// whose immediate children become candidates.
func New(useZoxide bool, limit int, roots []string) *Provider {
	return &Provider{
		useZoxide:  useZoxide,
		limit:      limit,
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
		// The session name hashes the cleaned path (not the bare base) so two directories
		// sharing a basename stay distinct rather than colliding onto one candidate the
		// registry would dedup away.
		name := sessname.For(base, path)
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
			// zoxide lists most-used first, so the head of the list is the popular set.
			if p.limit > 0 && len(dirs) > p.limit {
				dirs = dirs[:p.limit]
			}
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
