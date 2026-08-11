package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
)

func TestRunWatchNoTasks(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.Run("watch")
	if err != nil {
		t.Fatalf("watch with no tasks returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No tasks to watch") {
		t.Errorf("expected 'No tasks to watch', got: %q", output)
	}
}

func TestRunWatchAllDone(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	s := state.New()
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
	err := app.Run("watch")
	if err != nil {
		t.Fatalf("watch all done returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "tasks succeeded") {
		t.Errorf("expected 'tasks succeeded' in output, got: %q", output)
	}
	if !strings.Contains(output, "task-1") {
		t.Errorf("expected 'task-1' in output, got: %q", output)
	}
}

func TestRunWatchOneError(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	s := state.New()
	s.UpsertTask(domain.Task{
		ID:        "task-1",
		Issue:     1,
		Status:    domain.TaskDone,
		Commits:   5,
		Verdict:   domain.VerdictApproved,
		StartedAt: now,
		UpdatedAt: now,
	})
	s.UpsertTask(domain.Task{
		ID:        "task-2",
		Issue:     2,
		Status:    domain.TaskError,
		Commits:   0,
		StartedAt: now,
		UpdatedAt: now,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("watch")
	if err == nil {
		t.Fatal("expected error for one failed task")
	}

	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CommandError, got %T: %v", err, err)
	}
	if ce.Code != 2 {
		t.Errorf("expected exit code 2 (1+1), got %d", ce.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "1/2 tasks succeeded, 1 failed") {
		t.Errorf("expected '1/2 tasks succeeded, 1 failed' in output, got: %q", output)
	}
	if !strings.Contains(output, "FATAL") {
		t.Errorf("expected 'FATAL' verdict for error task, got: %q", output)
	}
}

func TestRunWatchProgressOutput(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	now := time.Now()
	s := state.New()
	s.UpsertTask(domain.Task{
		ID:        "task-42",
		Issue:     42,
		Status:    domain.TaskRunning,
		Commits:   0,
		StartedAt: now,
		UpdatedAt: now,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	// Since the task never becomes terminal, this will run until timeout.
	// Use --timeout 1 to exit quickly.
	err := app.Run("watch", "--timeout", "1s")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CommandError, got %T: %v", err, err)
	}
	if ce.Code != 124 {
		t.Errorf("expected exit code 124, got %d", ce.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "running") {
		t.Errorf("expected progress output to contain 'running', got: %q", output)
	}
	if !strings.Contains(output, "task-42") {
		t.Errorf("expected progress output to contain 'task-42', got: %q", output)
	}
}

func TestRunWatchTimeout(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	now := time.Now()
	s := state.New()
	s.UpsertTask(domain.Task{
		ID:        "task-99",
		Issue:     99,
		Status:    domain.TaskRunning,
		Commits:   0,
		StartedAt: now,
		UpdatedAt: now,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("watch", "--timeout", "1s")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CommandError, got %T: %v", err, err)
	}
	if ce.Code != 124 {
		t.Errorf("expected exit code 124, got %d", ce.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "Still running") {
		t.Errorf("expected 'Still running' in timeout output, got: %q", output)
	}
	if !strings.Contains(output, "task-99") {
		t.Errorf("expected 'task-99' in timeout output, got: %q", output)
	}
}

func TestRunWatchInterval(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	now := time.Now()
	s := state.New()
	s.UpsertTask(domain.Task{
		ID:        "task-7",
		Issue:     7,
		Status:    domain.TaskRunning,
		Commits:   0,
		StartedAt: now,
		UpdatedAt: now,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	// --interval 1 --timeout 1 should work without panicking.
	err := app.Run("watch", "--interval", "1", "--timeout", "1s")

	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CommandError from timeout, got %T: %v", err, err)
	}
	if ce.Code != 124 {
		t.Errorf("expected exit code 124, got %d", ce.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "task-7") {
		t.Errorf("expected output to contain 'task-7', got: %q", output)
	}
}

func TestRunWatchHelpFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}

	err := app.runWatch([]string{"-h"})
	if err != nil {
		t.Fatalf("runWatch with -h returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mill watch") {
		t.Errorf("expected help text to contain 'mill watch', got: %q", output)
	}
	if !strings.Contains(output, "--interval") {
		t.Errorf("expected help text to contain '--interval', got: %q", output)
	}
	if !strings.Contains(output, "--timeout") {
		t.Errorf("expected help text to contain '--timeout', got: %q", output)
	}
}

func TestRunWatchFilterOnlyDelegateTasks(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	now := time.Now()
	s := state.New()
	s.UpsertTask(domain.Task{
		ID:        "task-42",
		Issue:     42,
		Status:    domain.TaskDone,
		Commits:   1,
		Verdict:   domain.VerdictApproved,
		StartedAt: now,
		UpdatedAt: now,
	})
	s.UpsertTask(domain.Task{
		ID:        "manual-insert",
		Issue:     99,
		Status:    domain.TaskDone,
		Commits:   0,
		StartedAt: now,
		UpdatedAt: now,
	})
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("watch")
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "task-42") {
		t.Errorf("expected output to contain 'task-42', got: %q", output)
	}
	if strings.Contains(output, "manual-insert") {
		t.Errorf("expected output NOT to contain 'manual-insert', got: %q", output)
	}
	if !strings.Contains(output, "tasks succeeded") {
		t.Errorf("expected 'tasks succeeded' in output, got: %q", output)
	}
}

func TestRunWatchRouting(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)

	// Empty state — watch with no tasks should print "No tasks to watch".
	s := state.New()
	if err := s.Save(dir + "/state.json"); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("watch")
	if err != nil {
		t.Fatalf("watch routing returned error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "unknown command") {
		t.Error("expected watch to be dispatched, not fall through to unknown command")
	}
	if !strings.Contains(output, "No tasks to watch") {
		t.Errorf("expected 'No tasks to watch', got: %q", output)
	}
}

func TestCommandErrorError(t *testing.T) {
	e := &CommandError{Code: 42, Msg: "something went wrong"}
	if e.Error() != "something went wrong" {
		t.Errorf("Error() = %q, want %q", e.Error(), "something went wrong")
	}
}

func TestRunWatchDoubleDashHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}

	err := app.runWatch([]string{"--help"})
	if err != nil {
		t.Fatalf("runWatch with --help returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mill watch") {
		t.Errorf("expected help text to contain 'mill watch', got: %q", output)
	}
}
