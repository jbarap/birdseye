// Package picker presents session candidates through fzf and returns the
// selection. fzf is a hard requirement: when it is absent the picker fails with
// an actionable message rather than falling back.
package picker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/jbarap/birds-eye/internal/config"
	"github.com/jbarap/birds-eye/internal/provider"
)

var (
	// ErrFzfMissing means the fzf binary is not on PATH.
	ErrFzfMissing = errors.New("fzf is required but was not found on PATH; install it from https://github.com/junegunn/fzf")
	// ErrNoCandidates means there was nothing to choose from.
	ErrNoCandidates = errors.New("no session candidates available")
	// ErrNotInteractive means there is no controlling terminal for fzf.
	ErrNotInteractive = errors.New("be requires an interactive terminal")
	// ErrCancelled means the user dismissed the picker without choosing.
	ErrCancelled = errors.New("selection cancelled")
)

// Pick orders candidates by the configured type order, presents them through
// fzf, and returns the chosen candidate. The returned error is one of the
// sentinels above for the well-known empty/missing/cancelled cases.
func Pick(cands []provider.Candidate, cfg config.Config) (*provider.Candidate, error) {
	if len(cands) == 0 {
		return nil, ErrNoCandidates
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return nil, ErrFzfMissing
	}
	if !hasTTY() {
		return nil, ErrNotInteractive
	}

	t := newTheme()
	ordered := orderByType(cands, cfg.Order)
	var input strings.Builder
	for i, c := range ordered {
		// "<index>\t<display>"; index is hidden from the view via --with-nth.
		fmt.Fprintf(&input, "%d\t%s\n", i, display(c, cfg, t))
	}

	idx, err := runFzf(input.String(), t)
	if err != nil {
		return nil, err
	}
	if idx < 0 || idx >= len(ordered) {
		return nil, ErrCancelled
	}
	return &ordered[idx], nil
}

// Color escapes are emitted raw so they survive the pipe to fzf (which renders
// them under --ansi). lipgloss would strip them when stdout is not a TTY, so we
// format escapes directly.
const ansiReset = "\x1b[0m"

func bold(s string) string { return "\x1b[1m" + s + ansiReset }
func dim(s string) string  { return "\x1b[2m" + s + ansiReset }

// color carries both representations of an accent so the picker can render
// 24-bit when the terminal supports it and degrade to the nearest xterm-256
// palette entry otherwise.
type color struct {
	hex  string // "#rrggbb", used on truecolor terminals
	c256 int    // xterm-256 palette index, used as the fallback
}

var (
	cTmux     = color{"#4ea8ff", 39}  // blue
	cTmuxp    = color{"#ff7a6b", 209} // coral
	cDir      = color{"#f5c542", 220} // gold
	cWorktree = color{"#4ec98a", 114} // green
	cAttach   = color{"#38bdf8", 33}  // cyan-blue marker
	cCreate   = color{"#3fcf5f", 40}  // green marker
	cGray     = color{"#8a8a8a", 245} // dim/unlisted
	cBorder   = color{"#3a3a3a", 240} // border/info chrome
)

func colorOf(typ string) color {
	switch typ {
	case "tmux":
		return cTmux
	case "tmuxp":
		return cTmuxp
	case "dir":
		return cDir
	case "worktree":
		return cWorktree
	default:
		return cGray
	}
}

// theme decides, once per run, whether to emit 24-bit or 256-color escapes.
type theme struct{ truecolor bool }

// newTheme detects 24-bit support via COLORTERM, the convention shared by most
// CLI tools (bat, delta, fzf docs). tmux that forwards RGB sets it to
// "truecolor"; terminals without it fall back to the 256-color palette.
func newTheme() theme {
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return theme{truecolor: true}
	}
	return theme{truecolor: false}
}

