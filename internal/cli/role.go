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
			fmt.Fprintln(a.Out, "staff")
			return nil
		}
		return fmt.Errorf("failed to read role: %w", err)
	}
	role := strings.TrimSpace(string(data))
	if role == "" {
		role = "staff"
	}
	if !validActiveRoles[role] {
		return fmt.Errorf("invalid role %q in .mill/role: only staff and pm are valid active roles; correct the file or run 'mill role set staff'", role)
	}
	fmt.Fprintln(a.Out, role)
	return nil
}

// roleSet validates and writes the active role to .mill/role.
func (a *App) roleSet(role string) error {
	role = strings.ToLower(strings.TrimSpace(role))

	if !validActiveRoles[role] {
		return fmt.Errorf("role '%s' is delegation-only. Use mill delegate to dispatch work to this role.", role)
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

// detectRole classifies input text into a role.
// Product keywords → "pm", technical keywords → "staff", unknown/empty → "staff".
func detectRole(input string) string {
	lower := strings.ToLower(input)

	productKeywords := []string{"feature", "user", "design", "spec", "priority", "product", "ui", "ux"}
	for _, kw := range productKeywords {
		if wordMatch(lower, kw) {
			return "pm"
		}
	}

	techKeywords := []string{"code", "bug", "architecture", "deploy", "test", "build", "refactor", "impl", "fix"}
	for _, kw := range techKeywords {
		if wordMatch(lower, kw) {
			return "staff"
		}
	}

	return "staff"
}

// wordMatch returns true if word appears as a whole word in s.
// A word boundary is a space, punctuation, or string start/end.
func wordMatch(s, word string) bool {
	i := 0
	for {
		idx := strings.Index(s[i:], word)
		if idx < 0 {
			return false
		}
		pos := i + idx
		before := pos == 0 || isBoundary(s[pos-1])
		after := pos+len(word) == len(s) || isBoundary(s[pos+len(word)])
		if before && after {
			return true
		}
		i = pos + 1
	}
}

func isBoundary(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '.' || b == ',' ||
		b == '!' || b == '?' || b == ':' || b == ';' || b == '-' ||
		b == '(' || b == ')' || b == '[' || b == ']'
}
