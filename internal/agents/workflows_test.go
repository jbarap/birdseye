package agents

import (
	"os"
	"path/filepath"
	"testing"
)

// shippedRels is the set of artifact paths birdseye ships for claude, relative to the
// config root — used to assert install/uninstall touch exactly these.
func shippedRels(t *testing.T) []string {
	t.Helper()
	files, err := shippedWorkflows("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one shipped workflow artifact")
	}
	rels := make([]string, len(files))
	for i, f := range files {
		rels[i] = f.rel
	}
	return rels
}

// TestWorkflowsNamespacedAndComplete checks the shipped artifacts live under the
// birdseye namespace and include the orchestrator, QA, and PR skills.
func TestWorkflowsNamespacedAndComplete(t *testing.T) {
	rels := shippedRels(t)
	want := map[string]bool{"orchestrator": false, "qa": false, "pr": false}
	for _, rel := range rels {
		if filepath.Dir(filepath.Dir(rel)) != "skills" {
			t.Fatalf("artifact %q is not under skills/<namespaced-dir>/", rel)
		}
		dir := filepath.Base(filepath.Dir(rel))
		if len(dir) < len("birdseye-") || dir[:len("birdseye-")] != "birdseye-" {
			t.Fatalf("artifact dir %q is not birdseye-namespaced", dir)
		}
		for k := range want {
			if dir == "birdseye-"+k {
				want[k] = true
			}
		}
	}
	for k, ok := range want {
		if !ok {
			t.Fatalf("shipped workflows missing the %q skill", k)
		}
	}
}

// TestInstallWorkflowsIdempotent installs into a temp config root, asserts the files land
// where expected, and that a second install reports no change.
func TestInstallWorkflowsIdempotent(t *testing.T) {
	dst := t.TempDir()
	changed, err := InstallWorkflows("claude", dst)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first install should report a change")
	}
	for _, rel := range shippedRels(t) {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected artifact installed at %q: %v", rel, err)
		}
	}
	changed, err = InstallWorkflows("claude", dst)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("re-installing unchanged artifacts should report no change")
	}
}

// TestInstallWorkflowsBacksUpModified verifies a user-modified artifact is backed up
// rather than silently overwritten, and is then refreshed to the shipped content.
func TestInstallWorkflowsBacksUpModified(t *testing.T) {
	dst := t.TempDir()
	if _, err := InstallWorkflows("claude", dst); err != nil {
		t.Fatal(err)
	}
	rel := shippedRels(t)[0]
	path := filepath.Join(dst, filepath.FromSlash(rel))
	if err := os.WriteFile(path, []byte("user edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallWorkflows("claude", dst)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("re-installing over a modified artifact should report a change")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("expected a backup of the modified artifact: %v", err)
	}
	if string(bak) != "user edit" {
		t.Fatalf("backup should hold the user's edit, got %q", bak)
	}
}

// TestUninstallWorkflowsRemovesOnlyOurs checks uninstall removes the birdseye artifacts
// and prunes their namespaced dirs, while leaving a user's own skill — and the shared
// skills/ directory — intact.
func TestUninstallWorkflowsRemovesOnlyOurs(t *testing.T) {
	dst := t.TempDir()
	if _, err := InstallWorkflows("claude", dst); err != nil {
		t.Fatal(err)
	}
	// A user skill that must survive uninstall.
	userSkill := filepath.Join(dst, "skills", "my-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(userSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userSkill, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := UninstallWorkflows("claude", dst)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("uninstall should report a change when artifacts were present")
	}
	for _, rel := range shippedRels(t) {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("artifact %q should be removed, stat err=%v", rel, err)
		}
		// The namespaced dir should be pruned too.
		if _, err := os.Stat(filepath.Dir(filepath.Join(dst, filepath.FromSlash(rel)))); !os.IsNotExist(err) {
			t.Fatalf("namespaced dir for %q should be pruned", rel)
		}
	}
	if _, err := os.Stat(userSkill); err != nil {
		t.Fatalf("the user's own skill must survive uninstall: %v", err)
	}

	// Idempotent: a second uninstall with nothing left reports no change.
	changed, err = UninstallWorkflows("claude", dst)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("uninstall with nothing to remove should report no change")
	}
}

// TestWorkflowsInstalledReflectsState checks the installed-probe used by the uninstall
// checklist tracks install/uninstall.
func TestWorkflowsInstalledReflectsState(t *testing.T) {
	dst := t.TempDir()
	if ok, err := WorkflowsInstalled("claude", dst); err != nil || ok {
		t.Fatalf("nothing installed yet: ok=%v err=%v", ok, err)
	}
	if _, err := InstallWorkflows("claude", dst); err != nil {
		t.Fatal(err)
	}
	if ok, err := WorkflowsInstalled("claude", dst); err != nil || !ok {
		t.Fatalf("should report installed: ok=%v err=%v", ok, err)
	}
	if _, err := UninstallWorkflows("claude", dst); err != nil {
		t.Fatal(err)
	}
	if ok, err := WorkflowsInstalled("claude", dst); err != nil || ok {
		t.Fatalf("should report not installed after uninstall: ok=%v err=%v", ok, err)
	}
}

// TestWorkflowsTierIndependentOfHooks confirms installing the workflows tier writes
// nothing into the hooks settings file — the tiers are independent.
func TestWorkflowsTierIndependentOfHooks(t *testing.T) {
	dst := t.TempDir()
	settings := filepath.Join(dst, "settings.json")
	if _, err := InstallWorkflows("claude", dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatalf("installing workflows must not create the hooks settings file, stat err=%v", err)
	}
	if ok, err := HooksInstalled(settings); err != nil || ok {
		t.Fatalf("hooks must remain uninstalled: ok=%v err=%v", ok, err)
	}
}

// TestUnknownAgentTypeRejected checks an unsupported type is refused by the artifact
// resolver (the CLI guards too, but the core must not silently no-op).
func TestUnknownAgentTypeRejected(t *testing.T) {
	if _, err := shippedWorkflows("codex"); err == nil {
		t.Fatal("an unsupported agent type should be rejected")
	}
}
