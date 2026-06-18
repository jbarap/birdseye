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
	tm := theme{truecolor: false}
	attach := display(provider.Candidate{Label: "x", Type: "tmux", Kind: provider.KindAttach}, cfg, tm)
	create := display(provider.Candidate{Label: "y", Type: "dir", Kind: provider.KindCreate}, cfg, tm)
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
	got := display(provider.Candidate{Label: "x", Type: "tmux", Kind: provider.KindAttach}, cfg, theme{})
	if !strings.Contains(got, "x") {
		t.Fatalf("display should still render the label without an icon, got %q", got)
	}
}

func TestNewThemeDetectsTruecolor(t *testing.T) {
	for _, v := range []string{"truecolor", "24bit"} {
		t.Setenv("COLORTERM", v)
		if !newTheme().truecolor {
			t.Errorf("COLORTERM=%q should enable truecolor", v)
		}
	}
	for _, v := range []string{"", "256color", "yes"} {
		t.Setenv("COLORTERM", v)
		if newTheme().truecolor {
			t.Errorf("COLORTERM=%q should not enable truecolor", v)
		}
	}
}

func TestThemeRendersTruecolorVs256(t *testing.T) {
	// Truecolor emits a 24-bit SGR (38;2;r;g;b) from the hex; 256 emits 38;5;n.
	tc := theme{truecolor: true}.fg(cTmux, "x")
	if !strings.Contains(tc, "\x1b[38;2;78;168;255m") { // #4ea8ff
		t.Errorf("truecolor fg should emit 24-bit SGR, got %q", tc)
	}
	p256 := theme{truecolor: false}.fg(cTmux, "x")
	if !strings.Contains(p256, "\x1b[38;5;39m") {
		t.Errorf("256 fg should emit palette index, got %q", p256)
	}
}

func TestFzfColorSpecSwitchesFormat(t *testing.T) {
	tc := fzfColorSpec(theme{truecolor: true})
	if !strings.Contains(tc, "hl:#4ea8ff") || !strings.Contains(tc, "hl+:#ff7a6b") {
		t.Errorf("truecolor spec should use hex accents, got %q", tc)
	}
	p := fzfColorSpec(theme{truecolor: false})
	if !strings.Contains(p, "hl:39") || !strings.Contains(p, "hl+:209") {
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
