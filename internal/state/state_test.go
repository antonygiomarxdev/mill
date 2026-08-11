package state

import (
	"encoding/json"
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

func TestSaveAtomicNoTempFileLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})

	if err := s.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Temp file must not exist after atomic rename.
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %s should not exist after atomic save", tmpPath)
	}

	// File must be valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read state file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("state file is empty")
	}
}

func TestSaveBackupRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	for i := 1; i <= 4; i++ {
		s := New()
		s.UpsertTask(domain.Task{ID: "t" + string(rune('0'+i)), Issue: i, Status: domain.TaskDone})
		if err := s.Save(path); err != nil {
			t.Fatalf("Save %d failed: %v", i, err)
		}
	}

	// After 4 saves: primary + .1 + .2 exist, .3 does not.
	for _, ext := range []string{"", ".1", ".2"} {
		if _, err := os.Stat(path + ext); os.IsNotExist(err) {
			t.Errorf("expected backup %s to exist", path+ext)
		}
	}

	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Error("backup .3 should not exist (max 3 copies)")
	}
}

func TestLoadFallbackToBackup1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save valid state twice — second save creates primary and shifts first to .1.
	s1 := New()
	s1.UpsertTask(domain.Task{ID: "from-backup", Issue: 1, Status: domain.TaskDone})
	if err := s1.Save(path); err != nil {
		t.Fatalf("Save 1 failed: %v", err)
	}

	s2 := New()
	s2.UpsertTask(domain.Task{ID: "from-primary", Issue: 2, Status: domain.TaskDone})
	if err := s2.Save(path); err != nil {
		t.Fatalf("Save 2 failed: %v", err)
	}

	// Corrupt primary.
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("failed to corrupt primary: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load should fall back to .1: %v", err)
	}

	if _, ok := loaded.Task("from-backup"); !ok {
		t.Error("expected task from backup .1 to be loaded")
	}
}

func TestLoadFallbackToBackup2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save 3 times: oldest in .2, middle in .1, newest in primary.
	for i := 1; i <= 3; i++ {
		s := New()
		s.UpsertTask(domain.Task{ID: "v" + string(rune('0'+i)), Issue: i, Status: domain.TaskDone})
		if err := s.Save(path); err != nil {
			t.Fatalf("Save %d failed: %v", i, err)
		}
	}

	// Corrupt primary and .1.
	if err := os.WriteFile(path, []byte("bad"), 0o644); err != nil {
		t.Fatalf("failed to corrupt primary: %v", err)
	}
	if err := os.WriteFile(path+".1", []byte("bad"), 0o644); err != nil {
		t.Fatalf("failed to corrupt .1: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load should fall back to .2: %v", err)
	}

	if _, ok := loaded.Task("v1"); !ok {
		t.Error("expected task from backup .2 to exist")
	}
}

func TestLoadAllCorruptReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Corrupt all three files.
	for _, ext := range []string{"", ".1", ".2"} {
		if err := os.WriteFile(path+ext, []byte("bad json {{{"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path+ext, err)
		}
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error when all files are corrupt")
	}
}

func TestLoadMissingBackupsNotError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Primary is corrupt, no backups exist.
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("failed to write corrupt primary: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error when primary corrupt and no backups exist")
	}
}

func TestSaveBackupContentMatchesSaveOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save 1: task "first"
	s1 := New()
	s1.UpsertTask(domain.Task{ID: "first", Issue: 1, Status: domain.TaskDone})
	if err := s1.Save(path); err != nil {
		t.Fatalf("Save 1 failed: %v", err)
	}

	// Save 2: task "second"
	s2 := New()
	s2.UpsertTask(domain.Task{ID: "second", Issue: 2, Status: domain.TaskRunning})
	if err := s2.Save(path); err != nil {
		t.Fatalf("Save 2 failed: %v", err)
	}

	// .1 should contain "first"
	data, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("cannot read .1: %v", err)
	}
	var b1 State
	if err := json.Unmarshal(data, &b1); err != nil {
		t.Fatalf("cannot parse .1: %v", err)
	}
	if _, ok := b1.Task("first"); !ok {
		t.Error(".1 should contain task 'first'")
	}
	if _, ok := b1.Task("second"); ok {
		t.Error(".1 should NOT contain task 'second'")
	}

	// Primary should contain "second"
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load primary failed: %v", err)
	}
	if _, ok := loaded.Task("second"); !ok {
		t.Error("primary should contain task 'second'")
	}
}
