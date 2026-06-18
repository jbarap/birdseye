package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/agents"
)

// newHookCmd is the machine-facing recorder invoked by Claude Code hooks.
func newHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook <event>",
		Short: "Record agent status from a Claude Code hook (reads hook JSON on stdin)",
		Long: "Intended to be invoked from Claude Code hooks. It reads the hook payload on\n" +
			"stdin and records the session's status so `be agents` can display it.\n" +
			"Known events: SessionStart, UserPromptSubmit, PreToolUse, PostToolUse,\n" +
			"Notification, Stop, SubagentStop, SessionEnd.\n\n" +
			"To install these hooks into your Claude config, use `be hooks install`.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Never fail the agent's hook chain: best-effort record, swallow
			// errors so a hook misconfiguration cannot block Claude Code.
			_ = agents.HandleHook(args[0], os.Stdin)
			return nil
		},
	}
}

// newHooksCmd is the user-facing manager that safely installs/uninstalls the
// hooks in the Claude Code settings file.
func newHooksCmd() *cobra.Command {
	var settingsPath string

	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Install or remove the Claude Code hooks that feed `be agents`",
	}
	cmd.PersistentFlags().StringVar(&settingsPath, "settings", "",
		"path to Claude settings.json (defaults to ~/.claude/settings.json)")

	resolvePath := func() (string, error) {
		if settingsPath != "" {
			return settingsPath, nil
		}
		return agents.DefaultSettingsPath()
	}

	var command string
	install := &cobra.Command{
		Use:   "install",
		Short: "Add bird's-eye hooks to your Claude settings, preserving existing config",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvePath()
			if err != nil {
				return err
			}
			invocation := command
			if invocation == "" {
				invocation = resolveSelfCommand()
			}
			changed, err := agents.InstallHooks(path, invocation)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("Installed bird's-eye hooks into %s\n", path)
				fmt.Printf("(a backup of the previous file is at %s.bak)\n", path)
			} else {
				fmt.Printf("bird's-eye hooks already up to date in %s\n", path)
			}
			fmt.Printf("Events: %v\n", agents.ManagedEvents)
			return nil
		},
	}
	install.Flags().StringVar(&command, "command", "",
		"hook command to install (defaults to the absolute path of this be binary)")

	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove only bird's-eye hooks from your Claude settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvePath()
			if err != nil {
				return err
			}
			changed, err := agents.UninstallHooks(path)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("Removed bird's-eye hooks from %s\n", path)
			} else {
				fmt.Printf("No bird's-eye hooks found in %s\n", path)
			}
			return nil
		},
	}

	cmd.AddCommand(install, uninstall)
	return cmd
}

// resolveSelfCommand returns the absolute path of the running be binary so the
// installed hook calls this exact binary regardless of PATH. It falls back to
// the bare name "be" if resolution fails.
func resolveSelfCommand() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return "be"
	}
	return exe
}
