package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
)

func TestStatusPrintsHeader(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.Run("status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ID") {
		t.Error("expected output to contain 'ID' header")
	}
	if !strings.Contains(output, "ISSUE") {
		t.Error("expected output to contain 'ISSUE' header")
	}
	if !strings.Contains(output, "STATUS") {
		t.Error("expected output to contain 'STATUS' header")
	}
	if !strings.Contains(output, "COMMITS") {
		t.Error("expected output to contain 'COMMITS' header")
	}
	if !strings.Contains(output, "RUNTIME") {
		t.Error("expected output to contain 'RUNTIME' header")
	}
	if !strings.Contains(output, "VERDICT") {
		t.Error("expected output to contain 'VERDICT' header")
	}
}

func TestStatusPrintsTasks(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	// Pre-populate state
	s := state.New()
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	s.UpsertTask(domain.Task{
		ID:        "task-1",
		Issue:     1,
		Status:    domain.TaskDone,
		Commits:   3,
		Verdict:   domain.VerdictApproved,
		StartedAt: now,
		UpdatedAt: now,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "task-1") {
		t.Error("expected output to contain 'task-1'")
	}
	if !strings.Contains(output, "approved") {
		t.Error("expected output to contain 'approved' verdict")
	}
	// Runtime should be present (non-zero since we set StartedAt)
	if !strings.Contains(output, "h") && !strings.Contains(output, "m") && !strings.Contains(output, "s") {
		t.Error("expected runtime to display a duration value")
	}
}

func TestStatusEmptyStatePrintsHeaderOnly(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.Run("status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ID") {
		t.Error("expected header to be printed even with no tasks")
	}
}

func TestStatusShowsRuntime(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	// Create a task that started 5 minutes ago
	s := state.New()
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	s.UpsertTask(domain.Task{
		ID:        "task-5m",
		Issue:     42,
		Status:    domain.TaskRunning,
		Commits:   0,
		StartedAt: fiveMinAgo,
		UpdatedAt: fiveMinAgo,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "RUNTIME") {
		t.Error("expected 'RUNTIME' header")
	}
	if !strings.Contains(output, "task-5m") {
		t.Error("expected 'task-5m' in output")
	}
	// Runtime should be approximately 5 minutes
	if !strings.Contains(output, "5m") {
		t.Errorf("expected runtime ~5m in output, got: %q", output)
	}
}

func TestStatusShowsDashForZeroStartedAt(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	s := state.New()
	s.UpsertTask(domain.Task{
		ID:     "task-zero",
		Issue:  99,
		Status: domain.TaskPending,
		// StartedAt is zero value — not yet started
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "—") {
		t.Errorf("expected '—' for zero StartedAt runtime, got: %q", output)
	}
}
