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

	ordered := orderByType(cands, cfg.Order)
	var input strings.Builder
	for i, c := range ordered {
		// "<index>\t<display>"; index is hidden from the view via --with-nth.
		fmt.Fprintf(&input, "%d\t%s\n", i, display(c, cfg))
	}

	idx, err := runFzf(input.String())
	if err != nil {
		return nil, err
	}
	if idx < 0 || idx >= len(ordered) {
		return nil, ErrCancelled
	}
	return &ordered[idx], nil
}

func display(c provider.Candidate, cfg config.Config) string {
	marker := "→" // attach to existing
	if c.Kind == provider.KindCreate {
		marker = "+" // create new
	}
	return fmt.Sprintf("%s %s  (%s)", marker, c.Label, cfg.Label(c.Type))
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

// runFzf pipes input to fzf and returns the selected line's index field, or -1
// when cancelled.
func runFzf(input string) (int, error) {
	cmd := exec.Command("fzf",
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--prompt", "be> ",
		"--no-multi",
		"--height", "100%",
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
