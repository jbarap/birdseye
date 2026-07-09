package agents

import (
	"bytes"
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

	"github.com/jbarap/birdseye/internal/config"
)

// ClaudeSource reads agent state written by Claude Code hooks and presents it as
// agents. An entry exists exactly as long as the Claude process that wrote it is
// still running: on each read, records whose process is gone are deleted. There is
// no time-based retention and no stale heuristic — process liveness is the single
// signal for whether an agent is still there.
type ClaudeSource struct {
	dir       string                            // state root: mutes.json (dash view state)
	recDir    string                            // agents subdir: per-session hook records
	alive     func(pid int) bool                // process-liveness check; overridable in tests
	startTime func(pid int) string              // anchor pid start time, for pid-reuse detection; overridable in tests
	titles    func() (map[string]string, error) // pane id -> OSC title; overridable in tests
	detector  levelDetector                     // derives the live working/idle level
}

// NewClaudeSource returns a source reading from the default state dir, using real
// process liveness to decide which tracked agents are still present. The live
// working/idle level is read from pane titles; the CLI wires a tmux-backed title
// source via SetTitleSource, and absent that the source degrades to hook-only status.
func NewClaudeSource() (*ClaudeSource, error) {
	root, err := StateDir()
	if err != nil {
		return nil, err
	}
	recDir, err := agentsDir()
	if err != nil {
		return nil, err
	}
	return &ClaudeSource{
		dir:       root,
		recDir:    recDir,
		alive:     processAlive,
		startTime: procStartTime,
		titles:    func() (map[string]string, error) { return map[string]string{}, nil },
		detector:  titleLevelDetector{},
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

// recordAlive reports whether the process that reported this record is still that same live
// process. Liveness is the anchor pid being alive; when a start-time token was recorded it must
// also still match, so a recycled pid (the original process exited and an unrelated one now
// holds the number) does not keep a dead agent's record alive indefinitely. A missing token (an
// older record, or a platform without /proc) or a currently unreadable start time degrades to
// the bare liveness check, so a live agent is never falsely reclaimed on a transient read miss.
func (s *ClaudeSource) recordAlive(r record) bool {
	return recordLiveWith(r, s.alive, s.startTime)
}

// recordLiveWith is the liveness rule of recordAlive with its process probes passed in, so the
// source (test-injected probes) and the hook path (real processAlive/procStartTime) share one
// definition of "still the same live process" and cannot drift.
func recordLiveWith(r record, alive func(int) bool, startTime func(int) string) bool {
	if !alive(r.PID) {
		return false
	}
	if r.PIDStart == "" || startTime == nil {
		return true
	}
	if cur := startTime(r.PID); cur != "" && cur != r.PIDStart {
		return false
	}
	return true
}

// sessionPID returns the pid of the Claude session process that owns this hook
// invocation. Claude Code runs each hook in a short-lived shell, so os.Getppid()
// is that shell — it exits the instant the hook returns, which would make a live
// agent's recorded pid dead within milliseconds. The durable handle is the outermost
// Claude ancestor (see walkToClaude), so we walk up to it. If no Claude ancestor is
// found (an unexpected install), fall back to the immediate parent as a best effort.
func sessionPID() int {
	parent := os.Getppid()
	if pid, ok := walkToClaude(parent, procParent); ok {
		return pid
	}
	return parent
}

// walkToClaude walks the ancestry from start and returns the outermost process in the
// contiguous run of Claude ancestors: the session's entry process, not the inner hosts it
// spawns. Claude Code runs a session as a chain of Claude processes - an interactive client
// (`claude --resume`) or per-session detached host, above a daemon and a rotating
// bg-pty-host/bg-spare pool that actually run the agent and execute hooks. Those inner hosts
// are recycled independently of the session, so anchoring liveness on the nearest one (where
// the hook fires) reclaims a live agent the moment its host rotates. Climbing to the top of
// the contiguous Claude chain anchors on the durable session process instead; a non-Claude
// gap (a Claude launched inside another Claude's shell) ends the chain, so a nested session
// still resolves to its own client. parent reports a pid's parent pid and argv[0]; the hop
// bound and pid<=1 guard keep the walk finite even if the chain is cyclic or rooted at init.
func walkToClaude(start int, parent func(int) (ppid int, argv0 string)) (int, bool) {
	top := -1
	for cur, hops := start, 0; cur > 1 && hops < 64; hops++ {
		ppid, argv0 := parent(cur)
		if isClaudeComm(argv0) {
			top = cur // highest Claude seen so far in the contiguous chain
		} else if top >= 0 {
			break // left the Claude chain: the previous Claude was the outermost
		}
		if ppid <= 0 {
			break
		}
		cur = ppid
	}
	if top < 0 {
		return 0, false
	}
	return top, true
}

// isClaudeComm reports whether a process's invoked executable (its argv[0], possibly a
// full path) is the Claude Code binary.
func isClaudeComm(argv0 string) bool {
	return filepath.Base(strings.TrimSpace(argv0)) == "claude"
}

// procParent reports a pid's parent pid and invoked executable (argv[0]). sessionPID walks
// the ancestry on every hook event, and PreToolUse/PostToolUse fire per tool call, so on Linux
// it reads /proc directly (stat for the parent pid, cmdline for argv[0]) rather than forking ps
// per hop. It falls back to ps on platforms without /proc or on any read failure. It is a
// package var so tests can supply a synthetic process tree. A failed lookup yields (0, "").
var procParent = func(pid int) (int, string) {
	if ppid, _, ok := readProcStat(pid); ok {
		return ppid, procArgv0(pid)
	}
	return psParent(pid)
}

// psParent is the portable ps-backed parent lookup, used on platforms without /proc (macOS) or
// when a /proc read fails. A failed lookup yields (0, "").
func psParent(pid int) (int, string) {
	out, err := exec.Command("ps", "-o", "ppid=,args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, ""
	}
	return parsePsParent(out)
}

// readProcStat parses /proc/<pid>/stat for the parent pid and the process start time. The
// start time (field 22, clock ticks since boot) is fixed for the life of a process and never
// reused, so it distinguishes a still-running process from a different one that recycled its
// pid. ok is false when /proc is unavailable (non-Linux) or the file cannot be read or parsed.
// A package var so tests can supply synthetic /proc contents.
var readProcStat = func(pid int) (ppid int, startTime string, ok bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, "", false
	}
	return parseProcStat(data)
}

// parseProcStat extracts the parent pid and start time from a /proc/<pid>/stat line. The comm
// field (2) is wrapped in parentheses and can itself contain spaces and ')', so the numeric
// fields are taken from after the final ')': fields[0] is state (stat field 3), so ppid (field
// 4) is fields[1] and starttime (field 22) is fields[19].
func parseProcStat(data []byte) (ppid int, startTime string, ok bool) {
	i := bytes.LastIndexByte(data, ')')
	if i < 0 {
		return 0, "", false
	}
	fields := strings.Fields(string(data[i+1:]))
	if len(fields) < 20 {
		return 0, "", false
	}
	p, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, "", false
	}
	return p, fields[19], true
}

