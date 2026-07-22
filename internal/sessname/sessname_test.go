package sessname

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

// legacyHomeSession is a verbatim copy of the pre-refactor dir.HomeSession implementation.
// Home must produce byte-identical output for every input, so a repository's home name is
// unchanged by the move and picker-open still agrees with dash-spawn.
func legacyHomeSession(gitCommonDir string) string {
	clean := filepath.Clean(gitCommonDir)
	base := strings.NewReplacer(".", "_", ":", "_", " ", "_").Replace(filepath.Base(filepath.Dir(clean)))
	sum := sha256.Sum256([]byte(clean))
	return base + "-" + hex.EncodeToString(sum[:])[:6]
}

func TestHomeMatchesLegacy(t *testing.T) {
	cases := []string{
		"/home/john/projects/birdseye/.git",
		"/home/john/projects/birds.eye/.git",
		"/srv/repos/a b c/.git",
		"/tmp/x/.git/worktrees/feature", // a linked-worktree common dir shape
		".git",
	}
	for _, gcd := range cases {
		if got, want := Home(gcd), legacyHomeSession(gcd); got != want {
			t.Errorf("Home(%q) = %q, legacy = %q", gcd, got, want)
		}
	}
}

func TestForDeterministic(t *testing.T) {
	a := For("v3", "/media/john/ssd/datasets/court/v3")
	b := For("v3", "/media/john/ssd/datasets/court/v3")
	if a != b {
		t.Fatalf("For not deterministic: %q != %q", a, b)
	}
}

func TestForDistinguishesSameBaseDifferentPath(t *testing.T) {
	x := For("v3", "/home/john/a/v3")
	y := For("v3", "/home/john/b/v3")
	if x == y {
		t.Fatalf("distinct paths sharing a base collided: both %q", x)
	}
	// Both keep the friendly base prefix.
	for _, n := range []string{x, y} {
		if !strings.HasPrefix(n, "v3-") {
			t.Errorf("name %q lost its base prefix", n)
		}
	}
}

func TestForDistinguishesSanitizerFolds(t *testing.T) {
	// Bases that fold to the same fragment must still yield distinct names via the identity
	// hash, closing the lossy-sanitize collision channel.
	names := map[string]bool{}
	for _, base := range []string{"v.3", "v:3", "v 3"} {
		if Sanitize(base) != "v_3" {
			t.Fatalf("Sanitize(%q) = %q, want v_3", base, Sanitize(base))
		}
		n := For(base, "/root/"+base)
		if names[n] {
			t.Fatalf("collision on %q", n)
		}
		names[n] = true
	}
}

func TestSanitizeFolds(t *testing.T) {
	if got := Sanitize("a.b:c d"); got != "a_b_c_d" {
		t.Errorf("Sanitize = %q, want a_b_c_d", got)
	}
	if got := Sanitize("already_clean"); got != "already_clean" {
		t.Errorf("Sanitize not idempotent: %q", got)
	}
}
