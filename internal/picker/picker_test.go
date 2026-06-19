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
	attach := display(provider.Candidate{Label: "x", Type: "tmux", Kind: provider.KindAttach}, cfg, false)
	create := display(provider.Candidate{Label: "y", Type: "dir", Kind: provider.KindCreate}, cfg, false)
	if !strings.Contains(attach, "→") {
		t.Errorf("attach should carry the arrow marker, got %q", attach)
	}
	if !strings.Contains(create, "+") {
		t.Errorf("create should carry the + marker, got %q", create)
	}
	// The label, the type's icon, and the type label all appear.
	if !strings.Contains(attach, "x") || !strings.Contains(attach, "session") {
		t.Errorf("attach should include label and type label, got %q", attach)
	}
	if icon := cfg.Icon("tmux"); icon != "" && !strings.Contains(attach, icon) {
		t.Errorf("attach should include the tmux icon %q, got %q", icon, attach)
	}
}

func TestDisplayOmitsIconWhenUnset(t *testing.T) {
	cfg := config.Default()
	cfg.Icons = map[string]string{} // no icons configured
	got := display(provider.Candidate{Label: "x", Type: "tmux", Kind: provider.KindAttach}, cfg, false)
	if !strings.Contains(got, "x") {
		t.Fatalf("display should still render the label without an icon, got %q", got)
	}
}

func TestFzfColorSpecSwitchesFormat(t *testing.T) {
	tc := fzfColorSpec(true)
	// Selection chrome uses the shared accent (#c792ea); the match base stays blue.
	if !strings.Contains(tc, "hl:#4ea8ff") || !strings.Contains(tc, "hl+:#c792ea") {
		t.Errorf("truecolor spec should use hex accents, got %q", tc)
	}
	for _, role := range []string{"pointer:#c792ea", "prompt:#c792ea", "marker:#c792ea"} {
		if !strings.Contains(tc, role) {
			t.Errorf("selection chrome should use the shared accent: missing %q in %q", role, tc)
		}
	}
	p := fzfColorSpec(false)
	if !strings.Contains(p, "hl:39") || !strings.Contains(p, "hl+:176") {
		t.Errorf("256 spec should use palette indices, got %q", p)
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

func TestFzfVersionAtLeast(t *testing.T) {
	cases := []struct {
		ver  string
		want bool
	}{
		{"0.72.0 (6fefe025)", true},
		{"0.72.0", true},
		{"0.80.1", true},
		{"1.0.0", true},
		{"0.71.0", false},
		{"0.9.0", false},
		{"", false},
		{"garbage", false},
		{"0", false},
	}
	for _, c := range cases {
		if got := fzfVersionAtLeast(c.ver, 0, 72); got != c.want {
			t.Errorf("fzfVersionAtLeast(%q, 0, 72) = %v, want %v", c.ver, got, c.want)
		}
	}
}
