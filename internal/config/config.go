// Package config loads birdseye configuration from a TOML file under the
// platform config directory, applying built-in defaults for absent keys.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// referenceTOML is the annotated configuration reference shipped with the
// binary and emitted by `be config`. It documents the structure, the built-in
// defaults, and how to override each key. A guard test asserts it parses back
// into Default(), so it cannot drift from the real defaults.
//
//go:embed default.toml
var referenceTOML string

// Reference returns the annotated default-configuration TOML: every section and
// key with its built-in default and override notes. `be config` prints this so a
// user can see the defaults and structure without hunting through the README.
func Reference() string {
	return referenceTOML
}

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
	Repo     Repo     `toml:"repo"`
	Agents   Agents   `toml:"agents"`
	Worktree Worktree `toml:"worktree"`
	// Workspaces is the dash namespaces feature's master switch (the `[workspaces]` table). Its
	// Enabled flag defaults to true (set in Default()); an explicit `enabled = false` turns the
	// whole feature off - no tab bar, no filtering, no automatic workspaces.
	Workspaces WorkspaceSettings `toml:"workspaces"`
	// WorkspaceDefs are the dash's path-derived namespaces (the `[[workspace]]` tables): each
	// slices `be dash` to the repositories under its roots. When the feature is enabled, repositories
	// under no declared root fall back to an automatic parent-derived workspace, so the array may be
	// empty and grouping still appears. See ResolveWorkspaces for membership and validation.
	WorkspaceDefs []Workspace `toml:"workspace"`
}

// WorkspaceSettings holds the dash namespaces feature's master switch. The default lives in
// Default() (like Dir.UseZoxide), so an absent [workspaces] table means the feature is enabled
// while an explicit `enabled = false` overrides it.
type WorkspaceSettings struct {
	// Enabled gates the whole namespaces feature: the tab bar, path-derived filtering, and the
	// automatic parent-derived workspaces. False makes the dash behave as if the feature did not exist.
	Enabled bool `toml:"enabled"`
}

// Workspace is one dash namespace: a name plus the roots whose repositories belong to it. A
// repository maps to the workspace whose root contains its path (longest match wins). Roots
// follow the same tilde/glob conventions as [repo].roots and [dir].roots.
type Workspace struct {
	Name  string   `toml:"name"`
	Roots []string `toml:"roots"`
}

// Worktree configures worktree creation. Like every section it can be set in the user
// config and overridden per-repository via .birdseye/config.toml (see Overlay).
type Worktree struct {
	// Setup is the command run on every worktree creation (after the .worktreeinclude
	// copy). Empty means no setup command runs.
	Setup string `toml:"setup"`
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
	// Split is the lens sidebar's share of the terminal width when the preview is shown, as a
	// fraction in (0,1): the stacked Agents/Projects lenses take this share of the width on the
	// left and the preview fills the rest at full height. Zero keeps the built-in default.
	Split float64 `toml:"split"`
	// Notify configures the out-of-band notifications birdseye emits when an agent crosses
	// into an urgent status. An absent [agents.notify] table means both edges notify with
	// automatic delivery.
	Notify Notify `toml:"notify"`
}

// Notify configures agent notifications. The edge flags default to true; that default lives in
// Default() (like Dir.UseZoxide), so an absent [agents.notify] table means both edges notify
// while an explicit `false` in the file overrides it.
type Notify struct {
	// Command is a shell command run to deliver each notification, receiving the alert as the
	// environment variables BE_AGENT, BE_STATUS, BE_REPO, BE_CWD, BE_MESSAGE. Empty selects the
	// auto-detected platform notifier (notify-send/osascript, terminal-bell fallback).
	Command string `toml:"command"`
	// NeedsAttention gates the "an agent entered needs-attention" edge.
	NeedsAttention bool `toml:"needs_attention"`
	// Finished gates the "an agent finished its turn (working -> idle)" edge.
	Finished bool `toml:"finished"`
}

// DefaultSplit is the lens sidebar's share of the terminal width when none is configured. The
// lens lists are narrow, so this leans toward the preview; raise it for a wider sidebar.
const DefaultSplit = 0.4

// SplitRatio resolves the lens sidebar's width share, defaulting to DefaultSplit. It is clamped
// to a sane band so the sidebar is never cramped and the preview never vanishes; a value outside
// (0,1) is reported as an error rather than silently coerced.
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

