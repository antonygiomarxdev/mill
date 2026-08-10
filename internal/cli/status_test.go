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
