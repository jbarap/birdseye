package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/worktree"
)

func newWorktreeCmd() *cobra.Command {
	var root string

	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage repos as <root>/<repo>/main plus sibling worktrees",
	}
	cmd.PersistentFlags().StringVar(&root, "root", "", "managed repositories root (defaults to config worktree.root or ~/code)")

	resolveRoot := func() (string, error) {
		if root != "" {
			return root, nil
		}
		cfg, err := loadConfig()
		if err != nil {
			return "", err
		}
		if cfg.Worktree.Root != "" {
			return cfg.Worktree.Root, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "code"), nil
	}

	clone := &cobra.Command{
		Use:   "clone <url>",
		Short: "Clone a repository into <root>/<repo>/main",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveRoot()
			if err != nil {
				return err
			}
			mainDir, existed, err := worktree.Clone(r, args[0])
			if err != nil {
				return err
			}
			verb := "Cloned"
			if existed {
				verb = "Already cloned"
			}
			fmt.Printf("%s %s  %s\n", okMark.Render("✓"), verb, hintStyle.Render(mainDir))
			return nil
		},
	}

	add := &cobra.Command{
		Use:   "add <repo> <name> [branch]",
		Short: "Add a sibling worktree <root>/<repo>/<name>",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveRoot()
			if err != nil {
				return err
			}
			branch := ""
			if len(args) == 3 {
				branch = args[2]
			}
			dir, err := worktree.Add(r, args[0], args[1], branch)
			if err != nil {
				return err
			}
			fmt.Printf("%s Created worktree  %s\n", okMark.Render("✓"), hintStyle.Render(dir))
			return nil
		},
	}

	cmd.AddCommand(clone, add)
	return cmd
}
