package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/jbarap/birds-eye/internal/agents"
	"github.com/jbarap/birds-eye/internal/theme"
)

// supportedAgentTypes are the integration types `be agents install`/`uninstall` accept.
// The <type> arg is the extension point for more agents; only claude ships today.
var supportedAgentTypes = map[string]bool{"claude": true}

// tier names the two independent, opt-in integration tiers.
type tier struct {
	name string // flag name: "hooks" / "workflows"
	desc string // one-line explanation shown in the checklist
}

var installTiers = []tier{
	{"hooks", "status detection — let bird's-eye see each agent's live status"},
	{"workflows", "bird's-eye-authored skills (orchestrator, QA, PR) that compose `be agents`"},
}

// newAgentsInstallCmd builds `be agents install <type>`: two independent opt-in tiers
// selected by --hooks/--workflows, an interactive checklist when neither is given in a
// TTY, and a refusal in a non-TTY. Nothing is ever installed implicitly.
func newAgentsInstallCmd() *cobra.Command {
	var hooks, workflows bool
	var settingsPath, command string
	cmd := &cobra.Command{
		Use:   "install <type>",
		Short: "Install an agent integration (hooks and/or workflows); opt-in per tier",
		Long: "Install the bird's-eye integration for an agent type (e.g. claude) in two\n" +
			"independent, opt-in tiers: --hooks (status detection) and --workflows\n" +
			"(skills that compose the be agents verbs). With no tier flag in a terminal a\n" +
			"checklist is shown; in a non-terminal the flags are required.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := args[0]
			if !supportedAgentTypes[t] {
				return fmt.Errorf("unsupported agent type %q (supported: claude)", t)
			}
			doHooks, doWorkflows, err := selectTiers(hooks, workflows, "install")
			if err != nil {
				return err
			}
			if doHooks {
				if err := installHooksTier(t, settingsPath, command); err != nil {
					return err
				}
			}
			if doWorkflows {
				if err := installWorkflowsTier(t); err != nil {
					return err
				}
			}
			if !doHooks && !doWorkflows {
				fmt.Println("Nothing selected; nothing installed.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&hooks, "hooks", false, "install the status-detection hooks tier")
	cmd.Flags().BoolVar(&workflows, "workflows", false, "install the workflow skills tier")
	cmd.Flags().StringVar(&settingsPath, "settings", "", "path to settings.json for the hooks tier (defaults to ~/.claude/settings.json)")
	cmd.Flags().StringVar(&command, "command", "", "hook command to install (defaults to the absolute path of this be binary)")
	return cmd
}

// newAgentsUninstallCmd builds `be agents uninstall <type>`: symmetric tier-scoped
// removal that touches only what bird's-eye wrote. With no flag in a TTY it offers a
// checklist of the currently-installed tiers.
func newAgentsUninstallCmd() *cobra.Command {
	var hooks, workflows bool
	var settingsPath string
	cmd := &cobra.Command{
		Use:   "uninstall <type>",
		Short: "Remove an agent integration tier (hooks and/or workflows); removes only bird's-eye's own entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := args[0]
			if !supportedAgentTypes[t] {
				return fmt.Errorf("unsupported agent type %q (supported: claude)", t)
			}
			doHooks, doWorkflows, err := selectUninstallTiers(t, hooks, workflows, settingsPath)
			if err != nil {
				return err
			}
			if doHooks {
				if err := uninstallHooksTier(t, settingsPath); err != nil {
					return err
				}
			}
			if doWorkflows {
				if err := uninstallWorkflowsTier(t); err != nil {
					return err
				}
			}
			if !doHooks && !doWorkflows {
				fmt.Println("Nothing selected; nothing removed.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&hooks, "hooks", false, "remove the status-detection hooks tier")
	cmd.Flags().BoolVar(&workflows, "workflows", false, "remove the workflow skills tier")
	cmd.Flags().StringVar(&settingsPath, "settings", "", "path to settings.json for the hooks tier (defaults to ~/.claude/settings.json)")
	return cmd
}

// selectTiers resolves which tiers to act on from the flags. When a flag is set it is
// honored directly. When neither is set it shows the interactive checklist in a TTY, or
// errors in a non-TTY (rather than prompt or guess).
func selectTiers(hooks, workflows bool, verb string) (doHooks, doWorkflows bool, err error) {
	if hooks || workflows {
		return hooks, workflows, nil
	}
	if !stdinIsTTY() {
		return false, false, fmt.Errorf("no terminal to prompt; pass --hooks and/or --workflows to %s", verb)
	}
	sel, err := runChecklist(fmt.Sprintf("Which tiers to %s?", verb), installTiers, nil)
	if err != nil {
		return false, false, err
	}
	return sel["hooks"], sel["workflows"], nil
}

// selectUninstallTiers is selectTiers for uninstall: the no-flag checklist is
// pre-populated with (and limited to) the tiers currently installed.
func selectUninstallTiers(agentType string, hooks, workflows bool, settingsPath string) (doHooks, doWorkflows bool, err error) {
	if hooks || workflows {
		return hooks, workflows, nil
	}
	if !stdinIsTTY() {
		return false, false, fmt.Errorf("no terminal to prompt; pass --hooks and/or --workflows to uninstall")
	}
	installed, err := installedTiers(agentType, settingsPath)
	if err != nil {
		return false, false, err
	}
	if !installed["hooks"] && !installed["workflows"] {
		fmt.Println("No bird's-eye integration is installed; nothing to remove.")
		return false, false, nil
	}
	sel, err := runChecklist("Which tiers to uninstall?", installTiers, installed)
	if err != nil {
		return false, false, err
	}
	// Only remove tiers that are actually installed, even if the user toggled others on.
	return sel["hooks"] && installed["hooks"], sel["workflows"] && installed["workflows"], nil
}

// installedTiers reports which tiers are currently present, for the uninstall checklist.
func installedTiers(agentType, settingsPath string) (map[string]bool, error) {
	out := map[string]bool{}
	path, err := hooksSettingsPath(settingsPath)
	if err != nil {
		return nil, err
	}
	if ok, err := agents.HooksInstalled(path); err != nil {
		return nil, err
	} else {
		out["hooks"] = ok
	}
	dir, err := agents.ClaudeConfigDir()
	if err != nil {
		return nil, err
	}
	if ok, err := agents.WorkflowsInstalled(agentType, dir); err != nil {
		return nil, err
	} else {
		out["workflows"] = ok
	}
	return out, nil
}

// hooksSettingsPath resolves the settings.json path for the hooks tier: the override
// when given, else the per-type default.
func hooksSettingsPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return agents.DefaultSettingsPath()
}

func installHooksTier(agentType, settingsPath, command string) error {
	path, err := hooksSettingsPath(settingsPath)
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
		fmt.Printf("Installed bird's-eye hooks into %s (backup at %s.bak)\n", path, path)
	} else {
		fmt.Printf("bird's-eye hooks already up to date in %s\n", path)
	}
	fmt.Printf("Events: %v\n", agents.ManagedEvents)
	return nil
}

func uninstallHooksTier(agentType, settingsPath string) error {
	path, err := hooksSettingsPath(settingsPath)
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
}

func installWorkflowsTier(agentType string) error {
	dir, err := agents.ClaudeConfigDir()
	if err != nil {
		return err
	}
	changed, err := agents.InstallWorkflows(agentType, dir)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("Installed bird's-eye workflow skills into %s/skills (birds-eye-*)\n", dir)
	} else {
		fmt.Printf("bird's-eye workflow skills already up to date in %s/skills\n", dir)
	}
	return nil
}

func uninstallWorkflowsTier(agentType string) error {
	dir, err := agents.ClaudeConfigDir()
	if err != nil {
		return err
	}
	changed, err := agents.UninstallWorkflows(agentType, dir)
	if err != nil {
		return err
	}
	if changed {
		fmt.Printf("Removed bird's-eye workflow skills from %s/skills\n", dir)
	} else {
		fmt.Printf("No bird's-eye workflow skills found in %s/skills\n", dir)
	}
	return nil
}

// stdinIsTTY reports whether stdin is an interactive terminal, so the no-flag form knows
// whether it may prompt.
func stdinIsTTY() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// checklistModel is a minimal multi-select for the install/uninstall tiers. Space toggles
// the item under the cursor, enter confirms, q/esc cancels (selecting nothing).
type checklistModel struct {
	title     string
	tiers     []tier
	checked   map[string]bool
	cursor    int
	done      bool
	cancelled bool
}

func (m checklistModel) Init() tea.Cmd { return nil }

func (m checklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled, m.done = true, true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.tiers)-1 {
			m.cursor++
		}
	case " ", "x":
		name := m.tiers[m.cursor].name
		m.checked[name] = !m.checked[name]
	case "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m checklistModel) View() string {
	if m.done {
		return ""
	}
	accent := lipgloss.Color(theme.Accent.Hex)
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Gray.Hex))
	cursorStyle := lipgloss.NewStyle().Foreground(accent)

	out := lipgloss.NewStyle().Bold(true).Render(m.title) + "\n\n"
	for i, t := range m.tiers {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render(theme.CursorGlyph + " ")
		}
		box := "[ ]"
		if m.checked[t.name] {
			box = "[x]"
		}
		line := fmt.Sprintf("%s%s %s  %s", cursor, box, t.name, gray.Render(t.desc))
		out += line + "\n"
	}
	out += "\n" + gray.Render("space toggle · enter confirm · q cancel") + "\n"
	return out
}

// runChecklist runs the interactive multi-select and returns the chosen tier set.
// initial pre-checks items (used by uninstall to default to the installed tiers); a
// cancel returns an empty selection with no error (treated as a no-op by the caller).
func runChecklist(title string, tiers []tier, initial map[string]bool) (map[string]bool, error) {
	checked := map[string]bool{}
	for name, on := range initial {
		checked[name] = on
	}
	m := checklistModel{title: title, tiers: tiers, checked: checked}
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, err
	}
	res := out.(checklistModel)
	if res.cancelled {
		return map[string]bool{}, nil
	}
	return res.checked, nil
}
