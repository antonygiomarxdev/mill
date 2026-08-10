package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validActiveRoles lists the only roles that can be active in a CTO session.
// Per docs/ARCHITECTURE.md: only Staff and PM interact directly with CTO.
var validActiveRoles = map[string]bool{
	"staff": true,
	"pm":    true,
}

// runRole handles the "role" command.
// Subcommands: get (prints current role), set (writes .mill/role).
func (a *App) runRole(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(a.Err, "usage: mill role <get|set> [role]\n")
		return fmt.Errorf("usage: mill role <get|set> [role]")
	}

	switch args[0] {
	case "get":
		return a.roleGet()
	case "set":
		if len(args) < 2 {
			fmt.Fprintf(a.Err, "usage: mill role set <role>\n")
			return fmt.Errorf("usage: mill role set <role>")
		}
		return a.roleSet(args[1])
	default:
		fmt.Fprintf(a.Err, "usage: mill role <get|set> [role]\n")
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

// roleGet reads and prints the current role from .mill/role.
func (a *App) roleGet() error {
	roleFile := filepath.Join(a.MillDir, "role")
	data, err := os.ReadFile(roleFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(a.Out, "none")
			return nil
		}
		return fmt.Errorf("failed to read role: %w", err)
	}
	role := strings.TrimSpace(string(data))
	if role == "" {
		role = "none"
	}
	fmt.Fprintln(a.Out, role)
	return nil
}

// roleSet validates and writes the active role to .mill/role.
func (a *App) roleSet(role string) error {
	role = strings.ToLower(strings.TrimSpace(role))

	if !validActiveRoles[role] {
		validList := "staff, pm"
		if _, ok := knownRoles[role]; ok {
			return fmt.Errorf("%s is delegation-only, not an active role. Valid: %s", role, validList)
		}
		return fmt.Errorf("unknown role: %s. Valid: %s", role, validList)
	}

	roleFile := filepath.Join(a.MillDir, "role")
	if err := os.MkdirAll(a.MillDir, 0755); err != nil {
		return fmt.Errorf("failed to create .mill directory: %w", err)
	}
	if err := os.WriteFile(roleFile, []byte(role), 0644); err != nil {
		return fmt.Errorf("failed to write role: %w", err)
	}

	fmt.Fprintf(a.Out, "mill: switched to %s\n", role)
	return nil
}

// knownRoles is the set of all roles defined in the org chart.
// Used to distinguish "delegation-only" from "unknown" in error messages.
var knownRoles = map[string]bool{
	"staff":        true,
	"pm":           true,
	"architect":    true,
	"tech-lead":    true,
	"reviewer":     true,
	"sr-dev":       true,
	"sr-dev-be":    true,
	"sr-dev-fe":    true,
	"sr-dev-data":  true,
	"ux-designer":  true,
	"ui-designer":  true,
	"qa-docs":      true,
}
