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
	src, err := agents.NewClaudeSource(cfg.Agents.StaleAfter.AsDuration())
	if err != nil {
		return err
	}
	chosen, err := agents.Run(src)
	if err != nil {
		return err
	}
	if chosen == nil || chosen.TmuxSession == "" {
		return nil
	}
	client, err := tmux.New()
	if err != nil {
		warn("cannot jump to agent: %v", err)
		return nil
	}
	if err := client.Connect(chosen.TmuxSession); err != nil {
		return fmt.Errorf("jumping to %s: %w", chosen.TmuxSession, err)
	}
	return nil
}
