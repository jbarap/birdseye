package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/agents"
)

// newHookCmd is the parent for agent status hooks. Each agent type is a
// subcommand (currently `claude`), leaving room to add others later.
func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage and feed the agent status hooks behind `be agents`",
	}
	cmd.AddCommand(newClaudeHookCmd())
	return cmd
}

// newClaudeHookCmd groups the Claude Code hook commands: install, uninstall,
// and the machine-facing record verb invoked by the installed hooks.
func newClaudeHookCmd() *cobra.Command {
	claude := &cobra.Command{
		Use:   "claude",
		Short: "Claude Code hooks that report session status to `be agents`",
	}

	var (
		settingsPath string
		command      string
	)

	resolvePath := func() (string, error) {
		if settingsPath != "" {
			return settingsPath, nil
		}
		return agents.DefaultSettingsPath()
	}

	install := &cobra.Command{
		Use:        "install",
		Short:      "Add the Claude Code hooks to your settings, preserving existing config",
		Hidden:     true,
		Deprecated: "use `be agents install claude --hooks`",
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
	install.Flags().StringVar(&settingsPath, "settings", "",
		"path to Claude settings.json (defaults to ~/.claude/settings.json)")
	install.Flags().StringVar(&command, "command", "",
		"hook command to install (defaults to the absolute path of this be binary)")

	uninstall := &cobra.Command{
		Use:        "uninstall",
		Short:      "Remove only bird's-eye's hooks from your Claude settings",
		Hidden:     true,
		Deprecated: "use `be agents uninstall claude --hooks`",
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
	uninstall.Flags().StringVar(&settingsPath, "settings", "",
		"path to Claude settings.json (defaults to ~/.claude/settings.json)")

	record := &cobra.Command{
		Use:   "record <event>",
		Short: "Record status from a Claude Code hook (reads hook JSON on stdin)",
		Long: "Invoked by the installed Claude Code hooks. It reads the hook payload on\n" +
			"stdin and records the session's status so `be agents` can display it.\n" +
			"Known events: SessionStart, UserPromptSubmit, PreToolUse, PostToolUse,\n" +
			"Notification, Stop, SubagentStop, SessionEnd.\n\n" +
			"You normally do not run this by hand; use `be hook claude install`.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Never fail the agent's hook chain: best-effort record, swallow
			// errors so a hook misconfiguration cannot block Claude Code.
			_ = agents.HandleHook(args[0], os.Stdin)
			return nil
		},
	}

	claude.AddCommand(install, uninstall, record)
	return claude
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
