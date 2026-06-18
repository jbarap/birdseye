package dir

import "testing"

func TestSessionNameSanitizes(t *testing.T) {
	cases := map[string]string{
		"my.project":  "my_project",
		"a:b":         "a_b",
		"with space":  "with_space",
		"plain":       "plain",
	}
	for in, want := range cases {
		if got := SessionName(in); got != want {
			t.Errorf("SessionName(%q) = %q, want %q", in, got, want)
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
