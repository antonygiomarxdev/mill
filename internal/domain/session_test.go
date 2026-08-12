package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewSessionCreatesSessionWithPendingStatus(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	session := NewSession("session-1", "task-390", 390)
	after := time.Now().UTC().Add(time.Second)

	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.TaskID != "task-390" {
		t.Errorf("expected task ID %q, got %q", "task-390", session.TaskID)
	}
	if session.Issue != 390 {
		t.Errorf("expected issue %d, got %d", 390, session.Issue)
	}
	if session.Status != SessionPending {
		t.Errorf("expected status %q, got %q", SessionPending, session.Status)
	}
	if session.StartedAt.Before(before) || session.StartedAt.After(after) {
		t.Error("StartedAt not within expected range")
	}
}

func TestSessionEndSetsEndedAt(t *testing.T) {
	session := NewSession("session-1", "task-1", 1)

	time.Sleep(10 * time.Millisecond)
	session.End(SessionDone, 0, 2, VerdictApproved, "output text", "artifact.txt")

	if session.Status != SessionDone {
		t.Errorf("expected status %q, got %q", SessionDone, session.Status)
	}
	if session.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", session.ExitCode)
	}
	if session.Commits != 2 {
		t.Errorf("expected commits 2, got %d", session.Commits)
	}
	if session.Verdict != VerdictApproved {
		t.Errorf("expected verdict %q, got %q", VerdictApproved, session.Verdict)
	}
	if session.Output != "output text" {
		t.Errorf("expected output %q, got %q", "output text", session.Output)
	}
	if session.EndedAt.IsZero() {
		t.Error("expected EndedAt to be set")
	}
}

func TestSessionEndWithError(t *testing.T) {
	session := NewSession("session-1", "task-1", 1)
	session.End(SessionError, 1, 0, "", "error output", "artifact.txt")

	if session.Status != SessionError {
		t.Errorf("expected status %q, got %q", SessionError, session.Status)
	}
	if session.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", session.ExitCode)
	}
}

func TestSessionJSONTimestampsAreRFC3339(t *testing.T) {
	session := Session{
		ID:        "sess-1",
		TaskID:    "task-1",
		Issue:     1,
		Status:    SessionDone,
		StartedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, field := range []string{"started_at"} {
		ts, ok := raw[field]
		if !ok {
			t.Fatalf("expected %s field in JSON", field)
		}
		var tsStr string
		if err := json.Unmarshal(ts, &tsStr); err != nil {
			t.Fatalf("expected string timestamp for %s: %v", field, err)
		}
		if _, err := time.Parse(time.RFC3339, tsStr); err != nil {
			t.Errorf("timestamp %q is not valid RFC3339: %v", tsStr, err)
		}
	}
}

func TestSessionStatusValues(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected string
	}{
		{SessionPending, "pending"},
		{SessionRunning, "running"},
		{SessionDone, "done"},
		{SessionError, "error"},
	}

	for _, tc := range tests {
		if string(tc.status) != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, string(tc.status))
		}
	}
}

func TestVerdictValues(t *testing.T) {
	tests := []struct {
		verdict  Verdict
		expected string
	}{
		{VerdictApproved, "approved"},
		{VerdictChanges, "changes"},
		{VerdictRejected, "rejected"},
	}

	for _, tc := range tests {
		if string(tc.verdict) != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, string(tc.verdict))
		}
	}
}

func TestSessionEndPopulatesResultFields(t *testing.T) {
	session := NewSession("session-1", "task-1", 1)

	time.Sleep(10 * time.Millisecond)
	session.RegisterHeartbeat()

	time.Sleep(10 * time.Millisecond)
	artifactPath := "/out/artifact.txt"
	session.End(SessionDone, 0, 0, VerdictApproved, "output", artifactPath)

	if session.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", session.Duration)
	}
	if session.HeartbeatStaleness < 0 {
		t.Errorf("expected non-negative heartbeat staleness, got %v", session.HeartbeatStaleness)
	}
	if session.HeartbeatStaleness >= session.Duration {
		t.Errorf("expected heartbeat staleness < duration, got staleness=%v duration=%v",
			session.HeartbeatStaleness, session.Duration)
	}
	if session.ArtifactPath != artifactPath {
		t.Errorf("expected artifact path %q, got %q", artifactPath, session.ArtifactPath)
	}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, key := range []string{"duration", "heartbeat_staleness", "artifact_path"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected %q key in JSON", key)
		}
	}
}
