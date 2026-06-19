package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hardcodedColor matches a #rrggbb literal, a lipgloss.Color("...") string-literal
// argument, or a color SGR escape (foreground/background truecolor/256, or the basic
// 30-37/40-47/90-97/100-107 codes). It deliberately does NOT match non-color escapes
// like reset (\x1b[0m), bold (\x1b[1m), or dim (\x1b[2m), which carry no color.
var hardcodedColor = regexp.MustCompile(
	`#[0-9a-fA-F]{6}\b` +
		`|lipgloss\.Color\("` +
		`|\\x1b\[(?:38|48);` +
		`|\\x1b\[(?:3[0-7]|4[0-7]|9[0-7]|10[0-7])m`,
)

// TestNoHardcodedDesignTokensOutsideTheme enforces DESIGN.md's single-source rule:
// every color and the selection glyph must come from this package. It scans the
// module's non-test Go sources (this package excepted) and fails on a hardcoded
// color literal or the raw selection glyph. New tokens belong in theme.
func TestNoHardcodedDesignTokensOutsideTheme(t *testing.T) {
	root := moduleRoot(t)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "openspec":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil // tests may carry sample literals (e.g. accent validation cases)
		}
		if strings.Contains(filepath.ToSlash(path), "/internal/theme/") {
			return nil // the source of truth
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		rel, _ := filepath.Rel(root, path)
		if m := hardcodedColor.FindString(src); m != "" {
			t.Errorf("%s: hardcoded color %q — use an internal/theme token (see DESIGN.md)", rel, m)
		}
		if strings.Contains(src, CursorGlyph) {
			t.Errorf("%s: hardcoded selection glyph — use theme.CursorGlyph (see DESIGN.md)", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// moduleRoot ascends from the test's working directory to the directory holding
// go.mod, so the scan covers the whole module regardless of where it runs.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test working directory")
		}
		dir = parent
	}
}
