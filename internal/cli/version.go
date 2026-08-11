package cli

import (
	"fmt"
	"os/exec"
	"strings"
)

// Version is set at build time via ldflags. If empty, version is
// resolved at runtime from the git repository.
var Version string

// runVersion prints the mill version to the configured output.
// Priority: ldflags build-time version > git describe > "dev".
func (a *App) runVersion(args []string) error {
	v := Version
	if v == "" {
		v = resolveVersion()
	}
	fmt.Fprintln(a.Out, v)
	return nil
}

func resolveVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	out, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "dev"
	}
	return v
}
