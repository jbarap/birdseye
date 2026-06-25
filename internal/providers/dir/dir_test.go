package dir

import (
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
// be- home name: it is a deterministic function of the git-common-dir (the same repository
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
	// Friendly part: the basename and the be- namespace prefix are both present.
	for _, n := range []string{na, nb} {
		if !strings.HasPrefix(n, "be-proj-") {
			t.Fatalf("home %q should carry the be- prefix and the repo basename", n)
		}
	}
}

func TestCandidatesEmptyWhenNoSources(t *testing.T) {
	p := New(false, nil)
	got, err := p.Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no candidates, got %d", len(got))
	}
}
