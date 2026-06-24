package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jbarap/birdseye/internal/agents"
	"github.com/jbarap/birdseye/internal/config"
	"github.com/jbarap/birdseye/internal/fleet"
	"github.com/jbarap/birdseye/internal/picker"
	"github.com/jbarap/birdseye/internal/provider"
	"github.com/jbarap/birdseye/internal/tmux"
	"github.com/jbarap/birdseye/internal/tools"
	"github.com/jbarap/birdseye/internal/worktree"
)

// newAgentsCmd builds the `be agents` headless verb namespace. These verbs and the
// `be dash` TUI resolve to the same operation layer (internal/fleet), so a `close`
// from a script and a close in the dash are the identical operation.
func newAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Headless agent fleet management (list, status, spawn, send, jump, close, delete)",
		Long: "Drive the agent fleet from any shell. Units of work are addressed by a derived " +
			"repo/worktree handle (a worktreeless agent by its tmux pane id); data verbs emit " +
			"--json. The same operations back the be dash TUI.",
	}
	cmd.AddCommand(
		newAgentsListCmd(),
		newAgentsStatusCmd(),
		newAgentsSpawnCmd(),
		newAgentsCloseCmd(),
		newAgentsDeleteCmd(),
		newAgentsJumpCmd(),
		newAgentsSendCmd(),
		newAgentsInstallCmd(),
		newAgentsUninstallCmd(),
	)
	return cmd
}

// buildFleet wires the shared operation layer from config and the tmux backend.
func buildFleet() (*fleet.Fleet, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	client, err := tmux.New()
	if err != nil {
		return nil, err
	}
	src, err := agents.NewClaudeSource()
	if err != nil {
		return nil, err
	}
	return fleet.New(client, cfg.Agents.AgentCommand(), src, paneLister{client}, repoResolver{}), nil
}

func newAgentsListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the agent fleet (work handles, status, tmux location)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildFleet()
			if err != nil {
				return err
			}
			recs, err := f.List()
			if err != nil {
				return err
			}
			return printRecords(recs, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func newAgentsStatusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status <handle>",
		Short: "Show the status record for one work handle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildFleet()
			if err != nil {
				return err
			}
			rec, err := f.Status(args[0])
			if err != nil {
				return err
			}
			return printRecords([]fleet.Record{rec}, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func newAgentsSpawnCmd() *cobra.Command {
	var branch, prompt string
	cmd := &cobra.Command{
		Use:   "spawn <dir>",
		Short: "Spawn an agent on a new sibling worktree of the repo at <dir>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(branch) == "" {
				return fmt.Errorf("spawn requires --branch to name the worktree")
			}
			f, err := buildFleet()
			if err != nil {
				return err
			}
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			// Session "" → derived from the repo name; spawn ensures-or-reuses it.
			handle, err := f.Spawn(abs, "", branch, "", prompt)
			if err != nil {
				return err
			}
			fmt.Println(handle)
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "branch to check out in the new worktree (required)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "initial prompt to seed the agent with")
	return cmd
}

func newAgentsCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <handle>",
		Short: "Close the work's tmux window, keeping the worktree on disk",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildFleet()
			if err != nil {
				return err
			}
			t, err := f.Resolve(args[0])
			if err != nil {
				return err
			}
			return f.Close(t)
		},
	}
}

func newAgentsDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <handle>",
		Short: "Close the window and remove the git worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildFleet()
			if err != nil {
				return err
			}
			t, err := f.Resolve(args[0])
			if err != nil {
				return err
			}
			err = f.Delete(t, force)
			if err == agents.ErrWorktreeDirty {
				return fmt.Errorf("%s has uncommitted changes; pass --force to remove it anyway", t.Handle)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "force-remove a dirty worktree")
	return cmd
}

func newAgentsJumpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "jump <handle>",
		Short: "Connect your tmux client to the work's session/window/pane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildFleet()
			if err != nil {
				return err
			}
			t, err := f.Resolve(args[0])
			if err != nil {
				return err
			}
			return f.Jump(t)
		},
	}
}

func newAgentsSendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "send <handle> <input>",
		Short: "Dispatch input to the work's agent (best-effort; reports sent, not received)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := buildFleet()
			if err != nil {
				return err
			}
			t, err := f.Resolve(args[0])
			if err != nil {
				return err
			}
			input := strings.Join(args[1:], " ")
			if err := f.Send(t, input); err != nil {
				return err
			}
			// send has no readiness signal; report dispatch, not receipt.
			fmt.Fprintf(os.Stderr, "dispatched to %s (not confirmed received)\n", t.Handle)
			return nil
		},
	}
}

// printRecords writes records as JSON (machine) or an aligned table (human).
func printRecords(recs []fleet.Record, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(recs)
	}
	if len(recs) == 0 {
		fmt.Println(infoStyle.Render("No agents or worktrees to show."))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "HANDLE\tKIND\tSTATUS\tBRANCH\tWINDOW\tPANE")
	for _, r := range recs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			dash(r.Handle), r.Kind, dash(r.Status), dash(r.Branch), dash(r.Window), dash(r.Pane))
	}
	return w.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// runDash launches the live agents view (`be dash`). Without tmux it still works for
