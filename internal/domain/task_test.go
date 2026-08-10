package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewTaskCreatesTaskWithPendingStatus(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	task := NewTask("task-390", 390)
	after := time.Now().UTC().Add(time.Second)

	if task.ID != "task-390" {
		t.Errorf("expected ID %q, got %q", "task-390", task.ID)
	}
	if task.Issue != 390 {
		t.Errorf("expected issue %d, got %d", 390, task.Issue)
	}
	if task.Status != TaskRunning {
		t.Errorf("expected status %q, got %q", TaskRunning, task.Status)
	}
	if task.Commits != 0 {
		t.Errorf("expected commits 0, got %d", task.Commits)
	}
	if task.Verdict != "" {
		t.Errorf("expected empty verdict, got %q", task.Verdict)
	}
	if task.StartedAt.Before(before) || task.StartedAt.After(after) {
		t.Errorf("StartedAt not within expected range")
	}
	if task.UpdatedAt.Before(before) || task.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt not within expected range")
	}
}

func TestTaskUpdateStatusSetsUpdatedAt(t *testing.T) {
	task := NewTask("task-1", 1)
	originalUpdated := task.UpdatedAt

	time.Sleep(10 * time.Millisecond)
	task.UpdateStatus(TaskDone, VerdictApproved, 3)

	if task.Status != TaskDone {
		t.Errorf("expected status %q, got %q", TaskDone, task.Status)
	}
	if task.Commits != 3 {
		t.Errorf("expected commits 3, got %d", task.Commits)
	}
	if task.Verdict != VerdictApproved {
		t.Errorf("expected verdict %q, got %q", VerdictApproved, task.Verdict)
	}
	if !task.UpdatedAt.After(originalUpdated) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestTaskJSONTimestampsAreRFC3339(t *testing.T) {
	task := Task{
		ID:        "task-1",
		Issue:     1,
		Status:    TaskDone,
		StartedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	ts, ok := raw["started_at"]
	if !ok {
		t.Fatal("expected started_at field in JSON")
	}

	var tsStr string
	if err := json.Unmarshal(ts, &tsStr); err != nil {
		t.Fatalf("expected string timestamp: %v", err)
	}

	if _, err := time.Parse(time.RFC3339, tsStr); err != nil {
		t.Errorf("timestamp %q is not valid RFC3339: %v", tsStr, err)
	}
}

func TestTaskStatusValues(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskPending, "pending"},
		{TaskRunning, "running"},
		{TaskDone, "done"},
		{TaskError, "error"},
	}

	for _, tc := range tests {
		if string(tc.status) != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, string(tc.status))
		}
	}
}
