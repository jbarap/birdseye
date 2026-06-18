package agents

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClaudeSource reads agent state written by Claude Code hooks and presents it
// as agents, marking stale state as unknown and pruning aged-out state files.
type ClaudeSource struct {
	dir         string
	staleAfter  time.Duration
	forgetDone  time.Duration
	forgetStale time.Duration
	now         func() time.Time
}

// NewClaudeSource returns a source reading from the default state dir. State
// older than staleAfter (for non-terminal agents) is reported as unknown. Ended
// (done) sessions are pruned after forgetDone and non-terminal sessions with no
// fresh updates after forgetStale; a zero TTL disables that pruning.
func NewClaudeSource(staleAfter, forgetDone, forgetStale time.Duration) (*ClaudeSource, error) {
	dir, err := StateDir()
	if err != nil {
		return nil, err
	}
	return &ClaudeSource{
		dir:         dir,
		staleAfter:  staleAfter,
		forgetDone:  forgetDone,
		forgetStale: forgetStale,
		now:         time.Now,
	}, nil
}

// Agents implements Source.
func (s *ClaudeSource) Agents() ([]Agent, error) {
	recs, err := readRecords(s.dir)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]Agent, 0, len(recs))
	at := map[string]int{} // dedup key -> index in out
	for _, r := range recs {
		// Garbage-collect aged-out state: delete the file and drop the agent.
		if s.expired(r, now) {
			_ = os.Remove(filepath.Join(s.dir, sanitize(r.SessionID)+".json"))
			continue
		}
		status := r.Status
		if status == "" {
			status = StatusUnknown
		}
		// Non-terminal agents with stale state are reported as unknown.
		if status != StatusDone && s.staleAfter > 0 && now.Sub(r.Updated) > s.staleAfter {
			status = StatusUnknown
		}
		a := Agent{
			SessionID:   r.SessionID,
			TmuxSession: r.TmuxSession,
			TmuxWindow:  r.TmuxWindow,
			TmuxPane:    r.TmuxPane,
			Title:       r.Title,
			Status:      status,
			Updated:     r.Updated,
		}
		// Multiple Claude sessions can map to one tmux pane (e.g. restarting
		// Claude in place). Show one row per location, keeping the most recent.
		key := dedupKey(a)
		if i, ok := at[key]; ok {
			if a.Updated.After(out[i].Updated) {
				out[i] = a
			}
			continue
		}
		at[key] = len(out)
		out = append(out, a)
	}
	out = dropSupersededByPane(out)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status.rank() != out[j].Status.rank() {
			return out[i].Status.rank() < out[j].Status.rank()
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

// expired reports whether a record has aged past its retention window: ended
// (done) records use forgetDone, all others forgetStale. A zero TTL disables
// pruning for that class.
func (s *ClaudeSource) expired(r record, now time.Time) bool {
	ttl := s.forgetStale
	if r.Status == StatusDone {
		ttl = s.forgetDone
	}
	return ttl > 0 && now.Sub(r.Updated) > ttl
}

// dedupKey identifies the tmux location an agent occupies, so records that share
// a pane (or window, for older records without a pane id) collapse to one row.
// Agents with no tmux location stay distinct, keyed by their own session id.
func dedupKey(a Agent) string {
	switch {
	case a.TmuxPane != "":
		return "pane:" + a.TmuxPane
	case a.TmuxSession != "" && a.TmuxWindow != "":
		return "win:" + a.TmuxSession + ":" + a.TmuxWindow
	case a.TmuxSession != "":
		return "sess:" + a.TmuxSession
	default:
		return "id:" + a.SessionID
	}
}

// dropSupersededByPane removes pre-upgrade records that lack a pane id when a
// pane-keyed agent already occupies the same window. Without this, a leftover
// "done" record (keyed by window) would show alongside the live pane-keyed agent
// for the same window. Genuine concurrent agents each carry their own pane id, so
// they are unaffected.
func dropSupersededByPane(in []Agent) []Agent {
	hasPane := map[string]bool{}
	for _, a := range in {
		if a.TmuxPane != "" && a.TmuxSession != "" {
			hasPane[a.TmuxSession+"\x00"+a.TmuxWindow] = true
		}
	}
	out := in[:0]
	for _, a := range in {
		if a.TmuxPane == "" && a.TmuxSession != "" && hasPane[a.TmuxSession+"\x00"+a.TmuxWindow] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// StatusForEvent maps a Claude Code hook event name to a status.
//
// The mapping is deliberately small and may be tuned (see design.md open
// questions):
//
//	SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, SubagentStop -> working
//	Notification                                                          -> needs-attention
//	Stop                                                                  -> idle
//	SessionEnd                                                            -> done
func StatusForEvent(event string) Status {
	switch event {
	case "Notification":
		return StatusNeedsAttention
	case "Stop":
		return StatusIdle
	case "SessionEnd":
		return StatusDone
	case "SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "SubagentStop":
		return StatusWorking
	default:
		return StatusWorking
	}
}

// hookInput is the subset of the Claude Code hook payload we consume.
type hookInput struct {
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
}

// HandleHook processes one hook invocation: it reads the hook payload from r,
// resolves the tmux location, and writes the updated state record.
func HandleHook(event string, r io.Reader) error {
	dir, err := StateDir()
	if err != nil {
		return err
	}
	var in hookInput
	if data, _ := io.ReadAll(r); len(data) > 0 {
		_ = json.Unmarshal(data, &in)
	}
	if in.SessionID == "" {
		in.SessionID = os.Getenv("CLAUDE_SESSION_ID")
	}

	rec, _ := readRecord(dir, in.SessionID)
	rec.SessionID = in.SessionID
	rec.Status = StatusForEvent(event)
	rec.Updated = time.Now()

	session, window, pane := resolveTmux()
	if session != "" {
		rec.TmuxSession = session
	}
	if window != "" {
		rec.TmuxWindow = window
	}
	if pane != "" {
		rec.TmuxPane = pane
	}
	rec.Title = title(rec.TmuxSession, in.CWD)
	return writeRecord(dir, rec)
}

func title(session, cwd string) string {
	switch {
	case session != "" && cwd != "":
		return session + ":" + filepath.Base(cwd)
	case session != "":
		return session
	case cwd != "":
		return filepath.Base(cwd)
	default:
		return "agent"
	}
}

// resolveTmux returns the tmux session name, window index, and pane id of the
// current pane, or empties when not running inside tmux. The pane id pins the
// exact pane the agent runs in, even when its window is split.
func resolveTmux() (session, window, pane string) {
	if os.Getenv("TMUX") == "" {
		return "", "", ""
	}
	target := os.Getenv("TMUX_PANE")
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, "#{session_name}\t#{window_index}\t#{pane_id}")
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 3)
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return parts[0], parts[1], ""
	case 1:
		return parts[0], "", ""
	default:
		return "", "", ""
	}
}
