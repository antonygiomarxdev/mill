package lessons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestAppend_ConcatenatesEntries(t *testing.T) {
	dir := t.TempDir()
	role := "architect"

	fc1, sig1 := domain.CLASS_OK, ""
	fc2, sig2 := domain.EXECUTION_FAILURE, "stderr indicates connection refused or network timeout"

	if err := Append(dir, role, fc1, sig1); err != nil {
		t.Fatalf("first Append failed: %v", err)
	}
	if err := Append(dir, role, fc2, sig2); err != nil {
		t.Fatalf("second Append failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, role+".md"))
	if err != nil {
		t.Fatalf("reading lesson file: %v", err)
	}

	// Both failure-class headings must be present.
	if !strings.Contains(string(content), "## CLASS_OK") {
		t.Errorf("file missing first entry heading ## CLASS_OK")
	}
	if !strings.Contains(string(content), "## EXECUTION_FAILURE") {
		t.Errorf("file missing second entry heading ## EXECUTION_FAILURE")
	}

	// Exactly two entries must be present: the second must still contain the
	// first entry's text (i.e. the file was not overwritten).
	if got := strings.Count(string(content), "## "); got != 2 {
		t.Errorf("expected 2 entries in file, got %d", got)
	}

	// RFC3339 timestamps are fixed-length, so each appended entry has the same
	// length regardless of the current wall-clock time. The combined content
	// length must be at least the sum of the two individual entries.
	e1 := formatEntry(fc1, sig1)
	e2 := formatEntry(fc2, sig2)
	if len(content) < len(e1)+len(e2) {
		t.Errorf("content length %d < sum of entries %d", len(content), len(e1)+len(e2))
	}
}

func TestAppend_PreservesPreExistingContent(t *testing.T) {
	dir := t.TempDir()
	role := "pm"
	sentinel := "PRE-EXISTING LEADERSHIP NOTES"

	path := filepath.Join(dir, role+".md")
	if err := os.WriteFile(path, []byte(sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("writing pre-existing file: %v", err)
	}

	if err := Append(dir, role, domain.CLASS_OK, ""); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading lesson file: %v", err)
	}

	if !strings.HasPrefix(string(content), sentinel) {
		t.Errorf("pre-existing content was not preserved at start of file:\n%s", string(content))
	}
}

func TestAppendResult_ResolvesFailureClassAndSignal(t *testing.T) {
	t.Run("connection refused -> EXECUTION_FAILURE with signal", func(t *testing.T) {
		dir := t.TempDir()
		result := domain.SessionResult{Stderr: "dial tcp: connection refused"}

		if err := AppendResult(dir, "architect", result); err != nil {
			t.Fatalf("AppendResult failed: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(dir, "architect.md"))
		if err != nil {
			t.Fatalf("reading lesson file: %v", err)
		}

		if !strings.Contains(string(content), "## EXECUTION_FAILURE") {
			t.Errorf("expected EXECUTION_FAILURE in file:\n%s", string(content))
		}
		if !strings.Contains(string(content), "**Signal:** stderr indicates connection refused or network timeout") {
			t.Errorf("expected observable signal description in file:\n%s", string(content))
		}
	})

	t.Run("exit code 0 clean -> CLASS_OK no signal", func(t *testing.T) {
		dir := t.TempDir()
		result := domain.SessionResult{ExitCode: 0}

		if err := AppendResult(dir, "architect", result); err != nil {
			t.Fatalf("AppendResult failed: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(dir, "architect.md"))
		if err != nil {
			t.Fatalf("reading lesson file: %v", err)
		}

		if !strings.Contains(string(content), "## CLASS_OK") {
			t.Errorf("expected CLASS_OK in file:\n%s", string(content))
		}
		if strings.Contains(string(content), "**Signal:**") {
			t.Errorf("expected no signal line for clean result:\n%s", string(content))
		}
	})
}

func TestAppend_RoleEmptyError(t *testing.T) {
	err := Append(t.TempDir(), "", domain.CLASS_OK, "")
	if err == nil {
		t.Fatal("expected error for empty role, got nil")
	}
	if !strings.Contains(err.Error(), "role must not be empty") {
		t.Errorf("expected role must not be empty error, got: %v", err)
	}
}
