package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Version is set at build time via ldflags. If empty, version is
// resolved at runtime from the VERSION file or git repository.
var Version string

// runVersion prints the mill version to the configured output.
// Priority: ldflags build-time version > VERSION file > git describe > "dev".
func (a *App) runVersion(args []string) error {
	v := Version
	if v == "" {
		v = resolveVersion()
	}
	fmt.Fprintln(a.Out, normalizeVersion(v))
	return nil
}

// resolveVersion determines the current version string.
// Priority: VERSION file > git describe > "dev".
func resolveVersion() string {
	// Try the VERSION file (committed, meaningful version for dev checkouts).
	if data, err := os.ReadFile("VERSION"); err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			return v
		}
	}
	// Fall back to git describe. The --dirty flag is omitted so that
	// uncommitted files in a dev checkout do not permanently suffix the
	// version with "-dirty".
	v, err := gitDescribe()
	if err != nil || v == "" {
		return "dev"
	}
	return v
}

// gitDescribe returns the output of `git describe --tags --always`.
// Overridable in tests to avoid real git calls.
var gitDescribe = func() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// normalizeVersion ensures the version string is non-empty and starts
// with "v" (e.g. "0.1.0" becomes "v0.1.0").
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
