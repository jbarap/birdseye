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
	if res.double['g'] != ActionTop {
		t.Fatalf("gg should jump to top, got %q", res.double['g'])
	}
	if res.single["G"] != ActionBottom {
		t.Fatalf("G should jump to bottom")
	}
	if res.single["enter"] != ActionSelect || res.single["q"] != ActionQuit {
		t.Fatalf("enter/q not bound")
	}
}

func TestResolveKeymapOverrideReplacesAction(t *testing.T) {
	km, err := ResolveKeymap(map[string][]string{"down": {"n"}})
	if err != nil {
		t.Fatal(err)
	}
	res, _ := km.buildResolver()
	if res.single["n"] != ActionDown {
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
