package cli

import "testing"

// TestSelectTiersHonorsFlags checks explicit flags bypass any prompt and map straight to
// the chosen tiers.
func TestSelectTiersHonorsFlags(t *testing.T) {
	cases := []struct {
		hooks, workflows    bool
		wantHooks, wantWork bool
	}{
		{true, false, true, false},
		{false, true, false, true},
		{true, true, true, true},
	}
	for _, c := range cases {
		gotH, gotW, err := selectTiers(c.hooks, c.workflows, "install")
		if err != nil {
			t.Fatalf("flags %v/%v: unexpected error %v", c.hooks, c.workflows, err)
		}
		if gotH != c.wantHooks || gotW != c.wantWork {
			t.Fatalf("flags %v/%v: got %v/%v want %v/%v", c.hooks, c.workflows, gotH, gotW, c.wantHooks, c.wantWork)
		}
	}
}

// TestSelectTiersNonTTYRefuses checks the no-flag form errors under a non-interactive
// stdin (the test runner's) rather than prompting or installing implicitly.
func TestSelectTiersNonTTYRefuses(t *testing.T) {
	if stdinIsTTY() {
		t.Skip("stdin is a TTY in this environment; the non-TTY refusal cannot be exercised")
	}
	if _, _, err := selectTiers(false, false, "install"); err == nil {
		t.Fatal("no-flag install in a non-TTY should error asking for --hooks/--workflows")
	}
}
