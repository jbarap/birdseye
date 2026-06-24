package agents

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// workflowFS holds the bird's-eye-authored workflow artifacts (skills/commands that
// compose the `be agents` verbs), shipped *with* the binary so install needs no network
// and the content matches the binary version. They are opinion, not core: the binary,
// the verbs, and the hooks tier all work whether or not these are installed.
//
//go:embed workflows
var workflowFS embed.FS

// workflowRoot is the embedded subtree for an agent type. Only "claude" ships today;
// the type arg is the extension point for others.
func workflowRoot(agentType string) (string, error) {
	switch agentType {
	case "claude":
		return "workflows/claude", nil
	default:
		return "", fmt.Errorf("unknown agent type %q (supported: claude)", agentType)
	}
}

// workflowFile is one shipped artifact: its path relative to the agent config root
// (e.g. skills/birds-eye-orchestrator/SKILL.md) and its embedded content.
type workflowFile struct {
	rel  string
	data []byte
}

// shippedWorkflows reads the embedded artifacts for an agent type. Every artifact lives
// under a bird's-eye-namespaced directory (skills/birds-eye-*), so uninstall can remove
// exactly the tool's files without touching the user's own skills.
func shippedWorkflows(agentType string) ([]workflowFile, error) {
	root, err := workflowRoot(agentType)
	if err != nil {
		return nil, err
	}
	var out []workflowFile
	err = fs.WalkDir(workflowFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := workflowFS.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, workflowFile{rel: filepath.ToSlash(rel), data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// InstallWorkflows writes the embedded workflow artifacts for agentType into dst (the
// agent config root, e.g. ~/.claude). It is idempotent: an artifact already present and
// unchanged is left alone; a user-modified one is backed up to <file>.bak before being
// rewritten, so no edit is lost silently. It reports whether anything changed.
func InstallWorkflows(agentType, dst string) (changed bool, err error) {
	files, err := shippedWorkflows(agentType)
	if err != nil {
		return false, err
	}
	for _, f := range files {
		path := filepath.Join(dst, filepath.FromSlash(f.rel))
		existing, readErr := os.ReadFile(path)
		switch {
		case readErr == nil && bytes.Equal(existing, f.data):
			continue // idempotent: present and unchanged
		case readErr == nil:
			// Present but different — a user edit or an older shipped version. Back it
			// up before overwriting rather than clobber it silently.
			if err := os.WriteFile(path+".bak", existing, 0o644); err != nil {
				return changed, fmt.Errorf("writing backup %s.bak: %w", path, err)
			}
		case !os.IsNotExist(readErr):
			return changed, fmt.Errorf("reading %s: %w", path, readErr)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return changed, err
		}
		if err := os.WriteFile(path, f.data, 0o644); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

// UninstallWorkflows removes only the artifacts bird's-eye ships for agentType from dst,
// then prunes any now-empty namespaced directories, leaving the user's own skills
// intact. It is idempotent and reports whether anything was removed.
func UninstallWorkflows(agentType, dst string) (changed bool, err error) {
	files, err := shippedWorkflows(agentType)
	if err != nil {
		return false, err
	}
	for _, f := range files {
		path := filepath.Join(dst, filepath.FromSlash(f.rel))
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return changed, err
		}
		changed = true
		pruneEmptyDirs(filepath.Dir(path), dst)
	}
	return changed, nil
}

// WorkflowsInstalled reports whether any shipped artifact for agentType is present in
// dst — used to build the uninstall checklist of currently-installed tiers.
func WorkflowsInstalled(agentType, dst string) (bool, error) {
	files, err := shippedWorkflows(agentType)
	if err != nil {
		return false, err
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(f.rel))); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// pruneEmptyDirs removes dir and its now-empty ancestors, stopping before stop (the
// agent config root) so a shared directory like skills/ is never removed while it still
// holds the user's own skills.
func pruneEmptyDirs(dir, stop string) {
	for dir != stop && dir != filepath.Dir(dir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
