package worktree

import (
	"strings"

	"github.com/jbarap/birds-eye/internal/provider"
)

// Type is the provider's source tag.
const Type = "worktree"

// Provider surfaces managed repositories' worktrees as session candidates.
type Provider struct {
	roots []string
}

// NewProvider returns a Provider listing worktrees discovered under roots. When roots
// is empty it yields no candidates.
func NewProvider(roots []string) *Provider { return &Provider{roots: roots} }

// Type returns the provider's source tag.
func (p *Provider) Type() string { return Type }

// Candidates returns one create candidate per managed worktree directory.
func (p *Provider) Candidates() ([]provider.Candidate, error) {
	if len(p.roots) == 0 {
		return nil, nil
	}
	managed, err := List(p.roots...)
	if err != nil {
		return nil, err
	}
	var out []provider.Candidate
	for _, m := range managed {
		for _, d := range m.Dirs {
			name := sessionName(m.Repo, d.Name)
			path := d.Path
			out = append(out, provider.Candidate{
				Name:  name,
				Label: m.Repo + "/" + d.Name,
				Type:  Type,
				Kind:  provider.KindCreate,
				Dir:   path,
				Action: func(b provider.Backend) error {
					if err := b.Ensure(name, path); err != nil {
						return err
					}
					return b.Connect(name)
				},
			})
		}
	}
	return out, nil
}

// sessionName builds a tmux-safe session name from repo and worktree names.
func sessionName(repo, wt string) string {
	r := strings.NewReplacer(".", "_", ":", "_", " ", "_", "/", "-")
	return r.Replace(repo + "-" + wt)
}
