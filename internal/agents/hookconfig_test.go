package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	return m
}

func TestInstallCreatesFileWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	changed, err := InstallHooks(path, "/usr/local/bin/be")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed=true when creating file")
	}
	m := readJSON(t, path)
	hooks := m["hooks"].(map[string]any)
	for _, ev := range ManagedEvents {
		if _, ok := hooks[ev]; !ok {
			t.Errorf("missing managed event %q", ev)
		}
	}
}

func TestInstallPreservesExistingConfigAndHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{
  "model": "claude-opus-4-8",
  "permissions": {"allow": ["Bash"]},
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "/usr/bin/notify-send done"}]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo guard"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallHooks(path, "be"); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, path)

	// Unrelated top-level keys preserved.
	if m["model"] != "claude-opus-4-8" {
		t.Errorf("model key not preserved: %v", m["model"])
	}
	if _, ok := m["permissions"]; !ok {
		t.Error("permissions key not preserved")
	}

	hooks := m["hooks"].(map[string]any)
	// User's unrelated PreToolUse hook untouched.
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("user's PreToolUse hook was dropped")
	}
	// User's notify-send Stop hook preserved alongside ours.
	stop := hooks["Stop"].([]any)
	var sawUser, sawOurs bool
	for _, g := range stop {
		for _, e := range asArray(asObject(g)["hooks"]) {
			cmd := commandOf(e)
			if cmd == "/usr/bin/notify-send done" {
				sawUser = true
			}
			if cmd == "be hook Stop" {
				sawOurs = true
			}
		}
	}
	if !sawUser {
		t.Error("user's Stop hook was lost")
	}
	if !sawOurs {
		t.Error("our Stop hook was not added")
	}

	// A backup of the original was written.
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("expected backup file: %v", err)
	}
}

func TestInstallIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := InstallHooks(path, "be"); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallHooks(path, "be")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second identical install should report no change")
	}
}

func TestInstallRefreshesChangedCommandPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := InstallHooks(path, "/old/be"); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallHooks(path, "/new/be"); err != nil {
		t.Fatal(err)
	}
	hooks := readJSON(t, path)["hooks"].(map[string]any)
	stop := hooks["Stop"].([]any)
	count := 0
	for _, g := range stop {
		for _, e := range asArray(asObject(g)["hooks"]) {
			if isOurCommand(commandOf(e)) {
				count++
				if commandOf(e) != "/new/be hook Stop" {
					t.Errorf("expected refreshed command, got %q", commandOf(e))
				}
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one of our Stop hooks after refresh, got %d", count)
	}
}

func TestUninstallRemovesOnlyOurs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := `{
  "model": "x",
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "/usr/bin/notify-send done"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallHooks(path, "be"); err != nil {
		t.Fatal(err)
	}
	if _, err := UninstallHooks(path); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, path)
	if m["model"] != "x" {
		t.Error("unrelated key lost during uninstall")
	}
	hooks := m["hooks"].(map[string]any)
	// Only the user's notify-send Stop hook should remain; no managed events.
	for _, ev := range ManagedEvents {
		if ev == "Stop" {
			continue
		}
		if _, ok := hooks[ev]; ok {
			t.Errorf("managed event %q should be gone after uninstall", ev)
		}
	}
	stop, ok := hooks["Stop"].([]any)
	if !ok || len(stop) != 1 {
		t.Fatalf("user's Stop hook should remain, got %v", hooks["Stop"])
	}
	if commandOf(asArray(asObject(stop[0])["hooks"])[0]) != "/usr/bin/notify-send done" {
		t.Error("user's Stop command was altered")
	}
}

func TestMalformedSettingsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallHooks(path, "be"); err == nil {
		t.Fatal("expected error on malformed settings, got nil")
	}
}

func TestIsOurCommand(t *testing.T) {
	yes := []string{"be hook Stop", "/usr/local/bin/be hook Notification", "  be   hook   SessionEnd  "}
	no := []string{"notify-send done", "maybe hookworm", "be agents", ""}
	for _, c := range yes {
		if !isOurCommand(c) {
			t.Errorf("expected %q to be ours", c)
		}
	}
	for _, c := range no {
		if isOurCommand(c) {
			t.Errorf("expected %q not to be ours", c)
		}
	}
}
