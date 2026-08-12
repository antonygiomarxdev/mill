// Package lessons provides append-only per-role lesson logging.
// Each role accumulates lessons in .mill/lessons/<role>.md. Lessons are
// appended, never rewritten: each session end adds one entry recording the
// FailureClass and the observable root-cause signal that produced it.
package lessons

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// Append appends a single lesson entry for role to a markdown file in dir.
// The file is opened in append-only mode; existing content is never truncated
// or rewritten.
func Append(dir, role string, fc domain.FailureClass, signal string) error {
	if role == "" {
		return fmt.Errorf("lessons: role must not be empty")
	}

	path := filepath.Join(dir, role+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("lessons: creating directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("lessons: opening file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(formatEntry(fc, signal)); err != nil {
		return fmt.Errorf("lessons: writing entry: %w", err)
	}
	return nil
}

// AppendResult resolves result into a FailureClass and the first matching
// observable signal description, then appends the lesson via Append.
func AppendResult(dir, role string, result domain.SessionResult) error {
	reg := domain.NewSignalRegistry()
	fc := reg.Resolve(result)

	signal := ""
	for _, s := range reg.Signals() {
		if s.Predicate(result) {
			signal = s.Description
			break
		}
	}

	return Append(dir, role, fc, signal)
}

// formatEntry renders a single markdown lesson entry.
func formatEntry(fc domain.FailureClass, signal string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", fc)
	fmt.Fprintf(&b, "**When:** %s\n", time.Now().UTC().Format(time.RFC3339))
	if signal != "" {
		fmt.Fprintf(&b, "**Signal:** %s\n", signal)
	}
	fmt.Fprint(&b, "\n---\n")
	b.WriteString("\n")
	return b.String()
}
