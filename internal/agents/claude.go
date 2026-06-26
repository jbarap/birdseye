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
	"unicode/utf8"
)

// ClaudeSource reads agent state written by Claude Code hooks and presents it as
// agents. An entry exists exactly as long as the Claude process that wrote it is
// still running: on each read, records whose process is gone are deleted. There is
// no time-based retention and no stale heuristic — process liveness is the single
// signal for whether an agent is still there.
type ClaudeSource struct {
	dir      string
	alive    func(pid int) bool                // process-liveness check; overridable in tests
	titles   func() (map[string]string, error) // pane id -> OSC title; overridable in tests
	detector levelDetector                     // derives the live working/idle level
}

// NewClaudeSource returns a source reading from the default state dir, using real
// process liveness to decide which tracked agents are still present. The live
// working/idle level is read from pane titles; the CLI wires a tmux-backed title
// source via SetTitleSource, and absent that the source degrades to hook-only status.
func NewClaudeSource() (*ClaudeSource, error) {
	dir, err := StateDir()
	if err != nil {
		return nil, err
	}
	return &ClaudeSource{
		dir:      dir,
		alive:    processAlive,
		titles:   func() (map[string]string, error) { return map[string]string{}, nil },
		detector: titleLevelDetector{},
	}, nil
}

// SetTitleSource installs the provider that yields pane id -> OSC title, used to correct
// the live working/idle level. The CLI wires this to tmux (Client.PaneTitles). Left
// unset, the source falls back to hook-written status, so behavior is never worse than
// hook-only.
func (s *ClaudeSource) SetTitleSource(f func() (map[string]string, error)) {
	s.titles = f
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
	// Read every pane's current title once per refresh (batched, see PaneTitles); the
	// detector resolves each agent's live working/idle level from it. Best-effort: a
	// failure leaves an empty map, so status falls back to the hook record.
	titles := map[string]string{}
	if s.titles != nil {
		if t, err := s.titles(); err == nil && t != nil {
			titles = t
		}
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
		// Correct working/idle from the agent's current terminal state: the hook status
		// is edge-triggered and goes stale (e.g. "working" after an Esc interrupt with no
		// Stop hook), while the live level reflects the pane now. reconcile encodes the
		// precedence (the needs-attention latch, the title overriding working/idle, the
		// no-signal fallback).
		if s.detector != nil {
			level, ok := s.detector.Level(r.TmuxPane, titles)
			status = reconcile(status, level, ok)
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

// claudeTitleLevel maps a pane's current OSC title to a live working/idle level for
// Claude, or ("", false) when the title carries no recognized state glyph. Claude
// broadcasts its state in the title it sets: a Braille spinner frame while working, a
// sparkle when idle. We match the Braille *class* (U+2800-U+28FF), not a specific frame,
// because the spinner animates across that range (~1 Hz observed) while working - a
// single-frame match would flicker working -> no-signal every tick. These glyphs are a
// Claude-version detail; together with idleNotification this is the one spot to update if
// Claude changes them.
func claudeTitleLevel(title string) (Status, bool) {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(title))
	switch {
	case r >= 0x2800 && r <= 0x28FF: // Braille pattern: a spinner frame
		return StatusWorking, true
	case r == 0x2733: // ✳ sparkle: idle at the prompt
		return StatusIdle, true
	default:
		return "", false
	}
}

// levelDetector reports an agent's live working/idle level from its current terminal
// state, independent of hook events. ok=false means "no signal" - the caller falls back
// to the hook-written status. The seam keeps status reconciliation agnostic to how a
// level was derived, so title-less agents can later be served by other detectors
// (terminal-activity diffing, buffer matching) without touching the reconciler or view.
type levelDetector interface {
	Level(paneID string, titles map[string]string) (Status, bool)
}

// titleLevelDetector reads the level from a pane's OSC title via claudeTitleLevel. It is
// the only detector today; it needs no signal for an agent with no pane.
type titleLevelDetector struct{}

func (titleLevelDetector) Level(paneID string, titles map[string]string) (Status, bool) {
	if paneID == "" {
		return "", false
	}
	return claudeTitleLevel(titles[paneID])
}

// reconcile resolves an agent's displayed status from its hook-written status and a live
// working/idle level. done always wins. needs-attention is a latch: it holds until the
// level shows the agent working (it resumed, so the user answered) - an idle or absent
// level does not clear it, keeping a real pending prompt visible. For every other hook
// status the live level overrides working/idle when present, and we fall back to the hook
// status when no level is available.
func reconcile(hookStatus, level Status, haveLevel bool) Status {
	switch hookStatus {
	case StatusDone:
		return StatusDone
	case StatusNeedsAttention:
		if haveLevel && level == StatusWorking {
			return StatusWorking
		}
		return StatusNeedsAttention
	default:
		if haveLevel {
			return level
		}
		return hookStatus
	}
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
