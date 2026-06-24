package agents

import (
	"strings"
	"testing"
)

func TestResolveKeymapDefaultsWhenAbsent(t *testing.T) {
	km, err := ResolveKeymap(nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := km.buildResolver()
	if err != nil {
		t.Fatal(err)
	}
	if res.single["j"] != ActionDown || res.single["k"] != ActionUp {
		t.Fatalf("default vim keys not bound: %+v", res.single)
	}
	if res.single["down"] != ActionDown || res.single["up"] != ActionUp {
		t.Fatalf("default arrow keys not bound: %+v", res.single)
	}
	if res.chords[[2]string{"g", "g"}] != ActionTop {
		t.Fatalf("gg should jump to top, got %q", res.chords[[2]string{"g", "g"}])
	}
	if res.single["G"] != ActionBottom {
		t.Fatalf("G should jump to bottom")
	}
	if res.single["enter"] != ActionSelect || res.single["q"] != ActionQuit {
		t.Fatalf("enter/q not bound")
	}
}

func TestResolveKeymapOverrideReplacesAction(t *testing.T) {
	km, err := ResolveKeymap(map[string][]string{"down": {"p"}})
	if err != nil {
		t.Fatal(err)
	}
	res, _ := km.buildResolver()
	if res.single["p"] != ActionDown {
		t.Fatalf("custom down key not bound")
	}
	if _, ok := res.single["j"]; ok {
		t.Fatalf("override should replace, not extend: j still bound")
	}
}

func TestResolveKeymapErrors(t *testing.T) {
	cases := map[string]map[string][]string{
		"unknown action": {"sideways": {"x"}},
		"empty keys":     {"down": {}},
		"duplicate key":  {"up": {"j"}}, // j is the default for down
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ResolveKeymap(overrides); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestResolveKeymapRuneChordConflict(t *testing.T) {
	// Binding single "g" to an action collides with the default "gg" chord.
	_, err := ResolveKeymap(map[string][]string{"down": {"g"}})
	if err == nil || !strings.Contains(err.Error(), "chord") {
		t.Fatalf("expected chord-conflict error, got %v", err)
	}
}

// TestTwoKeyChords covers the generalized resolver: a repeated-key chord (gg), and two
// mixed chords that share a first key but differ on the second (dd vs dD) resolving to
// distinct actions. The shared first key is a single prefix arming both.
func TestTwoKeyChords(t *testing.T) {
	res, err := DefaultKeymap().buildResolver()
	if err != nil {
		t.Fatal(err)
	}
	if got := res.chords[[2]string{"g", "g"}]; got != ActionTop {
		t.Fatalf("gg should be a chord for top, got %q", got)
	}
	if got := res.chords[[2]string{"d", "d"}]; got != ActionClose {
		t.Fatalf("dd should be a chord for close, got %q", got)
	}
	if got := res.chords[[2]string{"d", "D"}]; got != ActionDelete {
		t.Fatalf("dD should be a chord for delete, distinct from dd, got %q", got)
	}
	if !res.prefix["d"] || !res.prefix["g"] {
		t.Fatalf("d and g should both be chord prefixes: %+v", res.prefix)
	}
	// A first key that begins a chord must not also be a single binding.
	if _, ok := res.single["d"]; ok {
		t.Fatal("d should arm a chord, not be a single binding")
	}
}

// TestSingleConflictsWithChordPrefix checks that binding a single key equal to a chord's
// first key is rejected at load time, naming the conflict.
func TestSingleConflictsWithChordPrefix(t *testing.T) {
	// "d" begins the default dd/dD chords; binding it as a single must fail.
	_, err := ResolveKeymap(map[string][]string{"down": {"d"}})
	if err == nil || !strings.Contains(err.Error(), "chord") {
		t.Fatalf("a single key equal to a chord prefix should be a chord conflict, got %v", err)
	}
}
