package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "state.json")

	s := New()
	s.UpsertTask(domain.Task{
		ID:      "abc123",
		Issue:   390,
		Status:  domain.TaskRunning,
		Commits: 1,
		Verdict: domain.VerdictChanges,
		StartedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC),
	})

	if err := s.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected state.json to be created")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	task, ok := loaded.Task("abc123")
	if !ok {
		t.Fatal("expected task abc123 to exist")
	}

	if task.Issue != 390 {
		t.Errorf("expected issue %d, got %d", 390, task.Issue)
	}
	if task.Status != domain.TaskRunning {
		t.Errorf("expected status %q, got %q", domain.TaskRunning, task.Status)
	}
	if task.Commits != 1 {
		t.Errorf("expected commits %d, got %d", 1, task.Commits)
	}
	if task.Verdict != domain.VerdictChanges {
		t.Errorf("expected verdict %q, got %q", domain.VerdictChanges, task.Verdict)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}

	if len(loaded.Tasks) != 0 {
		t.Errorf("expected empty state, got %d tasks", len(loaded.Tasks))
	}
}

func TestUpsertTaskInsertsNew(t *testing.T) {
	s := New()

	s.UpsertTask(domain.Task{
		ID:      "t1",
		Issue:   10,
		Status:  domain.TaskPending,
		Commits: 0,
	})

	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(s.Tasks))
	}
}

func TestUpsertTaskUpdatesExisting(t *testing.T) {
	s := New()

	s.UpsertTask(domain.Task{
		ID:     "t1",
		Issue:  10,
		Status: domain.TaskPending,
	})

	s.UpsertTask(domain.Task{
		ID:      "t1",
		Issue:   10,
		Status:  domain.TaskDone,
		Commits: 3,
		Verdict: domain.VerdictApproved,
	})

	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task after upsert, got %d", len(s.Tasks))
	}

	task, ok := s.Task("t1")
	if !ok {
		t.Fatal("expected task t1 to exist")
	}
	if task.Status != domain.TaskDone {
		t.Errorf("expected status %q, got %q", domain.TaskDone, task.Status)
	}
	if task.Commits != 3 {
		t.Errorf("expected commits %d, got %d", 3, task.Commits)
	}
}

func TestTaskNotFound(t *testing.T) {
	s := New()

	_, ok := s.Task("nonexistent")
	if ok {
		t.Fatal("expected false for missing task")
	}
}

func TestSaveCreatesMillDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "state.json")

	s := New()
	if err := s.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected .mill/state.json to be created")
	}
}

func TestStateUsesDomainTask(t *testing.T) {
	s := New()
	task := domain.NewTask("task-390", 390)
	s.UpsertTask(task)

	got, ok := s.Task("task-390")
	if !ok {
		t.Fatal("expected task to exist")
	}

	if got.Status != domain.TaskRunning {
		t.Errorf("expected status %q, got %q", domain.TaskRunning, got.Status)
	}
	if !got.StartedAt.Equal(task.StartedAt) {
		t.Errorf("expected StartedAt %v, got %v", task.StartedAt, got.StartedAt)
	}
	if !got.UpdatedAt.Equal(task.UpdatedAt) {
		t.Errorf("expected UpdatedAt %v, got %v", task.UpdatedAt, got.UpdatedAt)
	}
}

func TestLoadOnDirectoryReturnsError(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when reading a directory path")
	}
	if os.IsNotExist(err) {
		t.Fatalf("expected non-NotExist error, got: %v", err)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadEmptyJSONObjectReturnsEmptyState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Tasks == nil {
		t.Fatal("expected Tasks map to be initialized, not nil")
	}
	if len(loaded.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(loaded.Tasks))
	}
}

func TestSaveFailsWhenParentIsFile(t *testing.T) {
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blockingfile")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	s := New()
	err := s.Save(filepath.Join(blockingFile, "state.json"))
	if err == nil {
		t.Fatal("expected error when parent path is a file")
	}
}

func TestUpsertTaskOnZeroValueState(t *testing.T) {
	var s State // zero value: Tasks is nil

	s.UpsertTask(domain.Task{
		ID:     "zero-val",
		Issue:  42,
		Status: domain.TaskPending,
	})

	if s.Tasks == nil {
		t.Fatal("expected Tasks to be initialized after upsert")
	}

	task, ok := s.Task("zero-val")
	if !ok {
		t.Fatal("expected task to exist after upsert on nil map")
	}
	if task.Issue != 42 {
		t.Errorf("expected issue %d, got %d", 42, task.Issue)
	}
}

func TestStateRoundTripsTimestamps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	ts := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	s := State{
		Tasks: map[string]domain.Task{
			"task-1": {
				ID:        "task-1",
				Issue:     1,
				Status:    domain.TaskDone,
				StartedAt: ts,
				UpdatedAt: ts,
			},
		},
	}

	if err := s.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	task, ok := loaded.Task("task-1")
	if !ok {
		t.Fatal("expected task to exist after round-trip")
	}

	if !task.StartedAt.Equal(ts) {
		t.Errorf("StartedAt round-trip mismatch: %v vs %v", task.StartedAt, ts)
	}
	if !task.UpdatedAt.Equal(ts) {
		t.Errorf("UpdatedAt round-trip mismatch: %v vs %v", task.UpdatedAt, ts)
	}
}
