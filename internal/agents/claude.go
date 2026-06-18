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
// as agents, marking stale state as unknown.
type ClaudeSource struct {
	dir        string
	staleAfter time.Duration
	now        func() time.Time
}

// NewClaudeSource returns a source reading from the default state dir. State
// older than staleAfter (for non-terminal agents) is reported as unknown.
func NewClaudeSource(staleAfter time.Duration) (*ClaudeSource, error) {
	dir, err := StateDir()
	if err != nil {
		return nil, err
	}
	return &ClaudeSource{dir: dir, staleAfter: staleAfter, now: time.Now}, nil
}

// Agents implements Source.
func (s *ClaudeSource) Agents() ([]Agent, error) {
	recs, err := readRecords(s.dir)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]Agent, 0, len(recs))
	for _, r := range recs {
		status := r.Status
		if status == "" {
			status = StatusUnknown
		}
		// Non-terminal agents with stale state are reported as unknown.
		if status != StatusDone && s.staleAfter > 0 && now.Sub(r.Updated) > s.staleAfter {
			status = StatusUnknown
		}
		out = append(out, Agent{
			SessionID:   r.SessionID,
			TmuxSession: r.TmuxSession,
			TmuxWindow:  r.TmuxWindow,
			Title:       r.Title,
			Status:      status,
			Updated:     r.Updated,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status.rank() != out[j].Status.rank() {
			return out[i].Status.rank() < out[j].Status.rank()
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
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

	session, window := resolveTmux()
	if session != "" {
		rec.TmuxSession = session
	}
	if window != "" {
		rec.TmuxWindow = window
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

// resolveTmux returns the tmux session name and window index of the current
// pane, or empties when not running inside tmux.
func resolveTmux() (session, window string) {
	if os.Getenv("TMUX") == "" {
		return "", ""
	}
	pane := os.Getenv("TMUX_PANE")
	args := []string{"display-message", "-p"}
	if pane != "" {
		args = append(args, "-t", pane)
	}
	args = append(args, "#{session_name}\t#{window_index}")
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "", ""
}
