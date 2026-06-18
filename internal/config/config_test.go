package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Agents.StaleAfter.AsDuration() != 5*time.Minute {
		t.Fatalf("expected default stale_after, got %v", cfg.Agents.StaleAfter.AsDuration())
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

[agents]
stale_after = "30s"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
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
	if cfg.Agents.StaleAfter.AsDuration() != 30*time.Second {
		t.Fatalf("stale_after not applied: %v", cfg.Agents.StaleAfter.AsDuration())
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