// procStartTime returns a process's start-time token (see readProcStat), or "" when it cannot
// be read (non-Linux, or the process is gone). It is compared against the token recorded when
// the agent was last seen: an equal token means the same process still holds the pid, a
// different one means the pid was recycled. A package var so tests can supply synthetic values.
var procStartTime = func(pid int) string {
	_, start, ok := readProcStat(pid)
	if !ok {
		return ""
	}
	return start
}

// procArgv0 reads a process's argv[0] from /proc/<pid>/cmdline (NUL-separated). Unlike comm,
// argv[0] is not renamed by Claude Code's background hosts, so it reliably reads "claude".
// Empty on any failure (non-Linux, unreadable, or a zombie with no cmdline).
var procArgv0 = func(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return ""
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return string(data[:i])
	}
	return string(data)
}

// parsePsParent extracts the parent pid and argv[0] from one `ps -o ppid=,args=` line (the
// macOS/fallback path). It keys off argv[0], not comm: Claude Code renames the comm of its background host processes
// to the version string (e.g. "2.1.204"), so comm no longer reads as "claude" for daemon and
// background sessions, while argv[0] stays the claude executable across every variant
// (interactive "claude --resume", "claude daemon run", "claude bg-spare", "claude bg-pty-host").
// Anchoring liveness on comm would miss those durable processes and fall back to the ephemeral
// hook shell, whose pid dies at once - reclaiming the agent the moment it is recorded.
func parsePsParent(out []byte) (int, string) {
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return 0, ""
	}
	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, ""
	}
	return ppid, fields[1] // argv[0]; isClaudeComm takes its basename
}

