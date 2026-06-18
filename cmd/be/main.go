// Command be is the bird's-eye CLI: a fuzzy view over tmux sessions and the AI
// agents inside them.
package main

import (
	"fmt"
	"os"

	"github.com/jbarap/birds-eye/internal/cli"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, "be:", err)
		os.Exit(1)
	}
}
