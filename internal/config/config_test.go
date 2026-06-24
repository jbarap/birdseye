package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if !cfg.Dir.UseZoxide {
		t.Fatal("expected zoxide on by default")
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
order = ["dir", "tmux"]

[providers]
tmuxp = false

[dir]
use_zoxide = false
roots = ["/home/u/code"]

# Retired keys from older configs must be ignored, not rejected.
[agents]
stale_after = "30s"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("a retired agents key should be ignored, not error: %v", err)
	}
	if len(cfg.Order) != 2 || cfg.Order[0] != "dir" {
		t.Fatalf("order not applied: %v", cfg.Order)
	}
	if cfg.Providers["tmuxp"] != false {
		t.Fatalf("expected tmuxp disabled")
	}
	if cfg.Dir.UseZoxide {
		t.Fatalf("expected zoxide disabled by override")
	}
	if len(cfg.Dir.Roots) != 1 || cfg.Dir.Roots[0] != "/home/u/code" {
		t.Fatalf("roots not applied: %v", cfg.Dir.Roots)
	}
}

func TestLoadExpandsTildePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to expand against")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[tmuxp]
dir = "~/.tmuxp"

[dir]
roots = ["~/code", "~", "/abs/path", "relative/path"]

[repo]
roots = ["~/projects/open_source"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".tmuxp"); cfg.Tmuxp.Dir != want {
		t.Errorf("tmuxp.dir = %q, want %q", cfg.Tmuxp.Dir, want)
	}
	wantDir := []string{filepath.Join(home, "code"), home, "/abs/path", "relative/path"}
	for i, w := range wantDir {
		if cfg.Dir.Roots[i] != w {
			t.Errorf("dir.roots[%d] = %q, want %q", i, cfg.Dir.Roots[i], w)
		}
	}
	if want := filepath.Join(home, "projects/open_source"); cfg.Repo.Roots[0] != want {
		t.Errorf("repo.roots[0] = %q, want %q", cfg.Repo.Roots[0], want)
	}
}

func TestLoadParsesAgentsKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[agents.keys]
down = ["n", "down"]
quit = ["q"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Agents.Keys["down"]
	if len(got) != 2 || got[0] != "n" || got[1] != "down" {
		t.Fatalf("agents.keys.down not parsed: %v", cfg.Agents.Keys)
	}
	if len(cfg.Agents.Keys["quit"]) != 1 || cfg.Agents.Keys["quit"][0] != "q" {
		t.Fatalf("agents.keys.quit not parsed: %v", cfg.Agents.Keys)
	}
}

func TestLoadParsesRepoRoots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[repo]
roots = ["~/code", "/work/repos"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	wantFirst := filepath.Join(home, "code")
	if len(cfg.Repo.Roots) != 2 || cfg.Repo.Roots[0] != wantFirst || cfg.Repo.Roots[1] != "/work/repos" {
		t.Fatalf("repo.roots not parsed/expanded: %v", cfg.Repo.Roots)
	}
}

func TestLoadRejectsLegacyWorktreeSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[worktree]
roots = ["~/code"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected the retired [worktree] section to be rejected")
	}
	if !contains(err.Error(), "repo") {
		t.Fatalf("error should point to the new [repo] section, got %q", err.Error())
	}
}

func TestLoadMalformedReportsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("this is = not valid = toml ]["), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for malformed config")
	}
	if want := path; !contains(err.Error(), want) {
		t.Fatalf("error should name the file %q, got %q", want, err.Error())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestAgentsRefreshAndCommand(t *testing.T) {
	// Defaults when unset.
	var a Agents
	if d, err := a.RefreshInterval(); err != nil || d.String() != "1s" {
		t.Fatalf("default refresh = (%v,%v), want 1s", d, err)
	}
	if a.AgentCommand() != "claude" {
		t.Fatalf("default command = %q, want claude", a.AgentCommand())
	}
	// Honored when set.
	a = Agents{Refresh: "2s", Command: "aider"}
	if d, err := a.RefreshInterval(); err != nil || d.String() != "2s" {
		t.Fatalf("refresh = (%v,%v), want 2s", d, err)
	}
	if a.AgentCommand() != "aider" {
		t.Fatalf("command = %q, want aider", a.AgentCommand())
	}
	// Malformed and non-positive are rejected.
	for _, bad := range []string{"nope", "0s", "-1s"} {
		if _, err := (Agents{Refresh: bad}).RefreshInterval(); err == nil {
			t.Errorf("refresh %q should be rejected", bad)
		}
	}
}
