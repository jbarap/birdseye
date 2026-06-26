// Package tmux is the backend that materializes selections into live tmux
// sessions. It wraps the tmux CLI behind a small, injectable runner so the rest
// of the program never shells out to tmux directly and the logic stays testable.
package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNotInstalled is returned when the tmux binary is not on PATH.
var ErrNotInstalled = errors.New("tmux is required but was not found on PATH")

// Runner executes a tmux invocation and returns its stdout. On failure the
// returned error includes tmux's stderr for diagnosis.
type Runner func(args ...string) (string, error)

// Attacher runs a foreground tmux invocation that must own the terminal
// (attach-session). Unlike Runner it does not capture output: tmux takes over
// the process's real stdin/stdout/stderr, which it requires to attach.
type Attacher func(args ...string) error

// Client drives tmux. Construct it with New (production) or NewWithRunner
// (tests).
type Client struct {
	run    Runner
	attach Attacher
	inTmux bool
}

// New returns a Client backed by the real tmux binary, or ErrNotInstalled if
// tmux is not available.
func New() (*Client, error) {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, ErrNotInstalled
	}
	return &Client{run: execRunner, attach: execAttach, inTmux: os.Getenv("TMUX") != ""}, nil
}

// NewWithRunner returns a Client driven by a custom runner. inTmux selects
// attach vs switch behavior. The attach path is routed through the same runner
// so tests can observe it. Intended for tests.
func NewWithRunner(run Runner, inTmux bool) *Client {
	return &Client{
		run:    run,
		attach: func(args ...string) error { _, err := run(args...); return err },
		inTmux: inTmux,
	}
}

func execRunner(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tmux %s: %s", strings.Join(args, " "), msg)
	}
	return out.String(), nil
}

// execAttach runs tmux in the foreground with the current terminal inherited so
// it can attach. Capturing stdout/stdin (as execRunner does) makes tmux report
// "open terminal failed: not a terminal", so attach must keep the real TTY.
func execAttach(args ...string) error {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return ErrNotInstalled
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// HasSession reports whether a session with the given name exists.
func (c *Client) HasSession(name string) (bool, error) {
	_, err := c.run("has-session", "-t", "="+name)
	if err == nil {
		return true, nil
	}
	// tmux exits non-zero both for "no such session" and for "no server
	// running". Treat either as absence; surface nothing as fatal here.
	return false, nil
}

// Ensure creates a detached session named name rooted at dir when absent, and
// is a no-op when the session already exists. window names the session's initial
// window (window 0) — the one holding the session root, e.g. a repo base — so it
// reads as the repo rather than the running shell; empty leaves tmux's default.
// Naming it explicitly also disables tmux automatic-rename for that window, so the
// name sticks.
func (c *Client) Ensure(name, dir, window string) error {
	exists, err := c.HasSession(name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	args := []string{"new-session", "-d", "-s", name}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if window != "" {
		args = append(args, "-n", window)
	}
	_, err = c.run(args...)
	return err
}

// Connect attaches to the named session when invoked outside tmux, or switches
// the current client to it when invoked inside tmux (avoiding nested clients).
func (c *Client) Connect(name string) error {
	if c.inTmux {
		_, err := c.run("switch-client", "-t", name)
		return err
	}
	// Outside tmux we must hand the terminal to tmux; a captured run would fail
	// with "open terminal failed: not a terminal".
	return c.attach("attach-session", "-t", name)
}

// CapturePane returns the recent visible content of the target pane as plain
// text. target is a tmux target such as "session" or "session:window"; the
// active pane of that target is captured. When lines > 0, that many lines of
// scrollback above the visible screen are included. A capture failure (no such
// session, no server) is returned as an error for the caller to handle.
func (c *Client) CapturePane(target string, lines int) (string, error) {
	args := []string{"capture-pane", "-p", "-t", target}
	if lines > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", lines))
	}
	return c.run(args...)
}

// ConnectPane attaches/switches to session and focuses the given window and
// pane so the user lands exactly where the agent runs. window and pane are
// optional; focusing them is best-effort (a since-closed pane simply falls back
// to the session's current selection). pane is a tmux pane id such as "%5".
func (c *Client) ConnectPane(session, window, pane string) error {
	// Focus the target before attach/switch so the client lands there.
	if pane != "" {
		_, _ = c.run("select-window", "-t", pane)
		_, _ = c.run("select-pane", "-t", pane)
	} else if window != "" {
		_, _ = c.run("select-window", "-t", session+":"+window)
	}
	return c.Connect(session)
}

// Pane is one tmux pane located within the session/window structure, carrying the
// directory it was started in. It is the per-tick input the agents view reconciles
// into its workspace snapshot.
type Pane struct {
	Session     string
	WindowIndex string
	WindowName  string
	PaneID      string
	StartPath   string
}

// ListPanes enumerates every pane across all sessions with its start path. When no
// server is running it returns an empty slice without error.
func (c *Client) ListPanes() ([]Pane, error) {
	const format = "#{session_name}\t#{window_index}\t#{window_name}\t#{pane_id}\t#{pane_start_path}"
	out, err := c.run("list-panes", "-a", "-F", format)
	if err != nil {
		// No server / no panes is not an error for enumeration purposes.
		return nil, nil
	}
	var panes []Pane
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 5)
		for len(f) < 5 {
			f = append(f, "")
		}
		panes = append(panes, Pane{
			Session:     f[0],
			WindowIndex: f[1],
			WindowName:  f[2],
			PaneID:      f[3],
			StartPath:   f[4],
		})
	}
	return panes, nil
}

