// Package sessname constructs the tmux session names be uses for its directory-rooted
// sessions. It is the single source of truth for that naming so every consumer - the
// picker providers, the fleet spawner, and the dash reconciler - derives the same name
// for the same input and lands in one session rather than several. The functions are pure
// and deterministic: a name is a function of its inputs alone, never of the live session
// or candidate set.
package sessname

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// Sanitize folds a directory or repository base name into a valid tmux session-name
// fragment. tmux forbids '.' and ':' in session names, and a space would split arguments,
// so each is replaced with '_'. It is idempotent - sanitizing an already-sanitized value
// changes nothing.
func Sanitize(base string) string {
	r := strings.NewReplacer(".", "_", ":", "_", " ", "_")
	return r.Replace(base)
}

// For builds a session name from a human base and the identity that disambiguates it:
// Sanitize(base) plus a short hash of identity. The hash does double duty - it keeps two
// locations that share a base name distinct (the folding in Sanitize is lossy, so bare
// bases collide), and it marks the session as be-derived without a noisy prefix. Callers
// pass the identity to hash explicitly because it is not always the base's own path: a
// repository hashes its git-common-dir (stable across all its worktrees), a plain directory
// hashes its own cleaned path.
func For(base, identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return Sanitize(base) + "-" + hex.EncodeToString(sum[:])[:6]
}

// Home is the deterministic session name for a repository's agent home - its write domain.
// It is a pure function of the repository's identity (its git-common-dir): the repository
// basename is the friendly part and a short hash of the git-common-dir is appended. The hash
// disambiguates two repositories that share a basename and marks the session as be-derived.
// Determinism is load-bearing - a repository's home always resolves to the same name
// regardless of what else is running - so the disambiguator is derived from the git-common-dir
// alone, never from the live session set.
func Home(gitCommonDir string) string {
	clean := filepath.Clean(gitCommonDir)
	return For(filepath.Base(filepath.Dir(clean)), clean)
}
