package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jbarap/birdseye/internal/worktree"
)

func newWorktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage a repo's grouped-sibling worktrees, anywhere on disk",
		Long: "Manage git worktrees under the grouped-sibling layout: a repository is a plain " +
			"clone at <path>/<repo>, and its worktrees live as grouped siblings under " +
			"<path>/<repo>.worktrees/<branch-slug>. Clone a repo with `git clone` directly; " +
			"there is no `be worktree clone`.",
	}

	add := &cobra.Command{
		Use:   "add <name> [branch]",
		Short: "Add a grouped-sibling worktree from inside a managed repo",
		Long: "Creates a worktree at <repo>.worktrees/<branch-slug> beside the clone. With no " +
			"branch argument a new branch named after <name> is created off the default branch; " +
			"a name matching an existing branch checks it out; an explicit branch is used as given.",
		Args: cobra.RangeArgs(1, 2),
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

	cmd.AddCommand(add)
	return cmd
}
