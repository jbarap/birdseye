package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jbarap/birdseye/internal/config"
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

	var noSetup bool
	add := &cobra.Command{
		Use:   "add <name> [branch]",
		Short: "Add a grouped-sibling worktree from inside a managed repo",
		Long: "Creates a worktree at <repo>.worktrees/<branch-slug> beside the clone. With no " +
			"branch argument a new branch named after <name> is created off the default branch; " +
			"a name matching an existing branch checks it out; an explicit branch is used as given.\n\n" +
			"On creation the worktree is provisioned: files listed in the repo's .worktreeinclude " +
			"are carried over, then the setup command from .birdseye/config.toml (or the global " +
			"default) runs in the new worktree. Use --no-setup to skip the setup command.",
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
			dir, warnings, err := worktree.Add(cwd, args[0], branch)
			if err != nil {
				return err
			}
			fmt.Printf("%s Created worktree  %s\n", okMark.Render("✓"), hintStyle.Render(dir))
			for _, w := range warnings {
				fmt.Fprintln(os.Stderr, hintStyle.Render("⚠ "+w))
			}
			if noSetup {
				return nil
			}
			return runSetup(dir, branch, args[0])
		},
	}
	add.Flags().BoolVar(&noSetup, "no-setup", false, "skip the worktree setup command (still carries .worktreeinclude files)")

	cmd.AddCommand(add)
	return cmd
}

// runSetup resolves and runs the worktree setup command inline for the agentless
// `be worktree add` path, streaming its output to the terminal. branch may be empty, in which
// case the worktree name stands in for the branch in the env contract. A non-zero exit fails
// the command (leaving the created worktree on disk so the user can fix and retry); when no
// setup command is configured this is a no-op.
func runSetup(worktreeDir, branch, name string) error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	info, ok := worktree.Resolve(worktreeDir)
	if !ok {
		return fmt.Errorf("could not resolve repository for %s", worktreeDir)
	}
	primary := filepath.Dir(info.GitDir)
	eff, err := config.Overlay(cfg, primary)
	if err != nil {
		return err
	}
	setup := eff.Worktree.Setup
	if setup == "" {
		return nil
	}
	if branch == "" {
		branch = name
	}
	c := exec.Command("sh", "-c", setup)
	c.Dir = worktreeDir
	c.Env = append(os.Environ(), worktree.SetupEnv(worktreeDir, primary, branch)...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("setup command failed: %w", err)
	}
	return nil
}
