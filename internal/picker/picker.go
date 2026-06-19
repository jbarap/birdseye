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
	"github.com/jbarap/birds-eye/internal/theme"
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

	tc := theme.Truecolor()
	ordered := orderByType(cands, cfg.Order)
	var input strings.Builder
	for i, c := range ordered {
		// "<index>\t<display>"; index is hidden from the view via --with-nth.
		fmt.Fprintf(&input, "%d\t%s\n", i, display(c, cfg, tc))
	}

	idx, err := runFzf(input.String(), tc)
	if err != nil {
		return nil, err
	}
	if idx < 0 || idx >= len(ordered) {
		return nil, ErrCancelled
	}
	return &ordered[idx], nil
}

// Bold/dim attribute escapes are emitted raw so they survive the pipe to fzf
// (which renders them under --ansi); lipgloss would strip them when stdout is
// not a TTY. Color comes from the shared theme package.
const ansiReset = "\x1b[0m"

func bold(s string) string { return "\x1b[1m" + s + ansiReset }
func dim(s string) string  { return "\x1b[2m" + s + ansiReset }

// colorOf is the accent color per candidate type; unlisted types use gray.
func colorOf(typ string) theme.Color {
	switch typ {
	case "tmux":
		return theme.Blue
	case "tmuxp":
		return theme.Coral
	case "dir":
		return theme.Gold
	case "worktree":
		return theme.Green
	default:
		return theme.Gray
	}
}

func display(c provider.Candidate, cfg config.Config, tc bool) string {
	// Kind marker: a green "+" to create, a blue "→" to attach to an existing one.
	mark := theme.Attach.FG("→", tc)
	if c.Kind == provider.KindCreate {
		mark = theme.Create.FG("+", tc)
	}

	var b strings.Builder
	b.WriteString(mark)
	b.WriteByte(' ')
	if icon := cfg.Icon(c.Type); icon != "" {
		b.WriteString(colorOf(c.Type).FG(icon, tc))
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

// fzfColorSpec builds fzf's --color argument from the shared palette. The selection
// chrome (match highlight, pointer, prompt, marker) uses the tool-wide accent so fzf
// presents "the selected thing" the same way the agents view does; type colors are
// carried by the candidate rows themselves. Uses 24-bit or 256 to suit the terminal.
func fzfColorSpec(tc bool) string {
	return strings.Join([]string{
		"hl:" + theme.Blue.Spec(tc),
		"hl+:" + theme.Accent.Spec(tc),
		"pointer:" + theme.Accent.Spec(tc),
		"prompt:" + theme.Accent.Spec(tc),
		"marker:" + theme.Accent.Spec(tc),
		"label:" + theme.Accent.Spec(tc),
		"border:" + theme.Border.Spec(tc),
		"info:" + theme.Border.Spec(tc),
		"gutter:-1",
	}, ",")
}

// fzfGutterCharSupported reports whether the installed fzf accepts the --gutter
// character flag, which was added in 0.72 alongside the default gutter bar. Older
// fzf rejects unknown flags (and draws no bar), so the flag is only needed and safe
// from 0.72 on. A failed probe is treated as unsupported.
func fzfGutterCharSupported() bool {
	out, err := exec.Command("fzf", "--version").Output()
	if err != nil {
		return false
	}
	return fzfVersionAtLeast(string(out), 0, 72)
}

// fzfBorderLabelSupported reports whether the installed fzf accepts the embedded
// border-title flags (--border-label / --border-label-pos and the `label` --color
// key), added in 0.35. Older fzf rejects them, so the title degrades to a plain
// bordered panel. A failed probe is treated as unsupported.
func fzfBorderLabelSupported() bool {
	out, err := exec.Command("fzf", "--version").Output()
	if err != nil {
		return false
	}
	return fzfVersionAtLeast(string(out), 0, 35)
}

// fzfVersionAtLeast parses fzf --version output (e.g. "0.72.0 (sha)") and reports
// whether it is at least maj.min. Unparseable input reports false.
func fzfVersionAtLeast(version string, maj, min int) bool {
	fields := strings.Fields(version)
	if len(fields) == 0 {
		return false
	}
	parts := strings.SplitN(fields[0], ".", 3)
	if len(parts) < 2 {
		return false
	}
	gotMaj, err1 := strconv.Atoi(parts[0])
	gotMin, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	if gotMaj != maj {
		return gotMaj > maj
	}
	return gotMin >= min
}

// fzfArgs builds fzf's argument list. The capability booleans gate flags that only
// newer fzf understands, so the picker degrades cleanly on older versions: borderLabel
// embeds the `sessions` title in the panel border (the tool-wide titled-panel look),
// and gutterChar blanks fzf's per-row gutter bar.
func fzfArgs(tc, gutterChar, borderLabel bool) []string {
	args := []string{
		"--ansi",
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--prompt", "❯ ",
		"--pointer", theme.CursorGlyph,
		"--no-multi",
		"--height", "100%",
		"--layout", "reverse",
		"--info", "inline",
		"--border",
		"--color", fzfColorSpec(tc),
	}
	// The title rides in the panel's top border, near the top-left corner (pos 2),
	// matching the agents view's titled panels; the prompt is a bare chevron.
	if borderLabel {
		args = append(args, "--border-label", " sessions ", "--border-label-pos", "2")
	}
	// fzf >=0.72 draws a left-column gutter bar (default "▌") on every row; our
	// --color only sets its color, so it renders in the terminal's default
	// foreground (white). Blank it with a space so there is no per-entry line (the
	// pointer still marks the current row). The flag is unknown to older fzf — which
	// also does not draw the bar — so only pass it when the version supports it.
	if gutterChar {
		args = append(args, "--gutter", " ")
	}
	return args
}

// runFzf pipes input to fzf and returns the selected line's index field, or -1
// when cancelled.
func runFzf(input string, tc bool) (int, error) {
	args := fzfArgs(tc, fzfGutterCharSupported(), fzfBorderLabelSupported())
	cmd := exec.Command("fzf", args...)
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
