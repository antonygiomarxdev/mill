package domain

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var updateGolden = flag.Bool("update", false, "regenerate golden files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestTaskJSONGoldenRoundTrip(t *testing.T) {
	task := Task{
		ID:           "task-golden",
		Issue:        42,
		Status:       TaskDone,
		Phase:        TaskPhaseReview,
		FailureClass: CLASS_OK,
		Commits:      5,
		Verdict:      VerdictApproved,
		Round:        2,
		StartedAt:    time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC),
	}

	got, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "task_golden.json")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata failed: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden file failed: %v", err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file failed: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("golden mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}

	// Round-trip the golden bytes back into a Task and verify every field.
	var back Task
	if err := json.Unmarshal(want, &back); err != nil {
		t.Fatalf("unmarshal golden failed: %v", err)
	}
	if back.ID != task.ID ||
		back.Issue != task.Issue ||
		back.Status != task.Status ||
		back.Phase != task.Phase ||
		back.FailureClass != task.FailureClass ||
		back.Commits != task.Commits ||
		back.Verdict != task.Verdict ||
		back.Round != task.Round ||
		!back.StartedAt.Equal(task.StartedAt) ||
		!back.UpdatedAt.Equal(task.UpdatedAt) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", back, task)
	}
}
