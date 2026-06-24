// Package provider defines the extension seam of birdseye: a small interface
// any session source implements to contribute candidates, plus a registry that
// aggregates them. The picker and CLI depend only on these types, never on a
// concrete provider, so new sources drop in without touching them.
package provider

// Kind distinguishes a candidate that attaches to an existing session from one
// that creates a new session.
type Kind int

const (
	// KindAttach connects to a session that is already running.
	KindAttach Kind = iota
	// KindCreate materializes a new session and then connects to it.
	KindCreate
)

func (k Kind) String() string {
	if k == KindAttach {
		return "attach"
	}
	return "create"
}

// Backend is the subset of tmux operations a candidate's action needs. It lives
// here (rather than in the tmux package) so providers do not import the tmux
// backend directly and no import cycle forms; the tmux client implements it.
type Backend interface {
	// Ensure creates a detached session named name rooted at dir if one does
	// not already exist, and is a no-op when it does. window names the session's
	// initial window (the one holding dir); empty leaves tmux's default.
	Ensure(name, dir, window string) error
	// Connect attaches to (outside tmux) or switches the client to (inside
	// tmux) the named session.
	Connect(name string) error
}

// Candidate is one selectable entry in the picker.
type Candidate struct {
	// Name is the canonical tmux session name. It is the dedup key across
	// providers.
	Name string
	// Label is the human-facing display text.
	Label string
	// Type is the source tag used for grouping, labeling, and config keys
	// (e.g. "tmux", "tmuxp", "dir", "repo").
	Type string
	// Kind records whether selecting this attaches or creates.
	Kind Kind
	// Dir is the directory a create candidate would be rooted in, where applicable
	// (dir and worktree candidates carry it; attach and template candidates leave it
	// empty). The headless `be sessions --json` surfaces it so a client can feed it to
	// `be agents spawn`.
	Dir string
	// Action performs the selection against the tmux backend.
	Action func(b Backend) error
}

// Provider is a source of session candidates.
type Provider interface {
	// Type returns the source tag shared by this provider's candidates. It is
	// also the key used to enable/disable the provider in config.
	Type() string
	// Candidates enumerates the provider's current candidates. Returning an
	// error is non-fatal: the registry isolates it and continues.
	Candidates() ([]Candidate, error)
}