// ResolveWorkspaces validates the configured `[[workspace]]` namespaces and returns them in
// declaration order. A workspace must name itself and list at least one root, and no two may
// share a name (the name is the tab label and the membership key). An empty configuration is
// valid and yields no declared workspaces; the feature still forms automatic parent-derived
// workspaces when enabled. Roots are used verbatim here; expandPaths has already resolved a
// leading "~".
func (c Config) ResolveWorkspaces() ([]Workspace, error) {
	seen := make(map[string]bool, len(c.WorkspaceDefs))
	for _, w := range c.WorkspaceDefs {
		if strings.TrimSpace(w.Name) == "" {
			return nil, fmt.Errorf("invalid [[workspace]]: every workspace needs a non-empty name")
		}
		if seen[w.Name] {
			return nil, fmt.Errorf("invalid [[workspace]] %q: duplicate workspace name", w.Name)
		}
		seen[w.Name] = true
		if len(w.Roots) == 0 {
			return nil, fmt.Errorf("invalid [[workspace]] %q: needs at least one root", w.Name)
		}
	}
	return c.WorkspaceDefs, nil
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
		Dir:        Dir{UseZoxide: true},
		Agents:     Agents{Notify: Notify{NeedsAttention: true, Finished: true}},
		Workspaces: WorkspaceSettings{Enabled: true},
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

// LoadFrom reads the config from a specific path. A missing file yields defaults; a
// malformed file, or one containing a key birdseye does not recognize (a typo, a retired
// option, an unknown candidate type), yields an error naming the file and the offending
// keys. Unrecognized keys are rejected rather than silently ignored so a fat-fingered key
// never quietly no-ops - run `be config --check` to see every problem at once.
func LoadFrom(path string) (Config, error) {
	cfg := Default()
	if err := mergeStrict(&cfg, path); err != nil {
		return Default(), err
	}
	cfg.expandPaths()
	return cfg, nil
}

// WithRuntimeDefaults returns a copy of c with the built-in runtime defaults
// materialized into the fields that resolve lazily at use-time (the [agents]
// refresh/command/split). It is what `be config` prints so the output reflects
// the values actually in effect, not blanks. Only unset fields are filled - a
// value you set is left verbatim - and the defaults are read from the same
// resolvers the app uses, so this can't drift from them. The original is
// unmodified.
func (c Config) WithRuntimeDefaults() Config {
	d := c.clone()
	if d.Agents.Refresh == "" {
		if iv, err := (Agents{}).RefreshInterval(); err == nil {
			d.Agents.Refresh = iv.String()
		}
	}
	if d.Agents.Command == "" {
		d.Agents.Command = (Agents{}).AgentCommand()
	}
	if d.Agents.Split == 0 {
		d.Agents.Split = DefaultSplit
	}
	return d
}

// CheckFile decodes the config at path and returns the keys it contains that birdseye does
// not recognize - typos, retired options, keys in the wrong section, or an unknown candidate
// type under order/[providers]/[labels]/[icons]. It is the structural half of `be config
// --check`; value-range checks (a bad split, an unknown keybinding) live at the use sites
// that already validate them. A missing file yields no keys and no error; a malformed file is
// reported as an error naming the file. The returned keys are sorted. Unlike the strict
// loaders this only reports - it never errors on a merely-unrecognized key - so --check can
// list every problem in one pass.
func CheckFile(path string) ([]string, error) {
	var cfg Config
	return mergeFile(&cfg, path)
}

// EffectiveLenient builds the merged config from base defaults plus the files at paths, in
// order (low to high precedence), tolerating unrecognized keys. It backs `be config --check`'s
// value pass, which validates keybindings, accent, split, and refresh on a config that may
// also have structural problems (those are reported separately via CheckFile). Production code
// uses the strict Load/Overlay instead. A malformed or unreadable file is an error.
func EffectiveLenient(paths ...string) (Config, error) {
	cfg := Default()
	for _, p := range paths {
		if _, err := mergeFile(&cfg, p); err != nil {
			return Default(), err
		}
	}
	cfg.expandPaths()
	return cfg, nil
}

// RepoConfigName is the repository-local config file, found at <repo-root>/.birdseye/config.toml.
const RepoConfigName = ".birdseye/config.toml"

// Overlay returns base with a repository's local .birdseye/config.toml layered on top, so a
// project can override any user-level setting where it chooses to and inherit the rest. It is
// the effective configuration for repository-scoped operations (worktree add, agent spawn).
// Merge semantics follow the TOML decoder: a scalar set by the repo overrides, a scalar it
// omits is inherited, maps merge key-by-key, and lists are replaced wholesale. base is left
// unmodified (it is deep-copied first); a missing repo config yields base unchanged, and a
// malformed one - or one with a key birdseye does not recognize - is an error.
func Overlay(base Config, repoRoot string) (Config, error) {
	eff := base.clone()
	if err := mergeStrict(&eff, filepath.Join(repoRoot, RepoConfigName)); err != nil {
		return base, err
	}
	eff.expandPaths()
	return eff, nil
}

// mergeStrict layers the file at path onto cfg and rejects any key birdseye does not
// recognize, so a typo or retired option fails loudly at load time instead of silently
// no-opping. It is the production loader's merge step; CheckFile/EffectiveLenient use the
// tolerant mergeFile directly.
func mergeStrict(cfg *Config, path string) error {
	unknown, err := mergeFile(cfg, path)
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		return unrecognizedConfigErr(path, unknown)
	}
	return nil
}

