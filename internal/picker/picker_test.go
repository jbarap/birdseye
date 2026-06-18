package picker

import (
	"strings"
	"testing"

	"github.com/jbarap/birds-eye/internal/config"
	"github.com/jbarap/birds-eye/internal/provider"
)

func TestOrderByType(t *testing.T) {
	cands := []provider.Candidate{
		{Name: "d1", Type: "dir"},
		{Name: "t1", Type: "tmux"},
		{Name: "d2", Type: "dir"},
		{Name: "p1", Type: "tmuxp"},
	}
	got := orderByType(cands, []string{"tmux", "tmuxp", "dir"})
	want := []string{"t1", "p1", "d1", "d2"}
	for i, w := range want {
		if got[i].Name != w {
			t.Fatalf("position %d: got %s want %s (full %v)", i, got[i].Name, w, names(got))
		}
	}
}

func TestDisplayMarksKind(t *testing.T) {
	cfg := config.Default()
	attach := display(provider.Candidate{Label: "x", Type: "tmux", Kind: provider.KindAttach}, cfg)
	create := display(provider.Candidate{Label: "y", Type: "dir", Kind: provider.KindCreate}, cfg)
	if !strings.HasPrefix(attach, "→") {
		t.Errorf("attach should start with arrow marker, got %q", attach)
	}
	if !strings.HasPrefix(create, "+") {
		t.Errorf("create should start with + marker, got %q", create)
	}
}

func TestPickEmptyReturnsSentinel(t *testing.T) {
	_, err := Pick(nil, config.Default())
	if err != ErrNoCandidates {
		t.Fatalf("expected ErrNoCandidates, got %v", err)
	}
}

func names(cs []provider.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}
