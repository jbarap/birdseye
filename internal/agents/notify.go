package agents

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
			return exec.Command(path, "-e", script).Run()
		}
	default:
		if path, err := exec.LookPath("notify-send"); err == nil {
			return exec.Command(path, notifyTitle, n.Message).Run()
		}
	}
	return bellFallback()
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
