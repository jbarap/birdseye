package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestReferenceMatchesDefaults guards the shipped `be config` reference against
// drift: parsing it must yield exactly Default(). The reference therefore keeps
// its uncommented lines equal to the built-in defaults and leaves every override
// example commented out. If a default changes, update default.toml in lockstep.
func TestReferenceMatchesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(Reference()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("the shipped config reference must load cleanly: %v", err)
	}
	if want := Default(); !reflect.DeepEqual(cfg, want) {
		t.Fatalf("config reference drifted from Default()\n got: %+v\nwant: %+v", cfg, want)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if !cfg.Dir.UseZoxide {
		t.Fatal("expected zoxide on by default")
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
order = ["dir", "tmux"]

[providers]
tmuxp = false

[dir]
use_zoxide = false
roots = ["/home/u/code"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("a valid override config should load: %v", err)
	}
	if len(cfg.Order) != 2 || cfg.Order[0] != "dir" {
		t.Fatalf("order not applied: %v", cfg.Order)
	}
	if cfg.Providers["tmuxp"] != false {
		t.Fatalf("expected tmuxp disabled")
	}
	if cfg.Dir.UseZoxide {
		t.Fatalf("expected zoxide disabled by override")
	}
	if len(cfg.Dir.Roots) != 1 || cfg.Dir.Roots[0] != "/home/u/code" {
		t.Fatalf("roots not applied: %v", cfg.Dir.Roots)
	}
}

func TestLoadExpandsTildePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to expand against")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[tmuxp]
dir = "~/.tmuxp"

[dir]
roots = ["~/code", "~", "/abs/path", "relative/path"]

[repo]
roots = ["~/projects/open_source"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".tmuxp"); cfg.Tmuxp.Dir != want {
		t.Errorf("tmuxp.dir = %q, want %q", cfg.Tmuxp.Dir, want)
	}
	wantDir := []string{filepath.Join(home, "code"), home, "/abs/path", "relative/path"}
	for i, w := range wantDir {
		if cfg.Dir.Roots[i] != w {
			t.Errorf("dir.roots[%d] = %q, want %q", i, cfg.Dir.Roots[i], w)
		}
	}
	if want := filepath.Join(home, "projects/open_source"); cfg.Repo.Roots[0] != want {
		t.Errorf("repo.roots[0] = %q, want %q", cfg.Repo.Roots[0], want)
	}
}

func TestLoadParsesAgentsKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[agents.keys]
down = ["n", "down"]
quit = ["q"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Agents.Keys["down"]
	if len(got) != 2 || got[0] != "n" || got[1] != "down" {
		t.Fatalf("agents.keys.down not parsed: %v", cfg.Agents.Keys)
	}
	if len(cfg.Agents.Keys["quit"]) != 1 || cfg.Agents.Keys["quit"][0] != "q" {
		t.Fatalf("agents.keys.quit not parsed: %v", cfg.Agents.Keys)
	}
}

func TestLoadParsesRepoRoots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[repo]
roots = ["~/code", "/work/repos"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	wantFirst := filepath.Join(home, "code")
	if len(cfg.Repo.Roots) != 2 || cfg.Repo.Roots[0] != wantFirst || cfg.Repo.Roots[1] != "/work/repos" {
		t.Fatalf("repo.roots not parsed/expanded: %v", cfg.Repo.Roots)
	}
}

func TestLoadParsesAndExpandsWorkspaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[[workspace]]
name  = "personal"
roots = ["~/projects"]
[[workspace]]
name  = "work"
roots = ["/srv/work", "~/clients"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	wss, err := cfg.ResolveWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if len(wss) != 2 || wss[0].Name != "personal" || wss[1].Name != "work" {
		t.Fatalf("workspaces not parsed in order: %+v", wss)
	}
	if wss[0].Roots[0] != filepath.Join(home, "projects") {
		t.Fatalf("workspace root tilde not expanded: %v", wss[0].Roots)
	}
	if wss[1].Roots[0] != "/srv/work" || wss[1].Roots[1] != filepath.Join(home, "clients") {
		t.Fatalf("work roots not parsed/expanded: %v", wss[1].Roots)
	}
}

func TestResolveWorkspacesRejectsBadDeclarations(t *testing.T) {
	cases := []struct {
		name string
		wss  []Workspace
	}{
		{"empty name", []Workspace{{Name: "", Roots: []string{"/a"}}}},
		{"no roots", []Workspace{{Name: "work", Roots: nil}}},
		{"duplicate name", []Workspace{{Name: "w", Roots: []string{"/a"}}, {Name: "w", Roots: []string{"/b"}}}},
	}
	for _, c := range cases {
		if _, err := (Config{Workspaces: c.wss}).ResolveWorkspaces(); err == nil {
			t.Errorf("%s: expected an error, got nil", c.name)
		}
	}
}

func TestLoadRejectsLegacyWorktreeSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[worktree]
roots = ["~/code"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected the retired [worktree] section to be rejected")
	}
	if !contains(err.Error(), "repo") {
		t.Fatalf("error should point to the new [repo] section, got %q", err.Error())
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// A typo'd top-level field, a typo'd struct field, and a misspelled candidate type in
	// both a list (order) and a map ([providers]) - all of which used to be silently ignored.
	body := `
oder = ["tmux"]
order = ["tmux", "reepo"]

[dir]
usezoxide = true

[providers]
reepo = false
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected unrecognized keys to be rejected, not silently ignored")
	}
	for _, want := range []string{"oder", "dir.usezoxide", `order type "reepo"`, "providers.reepo"} {
		if !contains(err.Error(), want) {
			t.Errorf("error should name %q, got %q", want, err.Error())
		}
	}
}

func TestLoadMalformedReportsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("this is = not valid = toml ]["), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for malformed config")
	}
	if want := path; !contains(err.Error(), want) {
		t.Fatalf("error should name the file %q, got %q", want, err.Error())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestAgentsRefreshAndCommand(t *testing.T) {
	// Defaults when unset.
	var a Agents
	if d, err := a.RefreshInterval(); err != nil || d.String() != "1s" {
		t.Fatalf("default refresh = (%v,%v), want 1s", d, err)
	}
	if a.AgentCommand() != "claude" {
		t.Fatalf("default command = %q, want claude", a.AgentCommand())
	}
	// Honored when set.
	a = Agents{Refresh: "2s", Command: "aider"}
	if d, err := a.RefreshInterval(); err != nil || d.String() != "2s" {
		t.Fatalf("refresh = (%v,%v), want 2s", d, err)
	}
	if a.AgentCommand() != "aider" {
		t.Fatalf("command = %q, want aider", a.AgentCommand())
	}
	// Malformed and non-positive are rejected.
	for _, bad := range []string{"nope", "0s", "-1s"} {
		if _, err := (Agents{Refresh: bad}).RefreshInterval(); err == nil {
			t.Errorf("refresh %q should be rejected", bad)
		}
	}
}

func TestAgentsSplitRatio(t *testing.T) {
	// Unset uses the built-in default.
	if r, err := (Agents{}).SplitRatio(); err != nil || r != DefaultSplit {
		t.Fatalf("default split = (%v,%v), want %v", r, err, DefaultSplit)
	}
	// An in-band value is honored verbatim.
	if r, err := (Agents{Split: 0.6}).SplitRatio(); err != nil || r != 0.6 {
		t.Fatalf("split 0.6 = (%v,%v), want 0.6", r, err)
	}
	// Out-of-band-but-valid fractions clamp to the sane range.
	if r, _ := (Agents{Split: 0.1}).SplitRatio(); r != 0.3 {
		t.Fatalf("split 0.1 should clamp up to 0.3, got %v", r)
	}
	if r, _ := (Agents{Split: 0.95}).SplitRatio(); r != 0.8 {
		t.Fatalf("split 0.95 should clamp down to 0.8, got %v", r)
	}
	// Values outside (0,1) are a configuration error.
	for _, bad := range []float64{-0.5, 1, 1.5} {
		if _, err := (Agents{Split: bad}).SplitRatio(); err == nil {
			t.Errorf("split %v should be rejected", bad)
		}
	}
}

func TestWithRuntimeDefaultsFillsUnsetOnly(t *testing.T) {
	// An unset config gets the built-in runtime values materialized.
	got := Default().WithRuntimeDefaults()
	if got.Agents.Refresh != "1s" {
		t.Errorf("refresh = %q, want 1s", got.Agents.Refresh)
	}
	if got.Agents.Command != "claude" {
		t.Errorf("command = %q, want claude", got.Agents.Command)
	}
	if got.Agents.Split != DefaultSplit {
		t.Errorf("split = %v, want %v", got.Agents.Split, DefaultSplit)
	}

	// Values the user set are left verbatim (not reformatted, not defaulted).
	base := Default()
	base.Agents.Refresh = "250ms"
	base.Agents.Command = "aider"
	base.Agents.Split = 0.6
	got = base.WithRuntimeDefaults()
	if got.Agents.Refresh != "250ms" || got.Agents.Command != "aider" || got.Agents.Split != 0.6 {
		t.Errorf("set values must be preserved, got %+v", got.Agents)
	}

	// The receiver is not mutated.
	if base.Agents.Refresh != "250ms" {
		t.Error("WithRuntimeDefaults must not mutate the receiver")
	}
	orig := Default()
	_ = orig.WithRuntimeDefaults()
	if orig.Agents.Refresh != "" || orig.Agents.Command != "" || orig.Agents.Split != 0 {
		t.Errorf("WithRuntimeDefaults mutated the receiver: %+v", orig.Agents)
	}
}

func TestCheckFile(t *testing.T) {
	dir := t.TempDir()

	// A missing file is not an error and has no unknown keys.
	if keys, err := CheckFile(filepath.Join(dir, "absent.toml")); err != nil || keys != nil {
		t.Fatalf("missing file: keys=%v err=%v, want nil,nil", keys, err)
	}

	// A clean config has no unknown keys.
	clean := filepath.Join(dir, "clean.toml")
	if err := os.WriteFile(clean, []byte("order = [\"tmux\"]\n[dir]\nuse_zoxide = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if keys, err := CheckFile(clean); err != nil || len(keys) != 0 {
		t.Fatalf("clean config: keys=%v err=%v, want none", keys, err)
	}

	// Typos, retired keys, and misspelled candidate types (in a list and a map) are all
	// reported, sorted.
	bad := filepath.Join(dir, "bad.toml")
	body := "oder = [\"tmux\"]\norder = [\"tmux\", \"reepo\"]\n[dir]\nusezoxide = true\n[providers]\nreepo = false\n[worktree]\nroots = [\"~/c\"]\n"
	if err := os.WriteFile(bad, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	keys, err := CheckFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dir.usezoxide", "oder", `order type "reepo"`, "providers.reepo", "worktree.roots"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("unknown keys = %v, want %v (sorted)", keys, want)
	}

	// Malformed TOML is an error naming the file.
	malformed := filepath.Join(dir, "malformed.toml")
	if err := os.WriteFile(malformed, []byte("this = = bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckFile(malformed); err == nil || !contains(err.Error(), malformed) {
		t.Fatalf("malformed: err=%v, want error naming the file", err)
	}
}

func writeRepoConfig(t *testing.T, repoRoot, body string) {
	t.Helper()
	dir := filepath.Join(repoRoot, ".birdseye")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayNoRepoConfigInheritsBase(t *testing.T) {
	base := Default()
	base.Worktree.Setup = "make bootstrap"
	base.Agents.Command = "claude"
	got, err := Overlay(base, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.Worktree.Setup != "make bootstrap" || got.Agents.Command != "claude" {
		t.Fatalf("with no repo config the base should be inherited verbatim, got %+v", got)
	}
}

func TestOverlayRepoOverridesAndInherits(t *testing.T) {
	base := Default()
	base.Worktree.Setup = "make bootstrap"
	base.Agents.Command = "claude"
	base.Agents.Refresh = "2s"
	repo := t.TempDir()
	writeRepoConfig(t, repo, "[worktree]\nsetup = \"./scripts/setup.sh\"\n[agents]\ncommand = \"codex\"\n")
	got, err := Overlay(base, repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Worktree.Setup != "./scripts/setup.sh" {
		t.Errorf("repo setup should override, got %q", got.Worktree.Setup)
	}
	if got.Agents.Command != "codex" {
		t.Errorf("repo agent command should override, got %q", got.Agents.Command)
	}
	if got.Agents.Refresh != "2s" {
		t.Errorf("a key the repo omits should be inherited, got %q", got.Agents.Refresh)
	}
}

func TestOverlayDoesNotMutateBase(t *testing.T) {
	base := Default()
	base.Worktree.Setup = "make bootstrap"
	repo := t.TempDir()
	writeRepoConfig(t, repo, "[worktree]\nsetup = \"override\"\n[labels]\ntmux = \"changed\"\n")
	if _, err := Overlay(base, repo); err != nil {
		t.Fatal(err)
	}
	if base.Worktree.Setup != "make bootstrap" {
		t.Errorf("Overlay must not mutate the base scalar, got %q", base.Worktree.Setup)
	}
	if base.Labels["tmux"] != "session" {
		t.Errorf("Overlay must not mutate the base map, got %q", base.Labels["tmux"])
	}
}

func TestOverlayMergesMaps(t *testing.T) {
	base := Default()
	repo := t.TempDir()
	writeRepoConfig(t, repo, "[labels]\ntmux = \"box\"\ndir = \"folder\"\n")
	got, err := Overlay(base, repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Labels["tmux"] != "box" || got.Labels["dir"] != "folder" {
		t.Errorf("repo should override the labels it sets, got %q/%q", got.Labels["tmux"], got.Labels["dir"])
	}
	if got.Labels["tmuxp"] != "template" {
		t.Errorf("a label the repo omits should be inherited, got %q", got.Labels["tmuxp"])
	}
}

func TestOverlayMalformedRepoConfigRejected(t *testing.T) {
	repo := t.TempDir()
	writeRepoConfig(t, repo, "setup = \n")
	if _, err := Overlay(Default(), repo); err == nil {
		t.Fatal("expected a malformed repo-local config to be rejected")
	}
}

func TestLoadAcceptsWorktreeSetupKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[worktree]\nsetup = \"make setup\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("[worktree] setup should be accepted, got %v", err)
	}
	if cfg.Worktree.Setup != "make setup" {
		t.Fatalf("expected global setup default, got %q", cfg.Worktree.Setup)
	}
}
