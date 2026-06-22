package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/worktree"
)

func newWorktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage repos as <repo>/<default-branch> plus sibling worktrees, anywhere on disk",
	}

	clone := &cobra.Command{
		Use:   "clone <url> [parent]",
		Short: "Clone a repository into <parent>/<repo>/<default-branch> (parent defaults to the current directory)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			parent := ""
			if len(args) == 2 {
				parent = args[1]
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				parent = cwd
			}
			dir, existed, err := worktree.Clone(parent, args[0])
			if err != nil {
				return err
			}
			verb := "Cloned"
			if existed {
				verb = "Already cloned"
			}
			fmt.Printf("%s %s  %s\n", okMark.Render("✓"), verb, hintStyle.Render(dir))
			return nil
		},
	}

	add := &cobra.Command{
		Use:   "add <name> [branch]",
		Short: "Add a sibling worktree from inside a managed repo",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			branch := ""
			if len(args) == 2 {
				branch = args[1]
			}
			dir, err := worktree.Add(cwd, args[0], branch)
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
