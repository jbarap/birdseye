// Package config loads birdseye configuration from a TOML file under the
// platform config directory, applying built-in defaults for absent keys.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	Tmuxp  Tmuxp  `toml:"tmuxp"`
	Dir    Dir    `toml:"dir"`
	Repo   Repo   `toml:"repo"`
	Agents Agents `toml:"agents"`
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

// Repo configures the repository session provider.
type Repo struct {
	// Roots are optional discovery directories scanned for git repositories. Each
	// repository found directly under a root becomes one picker candidate that opens a
	// session at its primary worktree; when empty the provider yields no candidates.
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
	// Split is the agents-list pane's share of the width when the preview is shown, as a
	// fraction of the terminal in (0,1): the list fills out to this share and the preview
	// takes the rest. Zero keeps the built-in default.
	Split float64 `toml:"split"`
}

// DefaultSplit is the agents-list pane's share of the width when none is configured. Agent
// rows are short, so this leans toward the preview; raise it for a wider list.
const DefaultSplit = 0.4

// SplitRatio resolves the agents-list pane's width share, defaulting to DefaultSplit. It is
// clamped to a sane band so the list is never cramped and the preview never vanishes; a
// value outside (0,1) is reported as an error rather than silently coerced.
func (a Agents) SplitRatio() (float64, error) {
	if a.Split == 0 {
		return DefaultSplit, nil
	}
	if a.Split <= 0 || a.Split >= 1 {
		return 0, fmt.Errorf("invalid agents.split %v: want a fraction between 0 and 1 (e.g. 0.4)", a.Split)
	}
	const lo, hi = 0.3, 0.8
	if a.Split < lo {
		return lo, nil
	}
	if a.Split > hi {
		return hi, nil
	}
	return a.Split, nil
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
		Order:     []string{"tmux", "tmuxp", "dir", "repo"},
		Labels: map[string]string{
			"tmux":  "session",
			"tmuxp": "template",
			"dir":   "dir",
			"repo":  "repo",
		},
		Icons: map[string]string{
			"tmux":  "", // oct-terminal
			"tmuxp": "󰏭", // md-pencil_box_outline
			"dir":   "", // cod-folder
			"repo":  "󰘬", // md-source_branch
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
	return filepath.Join(dir, "birdseye", "config.toml"), nil
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
	// The provider's config moved from `[worktree]` (and its earlier single `root` key)
	// to `[repo] roots`. Fail loudly rather than silently ignoring an old config.
	for _, key := range md.Undecoded() {
		if strings.HasPrefix(key.String(), "worktree.") {
			return Default(), fmt.Errorf("invalid config %s: the [worktree] section has been renamed; use [repo] with a roots list, e.g. roots = [\"~/code\"]", path)
		}
	}
	cfg.expandPaths()
	return cfg, nil
}

// expandPaths rewrites every path-typed config field through expandTilde, so a
// leading "~" means the user's home everywhere a path is accepted. The shell
// does this for command-line paths; values read from the config file never pass
// through a shell, so we expand them ourselves.
func (c *Config) expandPaths() {
	c.Tmuxp.Dir = expandTilde(c.Tmuxp.Dir)
	for i, p := range c.Dir.Roots {
		c.Dir.Roots[i] = expandTilde(p)
	}
	for i, p := range c.Repo.Roots {
		c.Repo.Roots[i] = expandTilde(p)
	}
}

// expandTilde resolves a leading "~" or "~/" to the user's home directory. Other
// values (absolute, relative, empty, or a "~user" form we don't support) are
// returned unchanged; if the home directory can't be resolved the path is left
// as-is rather than failing the whole config load.
func expandTilde(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
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
