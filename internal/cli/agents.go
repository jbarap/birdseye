package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/agents"
	"github.com/jbarap/birds-eye/internal/tmux"
	"github.com/jbarap/birds-eye/internal/worktree"
)

func newAgentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "Bird's-eye view of AI agent sessions and their status",
		Long: "Shows tracked agent sessions and their status (needs-attention / working / " +
			"idle / done). Designed for a tmux popup: tmux display-popup -E be agents.",
		RunE: func(cmd *cobra.Command, args []string) error { return runAgents() },
	}
}

func runAgents() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	keys, err := agents.ResolveKeymap(cfg.Agents.Keys)
	if err != nil {
		return err
	}
	accent, err := agents.ResolveAccent(cfg.Agents.Accent)
	if err != nil {
		return err
	}
	refresh, err := cfg.Agents.RefreshInterval()
	if err != nil {
		return err
	}
	src, err := agents.NewClaudeSource()
	if err != nil {
		return err
	}

	// Without tmux the view still works for triage: plain agent rows, no preview, and
	// no orchestration. With tmux it gains the live preview, managed-repo recognition
	// (the Workspace reconciler), and the create/delete actions.
	var rowSrc agents.RowSource = agents.AgentsAsRows(src)
	var prev agents.Previewer = agents.NoopPreviewer{}
	var orch agents.Orchestrator
	client, clientErr := tmux.New()
	if clientErr == nil {
		prev = agents.NewTmuxPreviewer(client, 200)
		rowSrc = agents.NewWorkspace(src, paneLister{client}, repoResolver{})
		orch = orchestrator{client: client, command: cfg.Agents.AgentCommand()}
	}

	chosen, err := agents.Run(rowSrc, prev, keys, accent, refresh, orch)
	if err != nil {
		return err
	}
	if chosen == nil || chosen.TmuxSession == "" {
		return nil
	}
	if client == nil {
		warn("cannot jump to agent: %v", clientErr)
		return nil
	}
	if err := client.ConnectPane(chosen.TmuxSession, chosen.TmuxWindow, chosen.TmuxPane); err != nil {
		return fmt.Errorf("jumping to %s: %w", chosen.TmuxSession, err)
	}
	return nil
}

// paneLister adapts *tmux.Client to agents.PaneLister.
type paneLister struct{ c *tmux.Client }

func (p paneLister) ListPanes() ([]agents.PaneInfo, error) {
	ps, err := p.c.ListPanes()
	if err != nil {
		return nil, err
	}
	out := make([]agents.PaneInfo, len(ps))
	for i, x := range ps {
		out[i] = agents.PaneInfo{
			Session:     x.Session,
			WindowIndex: x.WindowIndex,
			WindowName:  x.WindowName,
			PaneID:      x.PaneID,
			StartPath:   x.StartPath,
		}
	}
	return out, nil
}

// repoResolver adapts worktree.Resolve to agents.RepoResolver.
type repoResolver struct{}

func (repoResolver) Resolve(dir string) (agents.RepoInfo, bool) {
	info, ok := worktree.Resolve(dir)
	if !ok {
		return agents.RepoInfo{}, false
	}
	return agents.RepoInfo{
		Container:     info.Container,
		Repo:          info.Repo,
		DefaultBranch: info.DefaultBranch,
		TopLevel:      info.TopLevel,
		Worktree:      info.Worktree,
	}, true
}

// orchestrator implements agents.Orchestrator with tmux + worktree primitives.
type orchestrator struct {
	client  *tmux.Client
	command string
}

// Spawn adds a worktree in the row's repo, opens a window rooted there, and starts the
// configured agent command.
func (o orchestrator) Spawn(repo agents.Row, name string) error {
	dir, err := worktree.Add(repo.Dir, name, "")
	if err != nil {
		return err
	}
	_, err = o.client.NewWindow(repo.TmuxSession, dir, o.command)
	return err
}

// Remove kills the row's window and, when the row is a managed worktree, removes the
// git worktree. A dirty worktree without force is reported via ErrWorktreeDirty so the
// view can confirm; the dirty check runs before anything is killed so a cancel is
// non-destructive.
func (o orchestrator) Remove(r agents.Row, force bool) error {
	isWorktree := r.Worktree != "" && r.Kind != agents.RowAnchor && r.Dir != ""
	if isWorktree && !force {
		if dirty, err := worktree.IsDirty(r.Dir); err == nil && dirty {
			return agents.ErrWorktreeDirty
		}
	}
	if r.TmuxWindow != "" {
		_ = o.client.KillWindow(r.TmuxSession + ":" + r.TmuxWindow) // best-effort; may be gone
	}
	if isWorktree {
		return worktree.Remove(r.Dir, force)
	}
	return nil
}
