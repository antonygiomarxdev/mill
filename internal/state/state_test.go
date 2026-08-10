package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "state.json")

	s := New()
	s.UpsertTask(TaskState{
		ID:      "abc123",
		Issue:   390,
		Status:  "running",
		Commits: 1,
		Verdict: "changes",
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
	if task.Status != "running" {
		t.Errorf("expected status %q, got %q", "running", task.Status)
	}
	if task.Commits != 1 {
		t.Errorf("expected commits %d, got %d", 1, task.Commits)
	}
	if task.Verdict != "changes" {
		t.Errorf("expected verdict %q, got %q", "changes", task.Verdict)
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

	s.UpsertTask(TaskState{
		ID:      "t1",
		Issue:   10,
		Status:  "pending",
		Commits: 0,
	})

	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(s.Tasks))
	}
}

func TestUpsertTaskUpdatesExisting(t *testing.T) {
	s := New()

	s.UpsertTask(TaskState{
		ID:     "t1",
		Issue:  10,
		Status: "pending",
	})

	s.UpsertTask(TaskState{
		ID:     "t1",
		Issue:  10,
		Status: "done",
		Commits: 3,
		Verdict: "approved",
	})

	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task after upsert, got %d", len(s.Tasks))
	}

	task, ok := s.Task("t1")
	if !ok {
		t.Fatal("expected task t1 to exist")
	}
	if task.Status != "done" {
		t.Errorf("expected status %q, got %q", "done", task.Status)
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
