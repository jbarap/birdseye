// Package tmuxp provides create candidates from tmuxp templates. Selecting one
// loads the template into a new session.
package tmuxp

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jbarap/birds-eye/internal/provider"
)

// Type is the provider's source tag.
const Type = "tmuxp"

// ErrUnavailable is returned when the tmuxp binary is not installed; the
// registry turns it into a non-fatal warning.
var ErrUnavailable = errors.New("tmuxp is not installed")

// Lister reports the currently running tmux session names. The provider uses it
// to discover which session a template actually created (a template's
// session_name may differ from its file name).
type Lister interface {
	ListSessions() ([]string, error)
}

// Provider lists tmuxp templates as create candidates.
type Provider struct {
	available bool
	lister    Lister
	// list returns template names (without extension).
	list func() ([]string, error)
	// load loads the named template detached.
	load func(name string) error
}

// New returns a Provider. dir overrides the tmuxp config directory ("" uses the
// default discovery). available reflects whether the tmuxp binary is on PATH.
// lister is used to resolve the created session name after a load.
func New(dir string, available bool, lister Lister) *Provider {
	return &Provider{
		available: available,
		lister:    lister,
		list:      func() ([]string, error) { return listTemplates(dir) },
		load:      loadTemplate,
	}
}

// Type returns the provider's source tag.
func (p *Provider) Type() string { return Type }

// Candidates returns one create candidate per template, or ErrUnavailable when
// tmuxp is not installed.
func (p *Provider) Candidates() ([]provider.Candidate, error) {
	if !p.available {
		return nil, ErrUnavailable
	}
	names, err := p.list()
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
			Kind:  provider.KindCreate,
			Action: func(b provider.Backend) error {
				// A template's session_name may differ from its file name, so
				// snapshot sessions around the load and connect to whatever was
				// actually created rather than assuming the name.
				before := p.sessionSet()
				if err := p.load(name); err != nil {
					return err
				}
				target := name
				if created := p.newSession(before); created != "" {
					target = created
				}
				return b.Connect(target)
			},
		})
	}
	return out, nil
}

// sessionSet returns the set of running session names, or nil when no lister is
// configured or listing fails (in which case the caller falls back to the
// template name).
func (p *Provider) sessionSet() map[string]bool {
	if p.lister == nil {
		return nil
	}
	names, err := p.lister.ListSessions()
	if err != nil {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// newSession returns the first running session absent from before, i.e. the one
// the load just created. It returns "" when nothing new is detected.
func (p *Provider) newSession(before map[string]bool) string {
	if before == nil {
		return ""
	}
	for _, n := range p.sessionList() {
		if !before[n] {
			return n
		}
	}
	return ""
}

// sessionList returns the current running session names, or nil on failure.
func (p *Provider) sessionList() []string {
	if p.lister == nil {
		return nil
	}
	names, err := p.lister.ListSessions()
	if err != nil {
		return nil
	}
	return names
}

// configDir resolves the tmuxp template directory.
func configDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv("TMUXP_CONFIGDIR"); env != "" {
		return env, nil
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "tmuxp"), nil
}

func listTemplates(override string) ([]string, error) {
	dir, err := configDir(override)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".yaml", ".yml", ".json":
			names = append(names, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		}
	}
	return names, nil
}

func loadTemplate(name string) error {
	// -d loads the session detached so bird's-eye controls the connect step.
	cmd := exec.Command("tmuxp", "load", "-d", name)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
