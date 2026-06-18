package tmuxp

import "testing"

type fakeLister struct {
	sessions []string
	err      error
}

func (f fakeLister) ListSessions() ([]string, error) { return f.sessions, f.err }

func TestNewSessionDetectsCreated(t *testing.T) {
	p := New("", true, fakeLister{sessions: []string{"a", "b", "made-by-template"}})
	before := map[string]bool{"a": true, "b": true}
	if got := p.newSession(before); got != "made-by-template" {
		t.Fatalf("expected detected new session, got %q", got)
	}
}

func TestNewSessionNoneWhenUnchanged(t *testing.T) {
	p := New("", true, fakeLister{sessions: []string{"a", "b"}})
	before := map[string]bool{"a": true, "b": true}
	if got := p.newSession(before); got != "" {
		t.Fatalf("expected no new session, got %q", got)
	}
}

func TestNewSessionNilBeforeFallsBack(t *testing.T) {
	p := New("", true, fakeLister{sessions: []string{"a"}})
	if got := p.newSession(nil); got != "" {
		t.Fatalf("nil before should yield empty (caller uses template name), got %q", got)
	}
}

func TestSessionSetNilWhenNoLister(t *testing.T) {
	p := New("", true, nil)
	if p.sessionSet() != nil {
		t.Fatal("expected nil session set without a lister")
	}
}

func TestUnavailableReturnsError(t *testing.T) {
	p := New("", false, nil)
	if _, err := p.Candidates(); err != ErrUnavailable {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}
