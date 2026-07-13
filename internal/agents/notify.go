package agents

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Notification is one out-of-band alert about an agent crossing an urgent status edge.
// It is delivered by a Notifier; the fields are also exported to a user command template as
// environment variables (see commandNotifier).
type Notification struct {
	// Agent is a short human label for the agent (the record's Title, e.g. "sess:dir").
	Agent string
	// Status is the status the agent just entered (needs-attention or idle).
	Status Status
	// Repo is the agent's repository when resolvable at hook time, else empty.
	Repo string
	// CWD is the agent's working directory.
	CWD string
	// Message is the ready-to-show line, e.g. "birdseye needs you".
	Message string
}

// Notifier delivers a Notification. Implementations are best-effort: the caller discards any
// error so a delivery problem never disturbs the agent's hook chain.
type Notifier interface {
	Notify(n Notification) error
}

// digestTitleCap bounds how many finished-agent titles the run-settled digest names before
// collapsing the remainder to "(+N more)", so a large run yields a readable one-liner.
const digestTitleCap = 3

// digestNotification builds the single run-settled digest emitted when the last live worker
// settles. finished is the agents that hit working -> idle during the run; blocked is any agent
// still in needs-attention. Blocked agents are the anomaly and are named first.
func digestNotification(finished, blocked []string) Notification {
	return Notification{
		Agent:   fmt.Sprintf("%d finished", len(finished)),
		Status:  StatusIdle,
		Message: finishDigest(finished, blocked),
	}
}

// finishDigest renders the digest line: blocked agents first ("web-api needs you"), then the
// finished titles up to digestTitleCap with the rest collapsed ("finished: worker, docs (+2 more)").
func finishDigest(finished, blocked []string) string {
	var parts []string
	if len(blocked) > 0 {
		verb := "needs you"
		if len(blocked) > 1 {
			verb = "need you"
		}
		parts = append(parts, strings.Join(blocked, ", ")+" "+verb)
	}
	if len(finished) > 0 {
		parts = append(parts, "finished: "+cappedList(finished, digestTitleCap))
	}
	if len(parts) == 0 {
		return "run settled"
	}
	return strings.Join(parts, "; ")
}

// cappedList joins the first n items with commas and collapses any remainder to "(+N more)".
func cappedList(items []string, n int) string {
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(items[:n], ", "), len(items)-n)
}

// notifierFor returns the notifier selected by command: the user's command template when
// command is non-empty, otherwise the auto-detected platform notifier.
func notifierFor(command string) Notifier {
	if command != "" {
		return commandNotifier{command: command}
	}
	return autoNotifier{}
}

const notifyTitle = "birdseye"

// autoNotifier delivers through the platform's native notifier - notify-send on Linux,
// osascript on macOS - and falls back to a terminal bell, then a no-op, when none is present.
// A box without any notifier is not an error: like birdseye's other optional tools, delivery
// degrades gracefully rather than failing the hook.
type autoNotifier struct{}

func (autoNotifier) Notify(n Notification) error {
	switch runtime.GOOS {
	case "darwin":
		if path, err := exec.LookPath("osascript"); err == nil {
			script := fmt.Sprintf("display notification %q with title %q", n.Message, notifyTitle)
			if exec.Command(path, "-e", script).Run() == nil {
				return nil
			}
		}
	default:
		if path, err := exec.LookPath("notify-send"); err == nil {
			cmd := exec.Command(path, notifyTitle, n.Message)
			cmd.Env = notifyEnv()
			if cmd.Run() == nil {
				return nil
			}
		}
	}
	// The platform notifier was absent, or present but failed to deliver (a stripped hook env
	// with no reachable session bus is the common cause). Ring the bell rather than dropping the
	// alert silently.
	return bellFallback()
}

// notifyEnv is the environment for notify-send, restoring DBUS_SESSION_BUS_ADDRESS when the hook's
// environment lacks it. Claude Code runs hooks detached with a minimal env, so the session-bus
// address notify-send needs to reach the desktop's notification daemon is often absent - and
// without it notify-send fails outright ("Cannot autolaunch D-Bus without X11 $DISPLAY"). The
// systemd user bus lives at a well-known per-uid socket; point notify-send there when it exists so
// delivery survives the stripped env. A box with neither the env var nor that socket is left as-is
// and falls back to the bell.
func notifyEnv() []string {
	env := os.Environ()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		return env
	}
	bus := fmt.Sprintf("/run/user/%d/bus", os.Getuid())
	if _, err := os.Stat(bus); err != nil {
		return env
	}
	// Drop any empty-valued placeholder first: glibc getenv reads the first occurrence, so a
	// leading "DBUS_SESSION_BUS_ADDRESS=" would otherwise shadow the value we append.
	const key = "DBUS_SESSION_BUS_ADDRESS="
	out := env[:0:0]
	for _, e := range env {
		if !strings.HasPrefix(e, key) {
			out = append(out, e)
		}
	}
	return append(out, key+"unix:path="+bus)
}

// bellFallback rings the controlling terminal's bell by writing to /dev/tty directly, never to
// the hook process's stdout/stderr (which Claude Code may read as hook output). A box with no
// tty simply no-ops.
func bellFallback() error {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return nil
	}
	defer tty.Close()
	_, _ = tty.WriteString("\a")
	return nil
}

// commandNotifier runs a user-configured command, passing the notification as environment
// variables so the command can route it anywhere (ntfy, Slack, Pushover) without birdseye
// shipping platform-specific integrations. The command is run through the shell so a value like
// `notify-send "$BE_AGENT" "$BE_MESSAGE"` works as written.
type commandNotifier struct {
	command string
}

func (c commandNotifier) Notify(n Notification) error {
	cmd := exec.Command("sh", "-c", c.command)
	cmd.Env = append(os.Environ(),
		"BE_AGENT="+n.Agent,
		"BE_STATUS="+string(n.Status),
		"BE_REPO="+n.Repo,
		"BE_CWD="+n.CWD,
		"BE_MESSAGE="+n.Message,
	)
	return cmd.Run()
}
