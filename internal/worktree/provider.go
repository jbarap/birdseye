package worktree

import (
	"strings"

	"github.com/jbarap/birdseye/internal/provider"
)

// Type is the provider's source tag.
const Type = "repo"

// Provider surfaces discovered repositories as session candidates — one per repo.
type Provider struct {
	roots []string
}

// NewProvider returns a Provider listing worktrees discovered under roots. When roots
// is empty it yields no candidates.
func NewProvider(roots []string) *Provider { return &Provider{roots: roots} }

// Type returns the provider's source tag.
func (p *Provider) Type() string { return Type }

// Candidates returns one create candidate per discovered repository, opening a session at
// the repo's primary worktree.
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
		name := sessionName(m.Repo)
		path := m.Path
		out = append(out, provider.Candidate{
			Name:  name,
			Label: m.Repo,
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
	return out, nil
}

// sessionName builds a tmux-safe session name from a repo name.
func sessionName(repo string) string {
	r := strings.NewReplacer(".", "_", ":", "_", " ", "_", "/", "-")
	return r.Replace(repo)
}