// PaneTitles returns every pane's current OSC title keyed by pane id, via a single
// list-panes query. The title is the out-of-band channel an agent uses to broadcast
// state (e.g. Claude's spinner glyph while working, a sparkle when idle); reading it
// lets the status layer correct a stale hook-written working/idle level. A missing
// server, or any query failure, yields an empty map (not an error), so a refresh
// degrades to hook-only status rather than failing.
func (c *Client) PaneTitles() (map[string]string, error) {
	const format = "#{pane_id}\t#{pane_title}"
	out, err := c.run("list-panes", "-a", "-F", format)
	if err != nil {
		return map[string]string{}, nil
	}
	titles := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		id, title, _ := strings.Cut(line, "\t")
		if id == "" {
			continue
		}
		titles[id] = title
	}
	return titles, nil
}

// NewWindow creates a detached window in session rooted at dir and returns its window
// id (e.g. "@7"). name sets the window name (e.g. the worktree it holds) so it reads as
// the work rather than the running process; empty leaves tmux's default. Naming it also
// disables tmux automatic-rename for that window, so the name sticks. When command is
// non-empty it is typed into the window's shell via send-keys, so the agent runs *inside
// the user's interactive shell* — loading their normal per-directory environment (direnv,
// shell profile, PATH) exactly as opening a pane there would. Handing the command to tmux
// as the window's bare child process instead would bypass all of that, so an env-derived
// setting (e.g. a direnv-selected CLAUDE_CONFIG_DIR) would be wrong. command may be empty
// to leave a plain shell.
func (c *Client) NewWindow(session, dir, name, command string) (string, error) {
	args := []string{"new-window", "-d", "-P", "-F", "#{window_id}", "-t", session + ":"}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if name != "" {
		args = append(args, "-n", name)
	}
	out, err := c.run(args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if command != "" {
		if _, err := c.run("send-keys", "-t", id, command, "Enter"); err != nil {
			return id, err
		}
	}
	return id, nil
}

// KillWindow removes the window identified by target. target may be a window id
// ("@7"), a session:window target, or a pane id ("%154") — tmux resolves a pane id to
// its containing window, which is the stable way to reach a window whose index may
// have drifted.
func (c *Client) KillWindow(target string) error {
	_, err := c.run("kill-window", "-t", target)
	return err
}

// SendKeys dispatches text to the target (a pane id, window, or session:window) and
// presses Enter, the same mechanism NewWindow uses to start a command. It is
// best-effort: tmux send-keys has no readiness signal, so a client that needs delivery
// confirmation must poll the agent's status rather than trust this returning nil.
func (c *Client) SendKeys(target, text string) error {
	_, err := c.run("send-keys", "-t", target, text, "Enter")
	return err
}

// SessionIDs returns a map of session name to tmux session id (e.g. "work" -> "$3").
// The id is tmux's own stable-within-a-server handle; clients use it to drop to raw
// tmux. When no server is running it returns an empty map without error.
func (c *Client) SessionIDs() (map[string]string, error) {
	out, err := c.run("list-sessions", "-F", "#{session_name}\t#{session_id}")
	if err != nil {
		return map[string]string{}, nil
	}
	ids := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 2)
		if len(f) == 2 {
			ids[f[0]] = f[1]
		}
	}
	return ids, nil
}

// KillSession removes the named session.
func (c *Client) KillSession(name string) error {
	_, err := c.run("kill-session", "-t", "="+name)
	return err
}

// ListSessions returns the names of all running sessions. When no server is
// running it returns an empty slice without error.
func (c *Client) ListSessions() ([]string, error) {
	out, err := c.run("list-sessions", "-F", "#{session_name}")
	if err != nil {
		// No server / no sessions is not an error for listing purposes.
		return nil, nil
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}
