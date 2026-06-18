package provider

import "testing"

// stubProvider is a Provider whose output is fixed for testing.
type stubProvider struct {
	typ   string
	cands []Candidate
	err   error
}

func (s stubProvider) Type() string                   { return s.typ }
func (s stubProvider) Candidates() ([]Candidate, error) { return s.cands, s.err }

func names(cs []Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestRegistryAggregatesEnabled(t *testing.T) {
	r := NewRegistry(nil)
	r.Register(stubProvider{typ: "a", cands: []Candidate{{Name: "one", Type: "a"}}})
	r.Register(stubProvider{typ: "b", cands: []Candidate{{Name: "two", Type: "b"}}})

	got := names(r.Candidates())
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("expected [one two], got %v", got)
	}
}

func TestRegistrySkipsDisabled(t *testing.T) {
	r := NewRegistry(nil)
	r.Register(stubProvider{typ: "a", cands: []Candidate{{Name: "one", Type: "a"}}})
	r.Register(stubProvider{typ: "b", cands: []Candidate{{Name: "two", Type: "b"}}})
	r.SetEnabled(map[string]bool{"b": false})

	got := names(r.Candidates())
	if len(got) != 1 || got[0] != "one" {
		t.Fatalf("expected only [one], got %v", got)
	}
}

func TestRegistryIsolatesProviderError(t *testing.T) {
	var warned string
	r := NewRegistry(func(m string) { warned = m })
	r.Register(stubProvider{typ: "bad", err: errBoom})
	r.Register(stubProvider{typ: "good", cands: []Candidate{{Name: "ok", Type: "good"}}})

	got := names(r.Candidates())
	if len(got) != 1 || got[0] != "ok" {
		t.Fatalf("expected surviving [ok], got %v", got)
	}
	if warned == "" {
		t.Fatalf("expected a warning for the failing provider")
	}
}

func TestRegistryDedupAttachBeatsCreate(t *testing.T) {
	r := NewRegistry(nil)
	// Creator registered first, attach for the same name registered second.
	r.Register(stubProvider{typ: "dir", cands: []Candidate{{Name: "proj", Type: "dir", Kind: KindCreate}}})
	r.Register(stubProvider{typ: "tmux", cands: []Candidate{{Name: "proj", Type: "tmux", Kind: KindAttach}}})

	got := r.Candidates()
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped candidate, got %d", len(got))
	}
	if got[0].Kind != KindAttach || got[0].Type != "tmux" {
		t.Fatalf("expected attach (tmux) to win, got kind=%v type=%s", got[0].Kind, got[0].Type)
	}
}

func TestRegistryDedupKeepsAttachWhenCreateComesSecond(t *testing.T) {
	r := NewRegistry(nil)
	r.Register(stubProvider{typ: "tmux", cands: []Candidate{{Name: "proj", Type: "tmux", Kind: KindAttach}}})
	r.Register(stubProvider{typ: "dir", cands: []Candidate{{Name: "proj", Type: "dir", Kind: KindCreate}}})

	got := r.Candidates()
	if len(got) != 1 || got[0].Kind != KindAttach {
		t.Fatalf("attach should remain when a later create collides, got %+v", got)
	}
}

var errBoom = stubErr("boom")

type stubErr string

func (e stubErr) Error() string { return string(e) }
