package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/agents"
	"github.com/jbarap/birds-eye/internal/tmux"
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
	src, err := agents.NewClaudeSource(
		cfg.Agents.StaleAfter.AsDuration(),
		cfg.Agents.ForgetDone.AsDuration(),
		cfg.Agents.ForgetStale.AsDuration(),
	)
	if err != nil {
		return err
	}

	// A tmux client (when available) backs both the live preview and the
	// jump-to-session. When tmux is absent the preview is disabled and the view
	// still works for triage.
	var prev agents.Previewer = agents.NoopPreviewer{}
	client, clientErr := tmux.New()
	if clientErr == nil {
		prev = agents.NewTmuxPreviewer(client, 0)
	}

	chosen, err := agents.Run(src, prev, keys)
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