// mergeFile decodes the TOML file at path onto cfg (layering its keys over whatever cfg
// already holds, so callers can stack defaults, the user config, and a repo config) and
// returns the keys it did not recognize. A missing file is a no-op with no keys; a malformed
// or unreadable file is an error naming it. Whether unrecognized keys are fatal is the
// caller's choice - mergeStrict rejects them, the --check helpers report them.
func mergeFile(cfg *Config, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	md, err := toml.Decode(string(data), cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return unknownKeys(md, *cfg), nil
}

// knownTypes is the set of candidate types birdseye recognizes, taken from the default
// display order (the canonical list of built-in providers). Config keyed by type - order,
// [providers], [labels], [icons] - is validated against it, so a misspelled type such as
// "reepo" is rejected rather than silently leaving the real provider untouched.
var knownTypes = func() map[string]bool {
	m := make(map[string]bool)
	for _, t := range Default().Order {
		m[t] = true
	}
	return m
}()

// unknownKeys returns every key in a decoded config that birdseye does not recognize: TOML
// keys outside the schema (md.Undecoded - typos and retired fields), plus type-keyed entries
// naming an unknown candidate type (which decode into their maps, so the decoder cannot flag
// them). The result is sorted for a stable report.
func unknownKeys(md toml.MetaData, cfg Config) []string {
	var unknown []string
	for _, k := range md.Undecoded() {
		unknown = append(unknown, k.String())
	}
	for _, t := range cfg.Order {
		if !knownTypes[t] {
			unknown = append(unknown, fmt.Sprintf("order type %q", t))
		}
	}
	for t := range cfg.Providers {
		if !knownTypes[t] {
			unknown = append(unknown, "providers."+t)
		}
	}
	for t := range cfg.Labels {
		if !knownTypes[t] {
			unknown = append(unknown, "labels."+t)
		}
	}
	for t := range cfg.Icons {
		if !knownTypes[t] {
			unknown = append(unknown, "icons."+t)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// unrecognizedConfigErr builds the load-time error for a config with unrecognized keys,
// naming the file and listing every offender, with the [worktree] migration hint when a
// retired worktree-provider key is among them.
func unrecognizedConfigErr(path string, unknown []string) error {
	noun := "keys"
	if len(unknown) == 1 {
		noun = "key"
	}
	msg := fmt.Sprintf("invalid config %s: unrecognized %s: %s", path, noun, strings.Join(unknown, ", "))
	for _, k := range unknown {
		if strings.HasPrefix(k, "worktree.") {
			msg += "; the [worktree] provider roots moved to [repo] (use roots = [\"~/code\"])"
			break
		}
	}
	return errors.New(msg)
}

// clone returns a deep copy of c whose maps and slices are independent of the original, so
// Overlay can decode a repo config onto it without mutating the shared base config.
func (c Config) clone() Config {
	d := c
	d.Order = append([]string(nil), c.Order...)
	d.Dir.Roots = append([]string(nil), c.Dir.Roots...)
	d.Repo.Roots = append([]string(nil), c.Repo.Roots...)
	if c.WorkspaceDefs != nil {
		d.WorkspaceDefs = make([]Workspace, len(c.WorkspaceDefs))
		for i, w := range c.WorkspaceDefs {
			d.WorkspaceDefs[i] = Workspace{Name: w.Name, Roots: append([]string(nil), w.Roots...)}
		}
	}
	d.Providers = cloneMap(c.Providers)
	d.Labels = cloneMap(c.Labels)
	d.Icons = cloneMap(c.Icons)
	if c.Agents.Keys != nil {
		keys := make(map[string][]string, len(c.Agents.Keys))
		for k, v := range c.Agents.Keys {
			keys[k] = append([]string(nil), v...)
		}
		d.Agents.Keys = keys
	}
	return d
}

// cloneMap returns a shallow copy of m, or nil when m is nil.
func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	d := make(map[K]V, len(m))
	for k, v := range m {
		d[k] = v
	}
	return d
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
	for i := range c.WorkspaceDefs {
		for j, p := range c.WorkspaceDefs[i].Roots {
			c.WorkspaceDefs[i].Roots[j] = expandTilde(p)
		}
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
