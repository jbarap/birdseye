package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNotifyDefaults pins that an absent [agents.notify] table means both edges notify with
// automatic delivery.
func TestNotifyDefaults(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	n := cfg.Agents.Notify
	if !n.NeedsAttention {
		t.Error("needs-attention should default on")
	}
	if !n.Finished {
		t.Error("finished should default on")
	}
	if n.Command != "" {
		t.Errorf("command should default empty, got %q", n.Command)
	}
}

// TestNotifyExplicitFalseWins pins that an explicit false in the file overrides the default
// true - the whole reason the flags are pointers.
func TestNotifyExplicitFalseWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := "[agents.notify]\nfinished = false\ncommand = \"notify-send x\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	n := cfg.Agents.Notify
	if n.Finished {
		t.Error("finished=false should disable the finished edge")
	}
	if !n.NeedsAttention {
		t.Error("an unset needs_attention should stay on when finished is set false")
	}
	if n.Command != "notify-send x" {
		t.Errorf("command not parsed, got %q", n.Command)
	}
}

// TestNotifyUnknownKeyRejected pins that strict loading rejects a typo'd key nested under
// [agents.notify], like everywhere else in the config.
func TestNotifyUnknownKeyRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := "[agents.notify]\nfinishd = true\n" // typo
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected an unknown [agents.notify] key to be rejected")
	}
	if !strings.Contains(err.Error(), "finishd") {
		t.Errorf("error should name the offending key, got %v", err)
	}
}
