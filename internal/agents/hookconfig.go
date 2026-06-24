package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManagedEvents are the Claude Code hook events birdseye installs so that
// `be agents` can show live status.
var ManagedEvents = []string{
	"SessionStart",
	"UserPromptSubmit",
	"Notification",
	"Stop",
	"SessionEnd",
}

// ClaudeConfigDir returns the Claude Code config root, ~/.claude — the single place
// the settings path and the workflow artifacts are resolved from, so a layout change is
// a one-line fix.
func ClaudeConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// DefaultSettingsPath returns the standard Claude Code settings path,
// ~/.claude/settings.json.
func DefaultSettingsPath() (string, error) {
	dir, err := ClaudeConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// HooksInstalled reports whether any birdseye hook entry is present in the settings
// file at path — a non-destructive check for the uninstall checklist. A missing file
// reports false; a malformed file is an error (mirroring the install refusal).
func HooksInstalled(path string) (bool, error) {
	settings, before, err := loadSettings(path)
	if err != nil {
		return false, err
	}
	if before == nil {
		return false, nil
	}
	for _, raw := range asObject(settings["hooks"]) {
		for _, g := range asArray(raw) {
			for _, e := range asArray(asObject(g)["hooks"]) {
				if isOurCommand(commandOf(e)) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// InstallHooks merges birdseye's hook commands into the Claude settings file
// at path, preserving every other key and any unrelated hooks. command is the
// program invocation used in the hook (e.g. the absolute path to the be
// binary). It is idempotent and reports whether the file changed.
func InstallHooks(path, command string) (changed bool, err error) {
	settings, before, err := loadSettings(path)
	if err != nil {
		return false, err
	}
	hooks := asObject(settings["hooks"])
	for _, event := range ManagedEvents {
		groups := asArray(hooks[event])
		// Drop any stale birdseye entries first so a changed command path is
		// refreshed rather than duplicated.
		groups, _ = stripOurHooks(groups)
		groups = append(groups, ourGroup(command, event))
		hooks[event] = groups
	}
	settings["hooks"] = hooks
	return saveIfChanged(path, settings, before)
}

// UninstallHooks removes only birdseye's hook entries from the settings file
// at path, leaving every other key and unrelated hook intact. It is idempotent
// and reports whether the file changed.
func UninstallHooks(path string) (changed bool, err error) {
	settings, before, err := loadSettings(path)
	if err != nil {
		return false, err
	}
	if before == nil {
		return false, nil // nothing to uninstall; file absent
	}
	hooks := asObject(settings["hooks"])
	for event, raw := range hooks {
		groups, _ := stripOurHooks(asArray(raw))
		if len(groups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = groups
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
	return saveIfChanged(path, settings, before)
}

// ourGroup builds a hook group invoking `be hook claude record <event>`.
func ourGroup(command, event string) map[string]any {
	return map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command + " hook claude record " + event,
			},
		},
	}
}

// stripOurHooks removes birdseye command entries from each group, dropping a
// group once it has no hooks left. User hooks sharing a group are preserved.
func stripOurHooks(groups []any) (out []any, removed bool) {
	for _, g := range groups {
		gm := asObject(g)
		entries := asArray(gm["hooks"])
		kept := make([]any, 0, len(entries))
		for _, e := range entries {
			if isOurCommand(commandOf(e)) {
				removed = true
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			// Whole group was ours (or empty): drop it.
			if len(entries) > 0 {
				removed = true
			}
			continue
		}
		gm["hooks"] = kept
		out = append(out, gm)
	}
	return out, removed
}

// isOurCommand reports whether a hook command string is a birdseye hook call,
// matching both `be hook X` and `/abs/path/be hook X`.
func isOurCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return false
	}
	return filepath.Base(fields[0]) == "be" && fields[1] == "hook"
}

func commandOf(entry any) string {
	m := asObject(entry)
	if s, ok := m["command"].(string); ok {
		return s
	}
	return ""
}

func asObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asArray(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

// loadSettings reads and parses the settings file. A missing file yields an
// empty object and a nil before-snapshot. A malformed file is a clear error.
func loadSettings(path string) (settings map[string]any, before []byte, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil, nil
		}
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, data, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, nil, fmt.Errorf("%s is not valid JSON; refusing to modify it: %w", path, err)
	}
	return settings, data, nil
}

// saveIfChanged writes settings only when the marshaled result differs from
// before, backing up any existing file first. It writes atomically.
func saveIfChanged(path string, settings map[string]any, before []byte) (changed bool, err error) {
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	// Compare against the existing file content (normalized to the same form
	// would be ideal, but a byte compare avoids needless writes in the common
	// already-applied case).
	if before != nil && bytes.Equal(bytes.TrimRight(before, "\n"), bytes.TrimRight(out, "\n")) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if before != nil {
		if err := os.WriteFile(path+".bak", before, 0o644); err != nil {
			return false, fmt.Errorf("writing backup %s.bak: %w", path, err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, err
	}
	return true, nil
}