// triage (plain rows, no preview, no orchestration); with tmux it gains the preview,
// managed-repo recognition, and the create/delete actions — all routed through the
// shared fleet so the dash and the `be agents` verbs stay in lifecycle parity.
func runDash() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	keys, err := agents.ResolveKeymap(cfg.Agents.Keys)
	if err != nil {
		return err
	}
	accent, err := agents.ResolveAccent(cfg.Agents.Accent)
	if err != nil {
		return err
	}
	refresh, err := cfg.Agents.RefreshInterval()
	if err != nil {
		return err
	}
	src, err := agents.NewClaudeSource()
	if err != nil {
		return err
	}

	var rowSrc agents.RowSource = agents.AgentsAsRows(src)
	var prev agents.Previewer = agents.NoopPreviewer{}
	var orch agents.Orchestrator
	client, clientErr := tmux.New()
	if clientErr == nil {
		prev = agents.NewTmuxPreviewer(client, 200)
		rowSrc = agents.NewWorkspace(src, paneLister{client}, repoResolver{})
		f := fleet.New(client, cfg.Agents.AgentCommand(), src, paneLister{client}, repoResolver{})
		orch = orchestrator{fleet: f, client: client, cfg: cfg}
	}

	chosen, err := agents.Run(rowSrc, prev, keys, accent, refresh, orch)
	if err != nil {
		return err
	}
	if chosen == nil || chosen.TmuxSession == "" {
		return nil
	}
	if client == nil {
		warn("cannot jump to agent: %v", clientErr)
		return nil
	}
	if err := client.ConnectPane(chosen.TmuxSession, chosen.TmuxWindow, chosen.TmuxPane); err != nil {
		return fmt.Errorf("jumping to %s: %w", chosen.TmuxSession, err)
	}
	return nil
}

// paneLister adapts *tmux.Client to agents.PaneLister.
type paneLister struct{ c *tmux.Client }

func (p paneLister) ListPanes() ([]agents.PaneInfo, error) {
	ps, err := p.c.ListPanes()
	if err != nil {
		return nil, err
	}
	out := make([]agents.PaneInfo, len(ps))
	for i, x := range ps {
		out[i] = agents.PaneInfo{
			Session:     x.Session,
			WindowIndex: x.WindowIndex,
			WindowName:  x.WindowName,
			PaneID:      x.PaneID,
			StartPath:   x.StartPath,
		}
	}
	return out, nil
}

// repoResolver adapts worktree.Resolve / worktree.ListWorktrees to agents.RepoResolver.
type repoResolver struct{}

func (repoResolver) Resolve(dir string) (agents.RepoInfo, bool) {
	info, ok := worktree.Resolve(dir)
	if !ok {
		return agents.RepoInfo{}, false
	}
	return agents.RepoInfo{
		GitDir:    info.GitDir,
		Repo:      info.Repo,
		TopLevel:  info.TopLevel,
		Worktree:  info.Worktree,
		IsPrimary: info.IsPrimary,
	}, true
}

// Worktrees enumerates the repository's git worktree set. gitDir is the shared git
// common dir; its parent is the primary worktree, a valid directory to run git from.
func (repoResolver) Worktrees(gitDir string) ([]agents.WorktreeInfo, error) {
	wts, err := worktree.ListWorktrees(filepath.Dir(gitDir))
	if err != nil {
		return nil, err
	}
	out := make([]agents.WorktreeInfo, len(wts))
	for i, w := range wts {
		out[i] = agents.WorktreeInfo{
			Path:      w.Path,
			Name:      filepath.Base(w.Path),
			Branch:    w.Branch,
			IsPrimary: w.IsPrimary,
		}
	}
	return out, nil
}

// orchestrator implements agents.Orchestrator by delegating to the shared fleet, so a
// dash action (spawn/close/delete) is the same operation a `be agents` verb invokes.
// NewSession stays here: the interactive fzf picker is a dash-only ergonomic with no
// headless analog (its headless counterpart is `be sessions --json` + spawn).
type orchestrator struct {
	fleet  *fleet.Fleet
	client *tmux.Client
	cfg    config.Config
}

// Spawn adds a worktree in the row's repo and starts the agent, via the shared fleet.
func (o orchestrator) Spawn(repo agents.Row, branch, name string) error {
	_, err := o.fleet.Spawn(repo.Dir, repo.TmuxSession, branch, name, "")
	return err
}

// Close tears down a row's tmux window only (no worktree removal) via the shared fleet,
// so a dash `dd` and `be agents close` are the same operation.
func (o orchestrator) Close(r agents.Row) error {
	return o.fleet.Close(fleet.FromRow(r))
}

// Remove tears down a row through the shared fleet's Delete (window + worktree).
func (o orchestrator) Remove(r agents.Row, force bool) error {
	return o.fleet.Delete(fleet.FromRow(r), force)
}

// NewSession runs the session picker (the same providers and fuzzy UI as `be sessions`)
// and materializes the chosen candidate without attaching, returning its session name.
// Cancelling or an empty candidate list returns "" with no error. Not attaching keeps
// the agents view in front: ensureOnly runs the candidate's Ensure but skips Connect.
func (o orchestrator) NewSession() (string, error) {
	reg := buildRegistry(o.cfg, tools.Probe(), o.client)
	sel, err := picker.Pick(reg.Candidates(), o.cfg)
	switch err {
	case nil:
		// proceed
	case picker.ErrCancelled, picker.ErrNoCandidates:
		return "", nil
	default:
		return "", err
	}
	if sel == nil {
		return "", nil
	}
	if err := sel.Action(ensureOnly{o.client}); err != nil {
		return "", err
	}
	return sel.Name, nil
}

// ensureOnly is a provider.Backend that creates sessions but never connects, so a
// candidate's action materializes its tmux session without pulling the user out of the
// agents view into the new session.
type ensureOnly struct{ b provider.Backend }

func (e ensureOnly) Ensure(name, dir string) error { return e.b.Ensure(name, dir) }
func (e ensureOnly) Connect(string) error          { return nil }
