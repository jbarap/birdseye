// Package tmuxsess provides session candidates for currently running tmux
// sessions, surfaced as attach candidates.
package tmuxsess

import "github.com/jbarap/birds-eye/internal/provider"

// Type is the provider's source tag.
const Type = "tmux"

// Lister is the subset of the tmux backend this provider needs.
type Lister interface {
	ListSessions() ([]string, error)
}

// Provider lists running tmux sessions.
type Provider struct {
	tmux Lister
}

// New returns a Provider backed by the given session lister.
func New(t Lister) *Provider { return &Provider{tmux: t} }

// Type returns the provider's source tag.
func (p *Provider) Type() string { return Type }

// Candidates returns one attach candidate per running session. When no server
// is running the lister yields nothing and this returns no candidates.
func (p *Provider) Candidates() ([]provider.Candidate, error) {
	names, err := p.tmux.ListSessions()
	if err != nil {
		return nil, err
	}
	out := make([]provider.Candidate, 0, len(names))
	for _, name := range names {
		name := name
		out = append(out, provider.Candidate{
			Name:  name,
			Label: name,
			Type:  Type,
			Kind:  provider.KindAttach,
			Action: func(b provider.Backend) error {
				return b.Connect(name)
			},
		})
	}
	return out, nil
}
