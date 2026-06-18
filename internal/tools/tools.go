// Package tools centralizes discovery of the external binaries bird's-eye drives.
// It probes PATH once so the rest of the program can branch on availability
// instead of repeatedly shelling out.
package tools

import "os/exec"

// Names of the external tools bird's-eye knows about.
const (
	Tmux   = "tmux"
	Tmuxp  = "tmuxp"
	Zoxide = "zoxide"
	Git    = "git"
	Fzf    = "fzf"
)

// Set records which external tools are available on PATH.
type Set struct {
	Tmux   bool
	Tmuxp  bool
	Zoxide bool
	Git    bool
	Fzf    bool
}

// Available reports whether a binary with the given name is on PATH.
func Available(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Probe inspects PATH once and returns the availability of each known tool.
func Probe() Set {
	return Set{
		Tmux:   Available(Tmux),
		Tmuxp:  Available(Tmuxp),
		Zoxide: Available(Zoxide),
		Git:    Available(Git),
		Fzf:    Available(Fzf),
	}
}
