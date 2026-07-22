package worktree

import (
	"github.com/jbarap/birdseye/internal/provider"
	"github.com/jbarap/birdseye/internal/sessname"
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
		// Name the session by the repository's home (the same <repo>-<hash> the dash's
		// `n` and `be agents spawn` use) so opening a repo here and spawning into it land
		// in one session rather than two differently-named ones.
		name := sessname.Home(m.GitDir)
		path := m.Path
		out = append(out, provider.Candidate{
			Name:  name,
			Label: m.Repo,
			Type:  Type,
			Kind:  provider.KindCreate,
			Dir:   path,
			Action: func(b provider.Backend) error {
				// Name the base window after the repo, not the running shell.
				if err := b.Ensure(name, path, m.Repo); err != nil {
					return err
				}
				return b.Connect(name)
			},
		})
	}
	return out, nil
}