// fg wraps s in a foreground-color escape, 24-bit or 256 per the theme.
func (t theme) fg(c color, s string) string {
	if t.truecolor {
		r, g, b := hexRGB(c.hex)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", r, g, b, s, ansiReset)
	}
	return fmt.Sprintf("\x1b[38;5;%dm%s%s", c.c256, s, ansiReset)
}

// spec renders a color as an fzf --color token value: a hex literal on
// truecolor terminals (fzf accepts "#rrggbb"), else the 256 palette index.
func (t theme) spec(c color) string {
	if t.truecolor {
		return c.hex
	}
	return strconv.Itoa(c.c256)
}

// hexRGB parses "#rrggbb" into its components. A malformed value yields 0,0,0,
// which is harmless (renders black) but never happens for our literals.
func hexRGB(h string) (r, g, b int) {
	v, _ := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff
}

func display(c provider.Candidate, cfg config.Config, t theme) string {
	// Kind marker: a green "+" to create, a blue "→" to attach to an existing one.
	mark := t.fg(cAttach, "→")
	if c.Kind == provider.KindCreate {
		mark = t.fg(cCreate, "+")
	}

	var b strings.Builder
	b.WriteString(mark)
	b.WriteByte(' ')
	if icon := cfg.Icon(c.Type); icon != "" {
		b.WriteString(t.fg(colorOf(c.Type), icon))
		b.WriteByte(' ')
	}
	b.WriteString(bold(c.Label))
	b.WriteString("  ")
	b.WriteString(dim("(" + cfg.Label(c.Type) + ")"))
	return b.String()
}

// orderByType returns candidates grouped by the configured type order; types
// not listed sort after listed ones, and original order is preserved within a
// type (stable).
func orderByType(cands []provider.Candidate, order []string) []provider.Candidate {
	rank := map[string]int{}
	for i, t := range order {
		rank[t] = i
	}
	rankOf := func(t string) int {
		if r, ok := rank[t]; ok {
			return r
		}
		return len(order)
	}
	out := make([]provider.Candidate, len(cands))
	copy(out, cands)
	sort.SliceStable(out, func(i, j int) bool {
		return rankOf(out[i].Type) < rankOf(out[j].Type)
	})
	return out
}

// fzfColorSpec builds fzf's --color argument from the theme, so fzf's own
// chrome (match highlight, pointer, prompt, border) matches the candidate
// accents and uses 24-bit or 256 to suit the terminal.
func fzfColorSpec(t theme) string {
	return strings.Join([]string{
		"hl:" + t.spec(cTmux),
		"hl+:" + t.spec(cTmuxp),
		"pointer:" + t.spec(cTmuxp),
		"prompt:" + t.spec(cTmuxp),
		"marker:" + t.spec(cTmuxp),
		"border:" + t.spec(cBorder),
		"info:" + t.spec(cBorder),
		"gutter:-1",
	}, ",")
}

// runFzf pipes input to fzf and returns the selected line's index field, or -1
// when cancelled.
func runFzf(input string, t theme) (int, error) {
	cmd := exec.Command("fzf",
		"--ansi",
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--prompt", "be ❯ ",
		"--pointer", "▌",
		"--no-multi",
		"--height", "100%",
		"--layout", "reverse",
		"--info", "inline",
		"--cycle",
		"--border",
		"--color", fzfColorSpec(t),
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		// fzf exits 130 (cancel) or 1 (no match) for non-selection.
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			switch exit.ExitCode() {
			case 1, 130:
				return -1, nil
			}
		}
		return -1, fmt.Errorf("running fzf: %w", err)
	}
	line := strings.TrimRight(string(out), "\n")
	if line == "" {
		return -1, nil
	}
	field := line
	if tab := strings.IndexByte(line, '\t'); tab >= 0 {
		field = line[:tab]
	}
	idx, convErr := strconv.Atoi(strings.TrimSpace(field))
	if convErr != nil {
		return -1, fmt.Errorf("unexpected fzf output %q", line)
	}
	return idx, nil
}

func hasTTY() bool {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	_ = tty.Close()
	return true
}
