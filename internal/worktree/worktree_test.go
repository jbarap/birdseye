package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/u/birds-eye.git": "birds-eye",
		"https://github.com/u/birds-eye":     "birds-eye",
		"git@github.com:u/foo.git":           "foo",
		"git@github.com:u/foo":               "foo",
		"/local/path/bar/":                   "bar",
	}
	for url, want := range cases {
		if got := RepoNameFromURL(url); got != want {
			t.Errorf("RepoNameFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestListFindsManagedRepos(t *testing.T) {
	root := t.TempDir()
	// repo "alpha" with main + wt1; "loose" without main (ignored).
	mustMkdir(t, filepath.Join(root, "alpha", "main"))
	mustMkdir(t, filepath.Join(root, "alpha", "wt1"))
	mustMkdir(t, filepath.Join(root, "loose", "stuff"))

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Repo != "alpha" {
		t.Fatalf("expected only managed repo alpha, got %+v", got)
	}
	if len(got[0].Dirs) != 2 {
		t.Fatalf("expected 2 worktree dirs, got %d", len(got[0].Dirs))
	}
}

func TestProviderCandidatesFromWorktrees(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "alpha", "main"))
	mustMkdir(t, filepath.Join(root, "alpha", "wt1"))

	cands, err := NewProvider(root).Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}
	if cands[0].Name != "alpha-main" && cands[1].Name != "alpha-main" {
		t.Fatalf("expected an alpha-main candidate, got %v", []string{cands[0].Name, cands[1].Name})
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}
