// Package config loads bird's-eye configuration from a TOML file under the
// platform config directory, applying built-in defaults for absent keys.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the fully-resolved configuration (defaults merged with the user
// file).
type Config struct {
	// Providers maps a provider type to whether it is enabled. A type absent
	// from the map is enabled by default, so users only list what to disable.
	Providers map[string]bool `toml:"providers"`
	// Order is the display order of candidate types in the picker.
	Order []string `toml:"order"`
	// Labels maps a type to its display label; absent types fall back to the
	// type name.
	Labels map[string]string `toml:"labels"`
	// Icons maps a type to a glyph shown beside its candidates in the picker.
	// Defaults are Nerd Font glyphs; override with your own text/emoji, or set a
	// type to "" to hide its icon.
	Icons map[string]string `toml:"icons"`

	Tmuxp    Tmuxp    `toml:"tmuxp"`
	Dir      Dir      `toml:"dir"`
	Worktree Worktree `toml:"worktree"`
	Agents   Agents   `toml:"agents"`
}

// Tmuxp configures the tmuxp templates provider.
type Tmuxp struct {
	// Dir overrides the tmuxp config directory used to discover templates.
	// Empty means tmuxp's own default.
	Dir string `toml:"dir"`
}

// Dir configures the directory/zoxide provider.
type Dir struct {
	// UseZoxide includes zoxide's known directories when zoxide is available.
	UseZoxide bool `toml:"use_zoxide"`
	// Roots are directories whose immediate children become candidates.
	Roots []string `toml:"roots"`
}

// Worktree configures the git-worktree provider.
type Worktree struct {
	// Roots are optional discovery directories scanned for managed repositories
	// (each a <repo>/ container of git worktrees). It replaces the former single
	// `root`; when empty the provider yields no candidates. Worktrees can live
	// anywhere — these roots only feed the picker.
	Roots []string `toml:"roots"`
}

// Agents configures the agent view. Agent state is now reclaimed by process
// liveness (an entry exists only while its Claude process runs), so there are no
// retention/stale knobs here.
type Agents struct {
	// Keys overrides the agents-view keybindings, mapping an action name (under
	// [agents.keys]) to the keys that trigger it. Absent actions keep their
	// built-in bindings; an action listed here replaces (not extends) its keys.
	Keys map[string][]string `toml:"keys"`
	// Accent overrides the agents-view accent color (the title and the cursor
	// indicator). A #rrggbb hex; empty keeps the built-in default.
	Accent string `toml:"accent"`
	// Refresh is the live-refresh interval (a Go duration like "1s"); empty keeps
	// the one-second default.
	Refresh string `toml:"refresh"`
	// Command is the program spawned for a new orchestration agent; empty defaults
	// to "claude".
	Command string `toml:"command"`
}

// RefreshInterval resolves the configured live-refresh interval, defaulting to one
// second. A malformed or non-positive duration is reported as an error.
func (a Agents) RefreshInterval() (time.Duration, error) {
	if a.Refresh == "" {
		return time.Second, nil
	}
	d, err := time.ParseDuration(a.Refresh)
	if err != nil {
		return 0, fmt.Errorf("invalid agents.refresh %q: want a duration like \"1s\"", a.Refresh)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid agents.refresh %q: must be positive", a.Refresh)
	}
	return d, nil
}

// AgentCommand resolves the command spawned for a new agent, defaulting to "claude".
func (a Agents) AgentCommand() string {
	if a.Command == "" {
		return "claude"
	}
	return a.Command
}

// Default returns the built-in configuration used when no file (or no key) is
// present.
func Default() Config {
	return Config{
		Providers: map[string]bool{},
		Order:     []string{"tmux", "tmuxp", "dir", "worktree"},
		Labels: map[string]string{
			"tmux":     "session",
			"tmuxp":    "template",
			"dir":      "dir",
			"worktree": "worktree",
		},
		Icons: map[string]string{
			"tmux":     "", // oct-terminal
			"tmuxp":    "󰏭", // md-pencil_box_outline
			"dir":      "", // cod-folder
			"worktree": "󰘬", // md-source_branch
		},
		Dir: Dir{UseZoxide: true},
	}
}

// Path returns the resolved config file path under the platform config dir.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "birds-eye", "config.toml"), nil
}

// Load reads the config from the default path, returning defaults when the file
// is absent. It returns the path it used and a clear error on malformed input.
func Load() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Default(), "", err
	}
	cfg, err := LoadFrom(path)
	return cfg, path, err
}

// LoadFrom reads the config from a specific path. A missing file yields
// defaults; a malformed file yields an error naming the file.
func LoadFrom(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Default(), fmt.Errorf("reading config %s: %w", path, err)
	}
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Default(), fmt.Errorf("invalid config %s: %w", path, err)
	}
	// The single `[worktree] root` key was replaced by `roots` (a list of discovery
	// paths). Fail loudly rather than silently ignoring an old config.
	for _, key := range md.Undecoded() {
		if key.String() == "worktree.root" {
			return Default(), fmt.Errorf("invalid config %s: [worktree] root is no longer supported; use a list, e.g. roots = [\"~/code\"]", path)
		}
	}
	return cfg, nil
}

// Label returns the display label for a type, falling back to the type itself.
func (c Config) Label(typ string) string {
	if l, ok := c.Labels[typ]; ok && l != "" {
		return l
	}
	return typ
}

// Icon returns the configured glyph for a type, or empty when none is set.
func (c Config) Icon(typ string) string {
	return c.Icons[typ]
}
