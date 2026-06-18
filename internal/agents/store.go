package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// record is the on-disk per-session state a hook writes and the source reads.
type record struct {
	SessionID   string    `json:"session_id"`
	TmuxSession string    `json:"tmux_session"`
	TmuxWindow  string    `json:"tmux_window"`
	TmuxPane    string    `json:"tmux_pane"`
	Title       string    `json:"title"`
	Status      Status    `json:"status"`
	Updated     time.Time `json:"updated"`
}

// StateDir returns the directory where agent state files live, following the
// XDG state convention ($XDG_STATE_HOME or ~/.local/state).
func StateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "birds-eye", "agents"), nil
}

// sanitize makes a session id safe as a file name.
func sanitize(id string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	out := r.Replace(id)
	if out == "" {
		out = "unknown"
	}
	return out
}

// writeRecord persists a record atomically under dir, keyed by session id.
func writeRecord(dir string, r record) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, sanitize(r.SessionID)+".json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// readRecord loads and updates an existing record for a session, or returns a
// zero record when none exists.
func readRecord(dir, sessionID string) (record, error) {
	path := filepath.Join(dir, sanitize(sessionID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return record{SessionID: sessionID}, nil
		}
		return record{}, err
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		return record{SessionID: sessionID}, nil
	}
	return r, nil
}

// readRecords loads all records under dir. A missing dir yields no records.
func readRecords(dir string) ([]record, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r record
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
