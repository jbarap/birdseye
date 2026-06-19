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
