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
	SessionID string `json:"session_id"`
	PID       int    `json:"pid"`
	// PIDStart is the anchor process's start-time token (Linux /proc stat field 22), fixed for
	// that process's life. The liveness GC compares it against the live pid's current token so a
	// recycled pid does not keep a dead agent's record alive. Empty on platforms without /proc or
	// on records predating this field, where the GC degrades to a bare liveness check.
	PIDStart string `json:"pid_start,omitempty"`
	// Kind is Claude Code's session kind (CLAUDE_CODE_SESSION_KIND), e.g. "bg" for a
	// background/daemon session. Empty for an ordinary interactive session. It scopes the
	// SessionEnd-terminal GC (a bg session's anchor may be shared, so it cannot rely on pid
	// death) and lets the view mark detached sessions.
	Kind           string `json:"kind,omitempty"`
	TmuxSession    string `json:"tmux_session"`
	TmuxWindow     string `json:"tmux_window"`
	TmuxWindowName string `json:"tmux_window_name"`
	TmuxPane       string `json:"tmux_pane"`
	CWD            string `json:"cwd"`
	Title          string `json:"title"`
	Status         Status `json:"status"`
	// Background is the in-flight background work (subagents, backgrounded shells, ...) the
	// session reported at its last turn end. Non-empty means the session yielded its turn to
	// work that will wake it, so it is still working; it also buys the pane a longer stillness
	// window, since a session parked on a subagent legitimately renders nothing.
	Background []BackgroundTask `json:"background,omitempty"`
	// BackgroundAt is when a turn end last asserted that set. Only turn-end events state what is
	// in flight, so between them the set is a claim of unknown age: this timestamp is what lets a
	// reader tell a fresh claim from one the session has since outlived. Nil when nothing is in
	// flight - a pointer so the key is absent from such a record rather than carrying a zero time.
	BackgroundAt *time.Time `json:"background_at,omitempty"`
	Updated      time.Time  `json:"updated"`
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

// notifyBatchFile is the single per-state-dir file accumulating the titles of agents that have
// finished (working -> idle) while a run is still active, so the run-settled digest can name them.
// It lives at the state root beside mutes.json because it is birdseye-owned derived state written
// across independent hook processes, not per-session data. It is best-effort like the notification
// itself: a lost or duplicated entry only skews one digest's list, never blocks work.
const notifyBatchFile = "notify-batch.json"

// batchEntry is one held finish: the agent's title and when it was recorded. The digest flushes
// only on a finish edge; if the last worker dies without one (a SIGKILL, a crash), a batch would
// otherwise persist forever and dump a day-old list of unrelated titles into the next digest.
// The timestamp lets takeNotifyBatch shed entries past batchEntryTTL instead.
type batchEntry struct {
	Title string    `json:"title"`
	At    time.Time `json:"at"`
}

// batchEntryTTL bounds how long a held finish survives in the pending digest batch. A run whose
// last worker never re-fires a finish edge sheds entries older than this at flush time, so a stuck
// batch degrades to a few missing names rather than a dump of ancient history.
const batchEntryTTL = 24 * time.Hour

// appendNotifyBatch records one finished agent's title, stamped now, in the pending digest batch. It
// read-modify-writes the file atomically; a concurrent hook can still race and drop an entry, which
// is acceptable for a best-effort digest.
func appendNotifyBatch(dir, title string) error {
	entries := append(readNotifyBatch(dir), batchEntry{Title: title, At: time.Now()})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, notifyBatchFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// readNotifyBatch loads the pending digest batch. A missing or corrupt file yields no entries, so
// the batch simply starts empty.
func readNotifyBatch(dir string) []batchEntry {
	data, err := os.ReadFile(filepath.Join(dir, notifyBatchFile))
	if err != nil {
		return nil
	}
	var entries []batchEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// takeNotifyBatch reads the pending digest batch, clears it, and returns the titles of entries
// still within batchEntryTTL (in append order), so the flushed run's finishes are not carried into
// the next run's digest and ancient stuck entries are dropped rather than dumped.
func takeNotifyBatch(dir string) []string {
	entries := readNotifyBatch(dir)
	_ = os.Remove(filepath.Join(dir, notifyBatchFile))
	cutoff := time.Now().Add(-batchEntryTTL)
	var titles []string
	for _, e := range entries {
		if e.At.Before(cutoff) {
			continue
		}
		titles = append(titles, e.Title)
	}
	return titles
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
