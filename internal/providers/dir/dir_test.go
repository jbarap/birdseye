package dir

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCandidatesDistinctForSharedBasename pins that two directories sharing a basename but
// living at different paths produce distinct candidate names, so the registry (which dedups
// by name) keeps both rather than dropping the second.
func TestCandidatesDistinctForSharedBasename(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "a", "v3")
	b := filepath.Join(base, "b", "v3")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p := New(true, 0, nil)
	p.zoxideDirs = func() ([]string, error) { return []string{a, b}, nil }
	got, err := p.Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates for two distinct v3 dirs, got %d", len(got))
	}
	if got[0].Name == got[1].Name {
		t.Fatalf("distinct v3 dirs collided onto one name %q", got[0].Name)
	}
}

func TestCandidatesEmptyWhenNoSources(t *testing.T) {
	p := New(false, 0, nil)
	got, err := p.Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no candidates, got %d", len(got))
	}
}

// TestCandidatesLimitsZoxideToMostUsed pins that a positive limit keeps only the head of
// zoxide's frecency-sorted list (its most-used dirs), and that 0 leaves the list uncapped.
func TestCandidatesLimitsZoxideToMostUsed(t *testing.T) {
	base := t.TempDir()
	var dirs []string
	for _, n := range []string{"a", "b", "c", "d"} {
		d := filepath.Join(base, n)
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, d)
	}
	zoxide := func() ([]string, error) { return dirs, nil }

	p := New(true, 2, nil)
	p.zoxideDirs = zoxide
	got, err := p.Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("limit 2 should keep 2 candidates, got %d", len(got))
	}
	if got[0].Dir != dirs[0] || got[1].Dir != dirs[1] {
		t.Fatalf("expected the head of the frecency list, got %q and %q", got[0].Dir, got[1].Dir)
	}

	p = New(true, 0, nil)
	p.zoxideDirs = zoxide
	got, err = p.Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(dirs) {
		t.Fatalf("limit 0 should keep all %d candidates, got %d", len(dirs), len(got))
	}
}
