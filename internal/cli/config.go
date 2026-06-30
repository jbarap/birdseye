package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/jbarap/birdseye/internal/agents"
	"github.com/jbarap/birdseye/internal/config"
	"github.com/jbarap/birdseye/internal/worktree"
)

// newConfigCmd builds `be config`. By default it prints the effective configuration
// (built-in defaults with your user config - and, when run inside a repo, that repo's
// .birdseye/config.toml - merged on top), so you can see exactly which values are in
// effect and where they came from. Two alternate modes:
//
//	--defaults   the annotated default template (structure + defaults + how to override)
//	--check      validate the config files and report unknown or invalid fields
//
// The default and --defaults output are valid TOML and can seed a config file.
func newConfigCmd() *cobra.Command {
	var showDefaults, check bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the effective configuration (--defaults for the template, --check to lint)",
		Long: "Show the effective birdseye configuration: built-in defaults with your user " +
			"config, and a repo's .birdseye/config.toml when run inside one, merged on top. " +
			"Use --defaults for the annotated template and --check to validate your config files.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showDefaults && check {
				return fmt.Errorf("--defaults and --check are mutually exclusive")
			}
			switch {
			case showDefaults:
				return runConfigDefaults()
			case check:
				return runConfigCheck()
			default:
				return runConfigShow()
			}
		},
	}
	cmd.Flags().BoolVar(&showDefaults, "defaults", false, "print the annotated default configuration template instead of the effective config")
	cmd.Flags().BoolVar(&check, "check", false, "validate the config files and report unknown or invalid fields")
	return cmd
}

// configSource is one file that feeds the effective configuration, in precedence order.
type configSource struct {
	label  string // "user" or "repo"
	path   string
	loaded bool // the file exists and contributed; otherwise lower-precedence values stand
}

// effectiveConfig loads the user config and, when the cwd is inside a git repository,
// overlays that repo's .birdseye/config.toml - the same resolution birdseye uses for
// repository-scoped operations. It returns the merged config and the sources that fed it.
func effectiveConfig() (config.Config, []configSource, error) {
	cfg, userPath, err := config.Load()
	if err != nil {
		return config.Config{}, nil, err
	}
	sources := []configSource{{label: "user", path: userPath, loaded: fileExists(userPath)}}
	if root, ok := repoRoot(); ok {
		cfg, err = config.Overlay(cfg, root)
		if err != nil {
			return config.Config{}, nil, err
		}
		repoPath := filepath.Join(root, config.RepoConfigName)
		sources = append(sources, configSource{label: "repo", path: repoPath, loaded: fileExists(repoPath)})
	}
	return cfg, sources, nil
}

// repoRoot returns the primary worktree of the git repository the current directory
// belongs to - the root birdseye reads .birdseye/config.toml from - or ok=false when
// the cwd is not inside a git worktree.
func repoRoot() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	info, ok := worktree.Resolve(cwd)
	if !ok {
		return "", false
	}
	return filepath.Dir(info.GitDir), true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// runConfigShow prints the effective configuration as TOML, preceded by a comment block
// naming the sources that contributed (in precedence order). Runtime defaults are
// materialized so unset keys show the value actually used, not a blank.
func runConfigShow() error {
	cfg, sources, err := effectiveConfig()
	if err != nil {
		return fmt.Errorf("%w; run `be config --check` for a full report", err)
	}
	var h strings.Builder
	h.WriteString("# Effective birdseye configuration: built-in defaults with your config files\n")
	h.WriteString("# merged on top (later sources win). `be config --defaults` prints the\n")
	h.WriteString("# annotated template; `be config --check` validates your files.\n#\n")
	h.WriteString("# Sources, low to high precedence:\n")
	h.WriteString("#   defaults  (built-in)\n")
	for _, s := range sources {
		status := "absent - lower-precedence values stand"
		if s.loaded {
			status = "loaded"
		}
		fmt.Fprintf(&h, "#   %-9s %s (%s)\n", s.label, s.path, status)
	}
	h.WriteString("\n")
	if _, err := os.Stdout.WriteString(h.String()); err != nil {
		return err
	}
	return toml.NewEncoder(os.Stdout).Encode(cfg.WithRuntimeDefaults())
}

// runConfigDefaults prints the annotated default-configuration template.
func runConfigDefaults() error {
	if path, err := config.Path(); err == nil {
		fmt.Fprintf(os.Stdout, "# Config path: %s\n#\n", path)
	}
	fmt.Fprint(os.Stdout, config.Reference())
	return nil
}

// runConfigCheck validates the config files structurally (unknown or misspelled fields,
// malformed TOML) and then validates the values of the effective config (keybindings,
// accent, split, refresh). It prints a per-file report and returns a non-zero error when
// any problem is found, so scripts and pre-commit hooks can gate on it.
func runConfigCheck() error {
	problems := 0
	report := func(format string, a ...any) {
		problems++
		fmt.Fprintf(os.Stdout, "    %s %s\n", warnPrefix.Render("✗"), fmt.Sprintf(format, a...))
	}

	userPath, err := config.Path()
	if err != nil {
		return err
	}
	files := []configSource{{label: "user", path: userPath}}
	if root, ok := repoRoot(); ok {
		files = append(files, configSource{label: "repo", path: filepath.Join(root, config.RepoConfigName)})
	}
	parseFailed := false
	for _, f := range files {
		if !fileExists(f.path) {
			fmt.Fprintf(os.Stdout, "%s  %s (not present)\n", f.label, f.path)
			continue
		}
		fmt.Fprintf(os.Stdout, "%s  %s\n", f.label, f.path)
		unknown, err := config.CheckFile(f.path)
		if err != nil {
			report("%v", err)
			parseFailed = true
			continue
		}
		for _, k := range unknown {
			report("unrecognized: %s", k)
			if strings.HasPrefix(k, "worktree.") {
				fmt.Fprintln(os.Stdout, "      hint: the [worktree] provider roots moved to [repo] (roots = [\"~/code\"])")
			}
		}
	}

	// Value checks (keybindings, accent, split, refresh) need the merged config. An
	// unrecognized key is tolerated by the lenient merge, so these still run alongside the
	// unknown-key findings above - the user sees every problem in one pass. Only a malformed
	// file blocks them, and that error was already reported by the file loop.
	if !parseFailed {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.path
		}
		if cfg, err := config.EffectiveLenient(paths...); err != nil {
			report("%v", err)
		} else {
			if _, err := agents.ResolveKeymap(cfg.Agents.Keys); err != nil {
				report("%v", err)
			}
			if _, err := agents.ResolveAccent(cfg.Agents.Accent); err != nil {
				report("%v", err)
			}
			if _, err := cfg.Agents.SplitRatio(); err != nil {
				report("%v", err)
			}
			if _, err := cfg.Agents.RefreshInterval(); err != nil {
				report("%v", err)
			}
		}
	}

	if problems > 0 {
		return fmt.Errorf("found %d configuration problem(s)", problems)
	}
	fmt.Fprintln(os.Stdout, okMark.Render("✓"), "configuration OK")
	return nil
}
