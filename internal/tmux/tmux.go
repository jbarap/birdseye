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
// is a no-op when the session already exists.
func (c *Client) Ensure(name, dir string) error {
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
