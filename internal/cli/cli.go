// Package cli wires the birdseye commands together: it builds the provider
// registry from config and tool availability, and routes the picker, agents,
// worktree, and hook commands.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/jbarap/birdseye/internal/config"
	"github.com/jbarap/birdseye/internal/picker"
	"github.com/jbarap/birdseye/internal/provider"
	"github.com/jbarap/birdseye/internal/providers/dir"
	"github.com/jbarap/birdseye/internal/providers/tmuxp"
	"github.com/jbarap/birdseye/internal/providers/tmuxsess"
	"github.com/jbarap/birdseye/internal/theme"
	"github.com/jbarap/birdseye/internal/tmux"
	"github.com/jbarap/birdseye/internal/tools"
	"github.com/jbarap/birdseye/internal/worktree"
)

// Shared output styles. lipgloss auto-disables color when the target stream is
// not a terminal, so these degrade to plain text under pipes/redirects. warn
// uses a renderer bound to stderr; the rest target stdout (the default).
var (
	errRenderer = lipgloss.NewRenderer(os.Stderr)
	warnPrefix  = errRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.Red.Hex))

	okMark    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.Green.Hex))
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent.Hex))
	hintStyle = lipgloss.NewStyle().Faint(true)
)

// Execute builds and runs the root command. version is shown by `be --version`.
func Execute(version string) error {
	root := &cobra.Command{
		Use:           "be",
		Short:         "birdseye — a live view over your tmux agent sessions",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// A subcommand is required: bare `be` no longer launches a view. The TUI is
		// `be dash`; `be sessions` is the fuzzy picker; `be agents` is the headless
		// verb namespace. RunE errors so the exit status is non-zero.
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = false
			return fmt.Errorf("a subcommand is required (try `be dash`, `be sessions`, or `be agents`)")
		},
	}
	root.AddCommand(
		&cobra.Command{
			Use:   "dash",
			Short: "Live agents view (the TUI; suitable for a tmux popup)",
			RunE:  func(cmd *cobra.Command, args []string) error { return runDash() },
		},
		newSessionsCmd(),
		&cobra.Command{
			Use:    "list",
			Hidden: true, // deprecated alias for `be sessions`
			RunE:   func(cmd *cobra.Command, args []string) error { return runList() },
		},
		newAgentsCmd(),
		newWorktreeCmd(),
		newHookCmd(),
		newConfigCmd(),
	)
	return root.Execute()
}

// newSessionsCmd builds `be sessions`: the interactive fuzzy picker, plus a headless
// `--json` enumeration of the same provider registry's candidates (so a client can
// discover launch targets and feed one to `be agents spawn`).
func newSessionsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Fuzzy-pick a tmux session to attach to or create (--json to enumerate)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				return runSessionsJSON()
			}
			return runList()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "enumerate candidates as JSON instead of opening the picker")
	return cmd
}

func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", warnPrefix.Render("be:"), fmt.Sprintf(format, a...))
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
	r.Register(dir.New(cfg.Dir.UseZoxide && set.Zoxide, cfg.Dir.ZoxideLimit, cfg.Dir.Roots))
	r.Register(worktree.NewProvider(cfg.Repo.Roots))
	r.SetEnabled(cfg.Providers)
	return r
}

// sessionCandidate is one launch candidate in the headless `be sessions --json`
// contract: the same fields the picker draws, plus the directory (where applicable)
// so the output can feed `be agents spawn`.
type sessionCandidate struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Kind  string `json:"kind"` // "attach" or "create"
	Dir   string `json:"dir,omitempty"`
}

// runSessionsJSON serializes the provider registry's candidates non-interactively,
// over the same enabled providers the fuzzy picker uses (disabled providers are absent
// from both). It does not invoke fzf.
func runSessionsJSON() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	// tmux drives only the running-sessions provider; its absence is non-fatal here, so
	// the rest of the registry still enumerates.
	client, _ := tmux.New()
	var lister tmuxsess.Lister
	if client != nil {
		lister = client
	}
	reg := buildRegistry(cfg, tools.Probe(), lister)
	cands := reg.Candidates()
	out := make([]sessionCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, sessionCandidate{
			Name:  c.Name,
			Label: c.Label,
			Type:  c.Type,
			Kind:  c.Kind.String(),
			Dir:   c.Dir,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
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
		fmt.Println(infoStyle.Render("No sessions to show yet."))
		fmt.Println(hintStyle.Render("Add tmuxp templates, zoxide dirs, or configure providers."))
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