// Agents implements Source.
func (s *ClaudeSource) Agents() ([]Agent, error) {
	recs, err := readRecords(s.recDir)
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
		// running. A dead (or unidentifiable) process, or a pid the process recycled,
		// means the session is gone, so delete its state — even if its tmux pane (a
		// leftover shell) still lingers.
		if !s.recordAlive(r) {
			_ = os.Remove(filepath.Join(s.recDir, sanitize(r.SessionID)+".json"))
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
	s.applyMutes(out)
	sort.SliceStable(out, func(i, j int) bool {
		// Muted agents sort below every unmuted one regardless of status, so they settle
		// into the bottom band; mute is the highest-priority ordering term.
		if out[i].Muted != out[j].Muted {
			return !out[i].Muted
		}
		if out[i].Status.rank() != out[j].Status.rank() {
			return out[i].Status.rank() < out[j].Status.rank()
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

// applyMutes stamps the user's persisted mute intent onto the live agents and reconciles
// the mute store against them: a needs-attention agent is force-unmuted (a genuine block
// is never hidden - the snooze "wake me on something new" rule), and a stored key with no
// live agent is pruned (mute is reclaimed by the same liveness GC that drops the agent).
// It writes the store back only when it changed, so a steady state touches no disk.
func (s *ClaudeSource) applyMutes(out []Agent) {
	mutes, err := readMutes(s.dir)
	if err != nil || len(mutes) == 0 {
		// Nothing muted (or unreadable): leave every agent unmuted and write nothing.
		return
	}
	live := make(map[string]bool, len(out))
	dirty := false
	for i := range out {
		key := dedupKey(out[i])
		live[key] = true
		if !mutes[key] {
			continue
		}
		if out[i].Status == StatusNeedsAttention {
			delete(mutes, key) // a new block wins: auto-unmute
			dirty = true
			continue
		}
		out[i].Muted = true
	}
	for key := range mutes {
		if !live[key] {
			delete(mutes, key) // location gone (process exited): reclaim the mute
			dirty = true
		}
	}
	if dirty {
		_ = writeMutes(s.dir, mutes)
	}
}

// SetMuted records or clears the user's mute intent for a tmux location, keyed so it
// follows the location rather than a single session id. It is the write side of the mute
// store the view drives from the `m` toggle; Agents stamps the result back onto rows.
func (s *ClaudeSource) SetMuted(locKey string, muted bool) error {
	mutes, err := readMutes(s.dir)
	if err != nil {
		return err
	}
	if mutes[locKey] == muted {
		return nil
	}
	if muted {
		mutes[locKey] = true
	} else {
		delete(mutes, locKey)
	}
	return writeMutes(s.dir, mutes)
}

// dedupKey identifies the tmux location an agent occupies, so records that share
// a pane (or window, for older records without a pane id) collapse to one row.
// Agents with no tmux location stay distinct, keyed by their own session id.
func dedupKey(a Agent) string {
	return locationKey(a.TmuxPane, a.TmuxSession, a.TmuxWindow, a.SessionID)
}

// locationKey is the shared identity of a tmux location: the pane when known, else the
// window, else the session, else the agent's own id. dedupKey (collapsing records that
// share a location) and Row.muteKey (persisting mute against a location) both derive
// from it so they cannot drift apart.
func locationKey(pane, session, window, id string) string {
	switch {
	case pane != "":
		return "pane:" + pane
	case session != "" && window != "":
		return "win:" + session + ":" + window
	case session != "":
		return "sess:" + session
	default:
		return "id:" + id
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
// working/idle level. needs-attention is a latch: it holds until the level shows the agent
// working (it resumed, so the user answered) - an idle or absent level does not clear it,
// keeping a real pending prompt visible. For every other hook status the live level
// overrides working/idle when present, and we fall back to the hook status when no level
// is available.
func reconcile(hookStatus, level Status, haveLevel bool) Status {
	switch hookStatus {
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
//
// A SessionEnd event produces no terminal status: an ended session keeps its last status
// until its Claude process exits, at which point the liveness GC removes it. The
// Notification split is deliberately downgrade-only and conservative: only a recognized
// idle message becomes idle; everything else stays needs-attention, so an unfamiliar
// notification still reaches the user rather than being hidden.
func StatusFor(event string, in hookInput) Status {
	switch event {
	case "Notification":
		if strings.Contains(strings.ToLower(in.Message), idleNotification) {
			return StatusIdle
		}
		return StatusNeedsAttention
	case "Stop":
		return StatusIdle
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

// HandleHook processes one hook invocation: it reads the hook payload from r, resolves the
// tmux location, writes the updated state record, and emits an out-of-band notification when
// the agent crossed an urgent status edge. Notification policy is resolved from config here so
// the hook is self-contained; the core takes it as an argument so tests can inject a fake.
func HandleHook(event string, r io.Reader) error {
	return handleHook(event, r, loadNotifyPolicy())
}

func handleHook(event string, r io.Reader, policy notifyPolicy) error {
	root, err := StateDir()
	if err != nil {
		return err
	}
	recDir, err := agentsDir()
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

	rec, _ := readRecord(recDir, in.SessionID)
	// Claude Code marks background/daemon sessions with CLAUDE_CODE_SESSION_KIND=bg. Persist it
	// verbatim (the raw fact, interpreted at read time), falling back to any kind already on the
	// record so a payload that omits it does not erase what an earlier event captured.
	kind := os.Getenv("CLAUDE_CODE_SESSION_KIND")
	if kind == "" {
		kind = rec.Kind
	}
	// A background session's anchor process can be shared across sessions (a daemon that outlives
	// any one of them), so pid liveness alone may never reclaim its record. SessionEnd is the
	// authoritative end of a session, so for a bg session remove the record now rather than
	// leaving a row that never clears; the pid GC stays the backstop for a bg crash with no
	// SessionEnd. Interactive sessions keep their record until the process exits (below), so
	// their last status still lingers.
	if event == "SessionEnd" && kind == "bg" {
		_ = os.Remove(filepath.Join(recDir, sanitize(in.SessionID)+".json"))
		return nil
	}
	// Capture the prior status before the event overwrites it, so a notification can fire on
	// the transition (the edge) rather than on merely being in a state.
	old := rec.Status
	rec.SessionID = in.SessionID
	rec.Kind = kind
	// SessionEnd produces no terminal status: the session keeps its last status until its
	// Claude process exits and the liveness GC reclaims it. Every other event maps to one.
	if event != "SessionEnd" {
		rec.Status = StatusFor(event, in)
	}
	rec.Updated = time.Now()
	// Record the Claude session process so `be agents` can treat that process's
	// liveness as the agent's liveness (a closed Claude → its entry is reclaimed).
	// The hook runs in a short-lived shell, so we walk up to the Claude process
	// rather than recording our immediate (ephemeral) parent. The pid's start time
	// pins the identity of that exact process, so a later pid reuse cannot keep this
	// record alive (see recordAlive).
	rec.PID = sessionPID()
	rec.PIDStart = procStartTime(rec.PID)

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
	if err := writeRecord(recDir, rec); err != nil {
		return err
	}
	// Notification is best-effort and downstream of the record: a delivery problem must never
	// fail the hook chain or lose the status write. Mute lives at the state root, not with records.
	maybeNotify(root, recDir, old, rec, policy)
	return nil
}

// notifyPolicy is the resolved notification behavior for one hook invocation: which edges to
// notify and how to deliver.
type notifyPolicy struct {
	needsAttention bool
	finished       bool
	notifier       Notifier
}

// loadNotifyPolicy reads the effective notification config, falling back to the built-in
// defaults (both edges on, auto delivery) when config cannot be loaded, so notifications keep
// working even past a config error.
func loadNotifyPolicy() notifyPolicy {
	cfg, _, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	n := cfg.Agents.Notify
	return notifyPolicy{
		needsAttention: n.NeedsAttention,
		finished:       n.Finished,
		notifier:       notifierFor(n.Command),
	}
}

// edge is an urgent status transition worth notifying about.
type edge int

const (
	edgeNone edge = iota
	// edgeNeedsAttention: the agent just entered needs-attention (a block on the user).
	edgeNeedsAttention
	// edgeFinished: the agent just finished a turn (working -> idle).
	edgeFinished
)

// classifyEdge maps an (old -> new) status transition to the single edge it notifies on, if
// any. Entering needs-attention is defined as a transition *into* the state (not being in it),
// so a re-fired block does not re-notify; finished is scoped tightly to working -> idle so an
// idle nudge arriving while already idle is not read as a fresh finish.
func classifyEdge(old, cur Status) edge {
	switch {
	case cur == StatusNeedsAttention && old != StatusNeedsAttention:
		return edgeNeedsAttention
	case old == StatusWorking && cur == StatusIdle:
		return edgeFinished
	}
	return edgeNone
}

// maybeNotify decides what this agent's status transition pushes out of band. It swallows every
// error: notification is best-effort and must not disturb the hook. The policy is:
//   - Entering needs-attention pushes immediately (the block on the user) - the one always-on
//     escalation, never batched.
//   - A finish (working -> idle) is routine progress and pushes nothing on its own; it accrues
//     into a batch and flushes as a single run-settled digest only when this agent was the last
//     live worker to settle. Everything else is visible on the `be agents` pull surface.
//
// A muted agent is fully suppressed: no escalation, no digest, no batch entry.
func maybeNotify(dir, recDir string, old Status, rec record, policy notifyPolicy) {
	e := classifyEdge(old, rec.Status)
	if e == edgeNone || policy.notifier == nil {
		return
	}
	mutes, _ := readMutes(dir)
	if mutes[locationKey(rec.TmuxPane, rec.TmuxSession, rec.TmuxWindow, rec.SessionID)] {
		return
	}
	switch e {
	case edgeNeedsAttention:
		if !policy.needsAttention {
			return
		}
		_ = policy.notifier.Notify(notificationFor(rec))
	case edgeFinished:
		if !policy.finished {
			return
		}
		live := func(r record) bool { return recordLiveWith(r, processAlive, procStartTime) }
		recs, _ := readRecords(recDir)
		// Not the last to settle: the run is still active. Record the finish for the digest and
		// stay silent - the pull surface already shows the new idle status.
		if anyLiveWorker(recs, rec.SessionID, live) {
			_ = appendNotifyBatch(dir, rec.Title)
			return
		}
		// Last live worker settled: flush one digest for the whole run.
		finished := dedup(append(takeNotifyBatch(dir), rec.Title))
		_ = policy.notifier.Notify(digestNotification(finished, liveBlockedTitles(recs, rec.SessionID, live)))
	}
}

// anyLiveWorker reports whether any record other than exceptID is a live, working agent. Only live
// records count, so a crashed agent whose record is stuck at working does not hold the run open
// and suppress every future digest.
func anyLiveWorker(recs []record, exceptID string, live func(record) bool) bool {
	for _, r := range recs {
		if r.SessionID == exceptID || r.Status != StatusWorking {
			continue
		}
		if live(r) {
			return true
		}
	}
	return false
}

// liveBlockedTitles returns the titles of live agents (other than exceptID) currently in
// needs-attention. A blocked agent is not "working", so it does not hold the run open; when the
// run settles the digest names these first as the anomaly worth acting on.
func liveBlockedTitles(recs []record, exceptID string, live func(record) bool) []string {
	var out []string
	for _, r := range recs {
		if r.SessionID == exceptID || r.Status != StatusNeedsAttention {
			continue
		}
		if live(r) {
			out = append(out, r.Title)
		}
	}
	return dedup(out)
}

// dedup returns titles with duplicates removed, preserving first-seen order (an agent that
// finished twice within one run is named once).
func dedup(titles []string) []string {
	seen := make(map[string]bool, len(titles))
	out := titles[:0:0]
	for _, t := range titles {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// notificationFor builds the immediate needs-attention escalation from the agent's record.
func notificationFor(rec record) Notification {
	return Notification{
		Agent:   rec.Title,
		Status:  rec.Status,
		CWD:     rec.CWD,
		Message: rec.Title + " needs you",
	}
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
	tmuxEnv, target := os.Getenv("TMUX"), os.Getenv("TMUX_PANE")
	if tmuxEnv == "" {
		// Claude Code's daemon runs its sessions detached, with TMUX stripped from the hook
		// env even though the owning claude process still lives in a tmux pane. Recover the
		// server and pane from that process's environment (sessionPID already locates it) so
		// a daemon-run agent maps to its tmux location and stays jumpable, instead of looking
		// location-less. Best-effort: when it too has no tmux, the agent is genuinely headless
		// (surfaced as a detached agent, see Workspace.Rows).
		//
		// Hazard: the recovered TMUX_PANE is only correct while sessionPID is a per-session
		// process (the interactive client), whose environ pins the pane it launched in. If a
		// future Claude version anchored sessions on a process shared across sessions (a daemon)
		// whose environ carried a frozen TMUX_PANE, that one stale pane would be misattributed
		// to every recovered session. This is safe today because observed daemon environs carry
		// no TMUX at all, so recovery yields empties and the agent correctly reads as detached.
		// No per-session pane oracle exists to validate a recovered pane, so rather than guess
		// against unobserved behavior we prefer an honest detached agent to a mislabeled pane -
		// revisit here if a bg session ever recovers a pane it does not actually occupy.
		tmuxEnv, target = procTmux(sessionPID())
		if tmuxEnv == "" {
			return "", "", "", ""
		}
	}
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, "#{session_name}\t#{window_index}\t#{window_name}\t#{pane_id}")
	cmd := exec.Command("tmux", args...)
	// Point tmux at the recovered server when the hook's own env has none, so display-message
	// resolves the pane against the right socket.
	if os.Getenv("TMUX") == "" {
		cmd.Env = append(os.Environ(), "TMUX="+tmuxEnv)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2], parts[3]
}

// procTmux reads the TMUX (server socket) and TMUX_PANE values from a process's environment,
// used to recover the tmux location of a Claude daemon that spawned the current session with
// TMUX stripped. It is a package var so tests can supply a synthetic environment. Reads
// /proc/<pid>/environ (Linux); on platforms without it, or on any failure, it yields empties so
// resolveTmux falls back cleanly - an in-pane session already has TMUX in its own env and never
// needs this.
var procTmux = func(pid int) (tmux, pane string) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return "", ""
	}
	return parseTmuxEnv(data)
}

// parseTmuxEnv extracts TMUX and TMUX_PANE from a NUL-separated environ blob (the
// /proc/<pid>/environ format). Absent keys yield empty strings.
func parseTmuxEnv(environ []byte) (tmux, pane string) {
	for _, kv := range strings.Split(string(environ), "\x00") {
		switch {
		case strings.HasPrefix(kv, "TMUX="):
			tmux = strings.TrimPrefix(kv, "TMUX=")
		case strings.HasPrefix(kv, "TMUX_PANE="):
			pane = strings.TrimPrefix(kv, "TMUX_PANE=")
		}
	}
	return tmux, pane
}
