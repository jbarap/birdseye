package agents

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ClaudeSource reads agent state written by Claude Code hooks and presents it as
// agents. An entry exists exactly as long as the Claude process that wrote it is
// still running: on each read, records whose process is gone are deleted. There is
// no time-based retention and no stale heuristic — process liveness is the single
// signal for whether an agent is still there.
type ClaudeSource struct {
	dir   string
	alive func(pid int) bool // process-liveness check; overridable in tests
}

// NewClaudeSource returns a source reading from the default state dir, using real
// process liveness to decide which tracked agents are still present.
func NewClaudeSource() (*ClaudeSource, error) {
	dir, err := StateDir()
	if err != nil {
		return nil, err
	}
	return &ClaudeSource{dir: dir, alive: processAlive}, nil
}

// processAlive reports whether a process with the given pid is currently running.
// A non-positive pid (e.g. a record predating pid tracking) is treated as not
// running, so such records are reclaimed. EPERM means the process exists but is
// owned by another user — still alive.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// sessionPID returns the pid of the Claude session process that owns this hook
// invocation. Claude Code runs each hook in a short-lived shell, so os.Getppid()
// is that shell — it exits the instant the hook returns, which would make a live
// agent's recorded pid dead within milliseconds. The durable handle is the
// nearest ancestor that is the Claude process itself, so we walk up to it. If no
// Claude ancestor is found (an unexpected install), fall back to the immediate
// parent as a best effort.
func sessionPID() int {
	parent := os.Getppid()
	if pid, ok := walkToClaude(parent, procParent); ok {
		return pid
	}
	return parent
}

// walkToClaude walks the ancestry starting at pid, returning the first ancestor
// (including the start) whose command is the Claude binary. parent reports a
// pid's parent pid and command name; the bound and the pid<=1 guard keep the walk
// finite even if the chain is cyclic or rooted at init.
func walkToClaude(start int, parent func(int) (ppid int, comm string)) (int, bool) {
	for cur, hops := start, 0; cur > 1 && hops < 32; hops++ {
		ppid, comm := parent(cur)
		if isClaudeComm(comm) {
			return cur, true
		}
		if ppid <= 0 {
			break
		}
		cur = ppid
	}
	return 0, false
}

// isClaudeComm reports whether a process command name is the Claude Code binary.
func isClaudeComm(comm string) bool {
	return filepath.Base(strings.TrimSpace(comm)) == "claude"
}

// procParent reports a pid's parent pid and command name via ps, which works on
// both Linux and macOS (the tool's domain). It is a package var so tests can
// supply a synthetic process tree. A failed lookup yields (0, "").
var procParent = func(pid int) (int, string) {
	out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, ""
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return 0, ""
	}
	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, ""
	}
	return ppid, strings.Join(fields[1:], " ")
}

// Agents implements Source.
func (s *ClaudeSource) Agents() ([]Agent, error) {
	recs, err := readRecords(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]Agent, 0, len(recs))
	at := map[string]int{} // dedup key -> index in out
	for _, r := range recs {
		// Liveness GC: an agent exists only while the process that reported it is
		// running. A dead (or unidentifiable) process means the session is gone, so
		// delete its state — even if its tmux pane (a leftover shell) still lingers.
		if !s.alive(r.PID) {
			_ = os.Remove(filepath.Join(s.dir, sanitize(r.SessionID)+".json"))
			continue
		}
		status := r.Status
		if status == "" {
			status = StatusUnknown
		}
		a := Agent{
			SessionID:      r.SessionID,
			TmuxSession:    r.TmuxSession,
			TmuxWindow:     r.TmuxWindow,
			TmuxWindowName: r.TmuxWindowName,
			TmuxPane:       r.TmuxPane,
			CWD:            r.CWD,
			Title:          r.Title,
			Status:         status,
			Updated:        r.Updated,
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

// idleNotification is the lowercase substring that identifies Claude Code's
// periodic idle nudge ("Claude is waiting for your input"), as opposed to a
// permission/approval prompt. It is matched as a substring, not an exact
// string, so it survives small wording changes (e.g. a trailing duration);
// if Claude reword its idle copy, this is the one line to update.
const idleNotification = "waiting for your input"

// StatusFor maps a Claude Code hook event — together with its parsed payload —
// to a status. The event name is the primary signal; the payload only refines
// the cases where the name alone is ambiguous.
//
//	SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, SubagentStop -> working
//	Notification (idle "waiting for your input" message)                  -> idle
//	Notification (permission/approval or any unrecognized message)        -> needs-attention
//	Stop                                                                  -> idle
//	SessionEnd                                                            -> done
//
// The Notification split is deliberately downgrade-only and conservative: only a
// recognized idle message becomes idle; everything else stays needs-attention,
// so an unfamiliar notification still reaches the user rather than being hidden.
func StatusFor(event string, in hookInput) Status {
	switch event {
	case "Notification":
		if strings.Contains(strings.ToLower(in.Message), idleNotification) {
			return StatusIdle
		}
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

// hookInput is the subset of the Claude Code hook payload we consume. Message is
// the human notification text carried by Notification events, used to tell an
// idle nudge apart from a permission prompt.
type hookInput struct {
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
	Message   string `json:"message"`
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
	rec.Status = StatusFor(event, in)
	rec.Updated = time.Now()
	// Record the Claude session process so `be agents` can treat that process's
	// liveness as the agent's liveness (a closed Claude → its entry is reclaimed).
	// The hook runs in a short-lived shell, so we walk up to the Claude process
	// rather than recording our immediate (ephemeral) parent.
	rec.PID = sessionPID()

	session, window, windowName, pane := resolveTmux()
	if session != "" {
		rec.TmuxSession = session
	}
	if window != "" {
		rec.TmuxWindow = window
	}
	if windowName != "" {
		rec.TmuxWindowName = windowName
	}
	if pane != "" {
		rec.TmuxPane = pane
	}
	// Persist the agent's working directory: the reconciler resolves it to a
	// repository for recognition, so it must survive on the record, not only feed
	// the display title. The hook reports it on every event; an empty value (an
	// unexpected payload) leaves any previously recorded directory intact.
	if in.CWD != "" {
		rec.CWD = in.CWD
	}
	rec.Title = title(rec.TmuxSession, rec.CWD)
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

// resolveTmux returns the tmux session name, window index, window name, and pane
// id of the current pane, or empties when not running inside tmux. The window index
// is used for targeting (stable) and the name for display; the pane id pins the
// exact pane the agent runs in, even when its window is split.
func resolveTmux() (session, window, windowName, pane string) {
	if os.Getenv("TMUX") == "" {
		return "", "", "", ""
	}
	target := os.Getenv("TMUX_PANE")
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, "#{session_name}\t#{window_index}\t#{window_name}\t#{pane_id}")
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", "", "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2], parts[3]
}
