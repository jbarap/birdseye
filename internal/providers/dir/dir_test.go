package dir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionNameSanitizes(t *testing.T) {
	cases := map[string]string{
		"my.project": "my_project",
		"a:b":        "a_b",
		"with space": "with_space",
		"plain":      "plain",
	}
	for in, want := range cases {
		if got := SessionName(in); got != want {
			t.Errorf("SessionName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHomeSessionDeterministicAndCollisionSafe pins the two load-bearing properties of the
// home name: it is a deterministic function of the git-common-dir (the same repository
// always resolves to the same name, so spawning is idempotent), and two repositories that
// share a basename receive distinct names (so they never collide onto one home).
func TestHomeSessionDeterministicAndCollisionSafe(t *testing.T) {
	const a, b = "/work/proj/.git", "/other/proj/.git"

	// Deterministic: ensuring the same repository twice yields the same name.
	if HomeSession(a) != HomeSession(a) {
		t.Fatalf("HomeSession should be deterministic for one repository")
	}

	na, nb := HomeSession(a), HomeSession(b)
	// Collision-safe: two repos sharing the basename "proj" get distinct names.
	if na == nb {
		t.Fatalf("two repositories sharing a basename must get distinct homes, both = %q", na)
	}
	// The name is the repo basename plus the disambiguating/managed-marker hash, no prefix.
	for _, n := range []string{na, nb} {
		if !strings.HasPrefix(n, "proj-") {
			t.Fatalf("home %q should be the repo basename followed by its hash", n)
		}
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
