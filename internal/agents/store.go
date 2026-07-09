package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// record is the on-disk per-session state a hook writes and the source reads.
type record struct {
	SessionID      string    `json:"session_id"`
	PID            int       `json:"pid"`
	TmuxSession    string    `json:"tmux_session"`
	TmuxWindow     string    `json:"tmux_window"`
	TmuxWindowName string    `json:"tmux_window_name"`
	TmuxPane       string    `json:"tmux_pane"`
	CWD            string    `json:"cwd"`
	Title          string    `json:"title"`
	Status         Status    `json:"status"`
	Updated        time.Time `json:"updated"`
}

// StateDir returns birdseye's state root, following the XDG state convention
// ($XDG_STATE_HOME/birdseye or ~/.local/state/birdseye). Dash view state - mutes.json and
// dash-state.json - lives directly here; per-session agent hook records live under the agents
// subdirectory (see agentsDir).
func StateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "birdseye"), nil
}

// agentsDir returns the directory holding per-session agent hook records, namespaced under the
// state root because there is one record file per agent session and they are hook-produced agent
// data rather than user/view state.
func agentsDir() (string, error) {
	root, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "agents"), nil
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

// muteFile is the single per-state-dir file holding the user's mute intents: a set of
// location keys (see locationKey) the user has muted. It is keyed by location rather than
// by session id so a mute follows the tmux location and survives a session rotating in
// place. Only the view writes it; the source reads and prunes it.
const muteFile = "mutes.json"

// readMutes loads the set of muted location keys under dir. A missing file yields an empty
// set, so mute is simply off until the user mutes something.
func readMutes(dir string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, muteFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return map[string]bool{}, nil
	}
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set, nil
}

// writeMutes persists the muted location-key set atomically under dir, sorted for a stable
// file. An empty set still writes (an empty list), so unmuting the last agent clears cleanly.
func writeMutes(dir string, set map[string]bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, muteFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// dashStateFile is the single per-state-dir file holding the dash's persisted view state. It is a
// JSON object so more view state can be added later without a format migration; only the view reads
// and writes it.
const dashStateFile = "dash-state.json"

// dashState is the dash's persisted view state. Fields are optional and additive - readers tolerate
// a missing file and unknown keys - so the shape can grow (fold state, focused lens, ...) without a
// migration. Writers read-modify-write, so setting one field preserves the rest.
type dashState struct {
	// ActiveWorkspace is the key of the last-selected namespace tab: "" for All, a configured
	// namespace name, or an automatic workspace's key.
	ActiveWorkspace string `json:"active_workspace"`
}

// readDashState loads the persisted dash view state under dir. A missing or unreadable file yields
// the zero state (All active, no folds), so the dash simply opens with defaults.
func readDashState(dir string) (dashState, error) {
	data, err := os.ReadFile(filepath.Join(dir, dashStateFile))
	if err != nil {
		if os.IsNotExist(err) {
			return dashState{}, nil
		}
		return dashState{}, err
	}
	var s dashState
	if err := json.Unmarshal(data, &s); err != nil {
		return dashState{}, nil // tolerate a corrupt file: fall back to defaults
	}
	return s, nil
}

// writeDashState persists the dash view state atomically under dir.
func writeDashState(dir string, s dashState) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, dashStateFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
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
