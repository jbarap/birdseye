package cli

import (
	"testing"

	"github.com/jbarap/birdseye/internal/agents"
	"github.com/jbarap/birdseye/internal/config"
	"github.com/jbarap/birdseye/internal/fleet"
	"github.com/jbarap/birdseye/internal/tmux"
)

// TestRemoveTargetsWindowByPaneID guards a recurring fragility: a tmux window index
// drifts as windows are renumbered when others close, so a recorded index can point at
// the wrong window or none. Deleting an agent must target the window by its stable pane
// id instead. Regression for a zombie window that `d` could not remove. The dash's
// Remove delegates to the shared fleet, so this exercises that path end to end.
func TestRemoveTargetsWindowByPaneID(t *testing.T) {
	var calls [][]string
	run := func(args ...string) (string, error) {
		calls = append(calls, args)
		return "", nil
	}
	client := tmux.NewWithRunner(run, true)
	o := orchestrator{fleet: fleet.New(client, config.Config{}, nil, nil, nil), client: client}

	// An incidental agent row (no worktree, so no git removal): the recorded window
	// index "4" is stale, but the pane id "%154" still resolves to the live window.
	r := agents.Row{
		Kind:        agents.RowAgent,
		TmuxSession: "superset-main",
		TmuxWindow:  "4",
		TmuxPane:    "%154",
	}
	if err := o.Remove(r, false); err != nil {
		t.Fatal(err)
	}

	var killTarget string
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "kill-window" && c[1] == "-t" {
			killTarget = c[2]
		}
	}
	if killTarget != "%154" {
		t.Fatalf("kill-window target = %q, want the stable pane id %%154 (not the stale index)", killTarget)
	}
}
