package theme

import (
	"strings"
	"testing"
)

func TestTruecolorDetection(t *testing.T) {
	for _, v := range []string{"truecolor", "24bit"} {
		t.Setenv("COLORTERM", v)
		if !Truecolor() {
			t.Errorf("COLORTERM=%q should enable truecolor", v)
		}
	}
	for _, v := range []string{"", "256color", "yes"} {
		t.Setenv("COLORTERM", v)
		if Truecolor() {
			t.Errorf("COLORTERM=%q should not enable truecolor", v)
		}
	}
}

func TestFGTruecolorVs256(t *testing.T) {
	// Blue is #4ea8ff -> 78,168,255.
	if got := Blue.FG("x", true); !strings.Contains(got, "\x1b[38;2;78;168;255m") {
		t.Errorf("truecolor FG should emit 24-bit SGR, got %q", got)
	}
	if got := Blue.FG("x", false); !strings.Contains(got, "\x1b[38;5;39m") {
		t.Errorf("256 FG should emit palette index, got %q", got)
	}
}

func TestSpecFormat(t *testing.T) {
	if got := Coral.Spec(true); got != "#ff7a6b" {
		t.Errorf("truecolor spec should be hex, got %q", got)
	}
	if got := Coral.Spec(false); got != "209" {
		t.Errorf("256 spec should be palette index, got %q", got)
	}
}
