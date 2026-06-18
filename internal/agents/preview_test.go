package agents

import "testing"

type fakeCapturer struct{ target string }

func (f *fakeCapturer) CapturePane(target string, lines int) (string, error) {
	f.target = target
	return "captured", nil
}

func TestPreviewTargetsExactPane(t *testing.T) {
	cap := &fakeCapturer{}
	p := NewTmuxPreviewer(cap, 0)

	// Pane id wins over session:window so a split window captures the right pane.
	if _, err := p.Preview(Agent{TmuxSession: "s", TmuxWindow: "2", TmuxPane: "%7"}); err != nil {
		t.Fatal(err)
	}
	if cap.target != "%7" {
		t.Fatalf("expected pane id target %q, got %q", "%7", cap.target)
	}

	// Falls back to session:window when no pane recorded (older records).
	if _, err := p.Preview(Agent{TmuxSession: "s", TmuxWindow: "2"}); err != nil {
		t.Fatal(err)
	}
	if cap.target != "s:2" {
		t.Fatalf("expected session:window fallback, got %q", cap.target)
	}
}

func TestPreviewTargetEmptyWithoutLocation(t *testing.T) {
	if got := previewTarget(Agent{}); got != "" {
		t.Fatalf("expected empty target without tmux location, got %q", got)
	}
}
