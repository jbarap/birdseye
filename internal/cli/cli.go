// Package cli wires the bird's-eye commands together: it builds the provider
// registry from config and tool availability, and routes the picker, agents,
// worktree, and hook commands.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/config"
	"github.com/jbarap/birds-eye/internal/picker"
	"github.com/jbarap/birds-eye/internal/provider"
	"github.com/jbarap/birds-eye/internal/providers/dir"
	"github.com/jbarap/birds-eye/internal/providers/tmuxp"
	"github.com/jbarap/birds-eye/internal/providers/tmuxsess"
	"github.com/jbarap/birds-eye/internal/tmux"
	"github.com/jbarap/birds-eye/internal/tools"
	"github.com/jbarap/birds-eye/internal/worktree"
)

// Execute builds and runs the root command. version is shown by `be --version`.
func Execute(version string) error {
	root := &cobra.Command{
		Use:           "be",
		Short:         "bird's-eye — a fuzzy view over tmux sessions and the agents inside them",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, args []string) error { return runList() },
	}
	root.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "Fuzzy-pick a session to attach to or create",
			RunE:  func(cmd *cobra.Command, args []string) error { return runList() },
		},
		newAgentsCmd(),
		newWorktreeCmd(),
		newHookCmd(),
	)
	return root.Execute()
}

func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "be: "+format+"\n", a...)
}

// loadConfig loads config or exits-style errors to the caller.
func loadConfig() (config.Config, error) {
	cfg, _, err := config.Load()
	return cfg, err
}

// buildRegistry registers the built-in providers based on config and available
// tools.
func buildRegistry(cfg config.Config, set tools.Set, lister tmuxsess.Lister) *provider.Registry {
	r := provider.NewRegistry(func(m string) { warn("%s", m) })
	if lister != nil {
		r.Register(tmuxsess.New(lister))
	}
	r.Register(tmuxp.New(cfg.Tmuxp.Dir, set.Tmuxp, lister))
	r.Register(dir.New(cfg.Dir.UseZoxide && set.Zoxide, cfg.Dir.Roots))
	r.Register(worktree.NewProvider(cfg.Worktree.Root))
	r.SetEnabled(cfg.Providers)
	return r
}

// runList is the default command: aggregate candidates, fuzzy-pick, dispatch.
func runList() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client, err := tmux.New()
	if err != nil {
		return err // tmux is required for the picker flow
	}

	reg := buildRegistry(cfg, tools.Probe(), client)
	cands := reg.Candidates()

	sel, err := picker.Pick(cands, cfg)
	switch err {
	case nil:
		// proceed
	case picker.ErrNoCandidates:
		fmt.Println("No sessions to show yet. Add tmuxp templates, zoxide dirs, or configure providers.")
		return nil
	case picker.ErrCancelled:
		return nil
	default:
		return err
	}
	if sel == nil {
		return nil
	}
	return sel.Action(client)
}
