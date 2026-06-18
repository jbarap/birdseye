package agents

// Previewer yields a textual preview of what an agent's session is doing. It is
// a display-only convenience: the agents view never derives status from it.
type Previewer interface {
	// Preview returns recent terminal output for the agent's session, or an
	// error/empty string when no preview is available.
	Preview(a Agent) (string, error)
}

// PaneCapturer captures a tmux pane's recent output. *tmux.Client satisfies it
// without the agents package importing tmux.
type PaneCapturer interface {
	CapturePane(target string, lines int) (string, error)
}

// tmuxPreviewer previews an agent by capturing its tmux pane.
type tmuxPreviewer struct {
	cap   PaneCapturer
	lines int
}

// NewTmuxPreviewer returns a Previewer that captures up to lines of scrollback
// (0 = just the visible screen) from the agent's tmux session.
func NewTmuxPreviewer(cap PaneCapturer, lines int) Previewer {
	return &tmuxPreviewer{cap: cap, lines: lines}
}

func (p *tmuxPreviewer) Preview(a Agent) (string, error) {
	target := previewTarget(a)
	if target == "" {
		return "", nil
	}
	return p.cap.CapturePane(target, p.lines)
}

// previewTarget picks the most precise capture target: the exact pane id when
// known (correct even in a split window), else session:window, else the session.
func previewTarget(a Agent) string {
	if a.TmuxPane != "" {
		return a.TmuxPane
	}
	if a.TmuxSession == "" {
		return ""
	}
	if a.TmuxWindow != "" {
		return a.TmuxSession + ":" + a.TmuxWindow
	}
	return a.TmuxSession
}

// NoopPreviewer is used when no preview backend is available (e.g. tmux absent).
// The view hides the preview pane entirely for it rather than showing an empty
// box.
type NoopPreviewer struct{}

func (NoopPreviewer) Preview(Agent) (string, error) { return "", nil }
