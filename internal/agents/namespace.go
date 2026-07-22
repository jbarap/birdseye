package agents

import (
	"path/filepath"
	"strings"
)

// Namespace is one configured dash namespace (a "workspace"): a name plus the filesystem roots
// whose repositories belong to it. It is the agents-package view of a config `[[workspace]]`,
// mapped in cli so this package does not import config. Membership is path-derived and recomputed
// each refresh - see assignRepoTabs - so nothing about a namespace is persisted per agent.
type Namespace struct {
	Name  string
	Roots []string
}

// autoPrefix tags the identity key of an automatic (parent-derived) workspace so it can never
// collide with a configured namespace name. It is an internal key, never displayed; the tab shows
// the parent directory's base name instead.
const autoPrefix = "\x00auto:"

// repoTab is a repository's resolved workspace: the tab key it filters under (a configured
// namespace name, or an autoPrefix-tagged parent path), the tab's display label, and whether it
// is an automatic parent-derived workspace rather than a configured one.
type repoTab struct {
	key   string
	label string
	auto  bool
}

// representativePaths picks one filesystem path per recognized repository (by git-common-dir) to
// judge its workspace by: its primary worktree when present (the repo's stable home), else the
// first row seen. Incidental rows (no git-common-dir) are skipped - they belong to no repository.
func representativePaths(rows []Row) map[string]string {
	repPath := map[string]string{}
	primary := map[string]bool{}
	for _, r := range rows {
		if r.GitDir == "" || r.Dir == "" {
			continue
		}
		switch {
		case r.IsPrimary:
			repPath[r.GitDir] = r.Dir
			primary[r.GitDir] = true
		case !primary[r.GitDir]:
			if _, ok := repPath[r.GitDir]; !ok {
				repPath[r.GitDir] = r.Dir
			}
		}
	}
	return repPath
}

// namespaceMembership maps each recognized repository (by git-common-dir) to the configured
// namespace it belongs to, or "" when it matches none. It exists for reasoning about configured
// membership alone; the dashboard uses assignRepoTabs, which also forms the parent-derived
// fallback for unmatched repositories.
func namespaceMembership(rows []Row, nss []Namespace) map[string]string {
	rep := representativePaths(rows)
	out := make(map[string]string, len(rep))
	for gd, p := range rep {
		out[gd] = matchNamespace(p, nss)
	}
	return out
}

// rowTabKey is the identity a row is assigned a tab under: a repository's git-common-dir, a
// non-git directory session's own id (keyed by its path via assignRepoTabs), or "" for a row
// that belongs to no workspace (an incidental agent), which therefore appears only under All.
func rowTabKey(r Row) string {
	if r.Kind == RowDir {
		return r.SessionID
	}
	return r.GitDir
}

// assignRepoTabs resolves every recognized repository and every open non-git directory session to
// the tab it lives under. A path under a configured namespace's root takes that namespace (longest
// match wins); an unmatched one falls back to an automatic workspace keyed by its parent directory
// and labelled with that directory's base name, so unconfigured locations are grouped rather than
// confined to All. A repository is judged by its representative worktree path, a directory session
// by its own path - so a non-git directory opened under a workspace root files under that workspace
// alongside its repositories. Returns nil when the feature is disabled (inert), so no tabs form;
// when enabled it always assigns tabs, forming automatic workspaces even with no namespace configured.
func assignRepoTabs(rows []Row, nss []Namespace, enabled bool) map[string]repoTab {
	if !enabled {
		return nil
	}
	out := map[string]repoTab{}
	assign := func(key, path string) {
		if name := matchNamespace(path, nss); name != "" {
			out[key] = repoTab{key: name, label: name}
			return
		}
		parent := filepath.Dir(filepath.Clean(path))
		out[key] = repoTab{key: autoPrefix + parent, label: filepath.Base(parent), auto: true}
	}
	for gd, p := range representativePaths(rows) {
		assign(gd, p)
	}
	for _, r := range rows {
		if r.Kind == RowDir && r.Dir != "" {
			assign(rowTabKey(r), r.Dir)
		}
	}
	return out
}

// matchNamespace returns the name of the configured namespace whose root most specifically
// contains path, or "" when no root does. "Most specific" is the longest matching root path, so a
// repository under both ~/work and ~/work/clients lands in the clients namespace regardless of
// config order.
func matchNamespace(path string, nss []Namespace) string {
	best := ""
	bestLen := -1
	cp := filepath.Clean(path)
	for _, ns := range nss {
		for _, root := range ns.Roots {
			cr := filepath.Clean(root)
			if pathUnder(cp, cr) && len(cr) > bestLen {
				best, bestLen = ns.Name, len(cr)
			}
		}
	}
	return best
}

// pathUnder reports whether path is root or lives beneath it, comparing whole path segments so
// "/home/work" is not treated as under "/home/wor".
func pathUnder(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
