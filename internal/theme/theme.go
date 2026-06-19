// Package theme is the single source of truth for bird's-eye's colors. Each
// color carries a 24-bit hex value and a nearest xterm-256 fallback so callers
// can render truecolor when the terminal supports it and degrade otherwise.
//
// The picker renders these as raw ANSI (its output is piped to fzf, where
// lipgloss would strip color); the agents view and CLI hand the hex to lipgloss,
// which performs its own profile-based downgrade.
package theme

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Color is an accent with both a 24-bit and a 256-palette representation.
type Color struct {
	Hex  string // "#rrggbb", used on truecolor terminals
	C256 int    // xterm-256 palette index, used as the fallback
}

// Palette. Accent hues are shared across surfaces; the status colors are used by
// the agents view. Keep these distinct from one another so types stay readable.
var (
	Blue   = Color{"#4ea8ff", 39}  // tmux / sessions
	Coral  = Color{"#ff7a6b", 209} // tmuxp candidate-type color (see Accent for the tool-wide accent)
	Gold   = Color{"#f5c542", 220} // dir
	Green  = Color{"#4ec98a", 114} // worktree / success
	Attach = Color{"#38bdf8", 33}  // "attach" marker
	Create = Color{"#3fcf5f", 40}  // "create" marker
	Gray   = Color{"#8a8a8a", 245} // dim / unlisted
	Border = Color{"#3a3a3a", 240} // borders, info chrome
	Red    = Color{"#ff6b6b", 203} // needs-attention / errors

	Text      = Color{"#e6e6e6", 254} // primary row text (agent name)
	Accent    = Color{"#c792ea", 176} // tool-wide selection + brand accent (cursor, title, picker chrome); configurable
	SessionBg = Color{"#1d2b3f", 235} // agents view: session section-bar background
	SessionFg = Color{"#9cc7ff", 153} // agents view: session section-bar text
	RowHL     = Color{"#2c2c34", 236} // agents view: selected-row highlight background
)

// CursorGlyph is the tool-wide selection pointer: the agents-view cursor and the
// picker's fzf --pointer. A nerd-font arrow (U+F0055).
const CursorGlyph = "\U000f0055" // 󰁕

const reset = "\x1b[0m"

// Truecolor reports whether the terminal advertises 24-bit color via COLORTERM,
// the convention shared by most CLI tools (bat, delta, fzf). tmux that forwards
// RGB sets it to "truecolor".
func Truecolor() bool {
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return true
	}
	return false
}

// FG wraps s in a foreground-color SGR escape: 24-bit when truecolor, else the
// 256-palette index. Emitted raw so it survives a pipe to fzf under --ansi.
func (c Color) FG(s string, truecolor bool) string {
	if truecolor {
		r, g, b := c.rgb()
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", r, g, b, s, reset)
	}
	return fmt.Sprintf("\x1b[38;5;%dm%s%s", c.C256, s, reset)
}

// Spec renders the color as an fzf --color token value: a hex literal on
// truecolor terminals (fzf accepts "#rrggbb"), else the 256 palette index.
func (c Color) Spec(truecolor bool) string {
	if truecolor {
		return c.Hex
	}
	return strconv.Itoa(c.C256)
}

// rgb parses the hex into components. A malformed value yields 0,0,0, which is
// harmless and never happens for the literals above.
func (c Color) rgb() (r, g, b int) {
	v, _ := strconv.ParseUint(strings.TrimPrefix(c.Hex, "#"), 16, 32)
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff
}
