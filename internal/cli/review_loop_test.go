package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
)

// multiResultAdapter supports a sequence of results for testing cycles.
type multiResultAdapter struct {
	results   []adapter.SessionResult
	callCount int
}

func (m *multiResultAdapter) Dispatch(opts adapter.DispatchOpts) (adapter.Session, error) {
	idx := m.callCount
	m.callCount++
	if idx >= len(m.results) {
		idx = len(m.results) - 1
	}
	return &fakeSession{result: m.results[idx]}, nil
}

func (m *multiResultAdapter) Resume(sessionID string) (adapter.Session, error) {
	return &fakeSession{}, nil
}

func (m *multiResultAdapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Models: []string{"test"}}
}

func TestReviewLoopApprovedFirstRound(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	madapter := &multiResultAdapter{
		results: []adapter.SessionResult{
			{ExitCode: 0, Commits: 3, Output: "code produced", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "review done", Stderr: "APPROVED: LGTM"},
		},
	}

	cfg := config.Default()
	cfg.MaxRounds = 4

	var errBuf bytes.Buffer
	app := &App{
		Adapter:     madapter,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         &errBuf,
		Out:         &bytes.Buffer{},
	}

	opts := adapter.DispatchOpts{
		Worktree: dir,
		Prompt:   "Fix the bug",
		Model:    "laguna-free",
		MaxTurns: 10,
	}

	err := runDispatchLoop54(app, 1, "task-1", opts, "Test issue body", nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}
	if !strings.Contains(string(data), `"status": "done"`) {
		t.Error("expected task status 'done'")
	}
	if !strings.Contains(string(data), `"verdict": "approved"`) {
		t.Error("expected verdict 'approved'")
	}

	ledgerData, err := os.ReadFile(filepath.Join(dir, "ledger", "1.jsonl"))
	if err != nil {
		t.Fatalf("ledger file not created: %v", err)
	}
	ledgerStr := string(ledgerData)
	if !strings.Contains(ledgerStr, `"event":"produce"`) {
		t.Error("expected produce ledger entry")
	}
	if !strings.Contains(ledgerStr, `"event":"review"`) {
		t.Error("expected review ledger entry")
	}
	if !strings.Contains(ledgerStr, `"event":"complete"`) {
		t.Error("expected complete ledger entry")
	}
}

func TestReviewLoopChangesRequestedThenApproved(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	madapter := &multiResultAdapter{
		results: []adapter.SessionResult{
			{ExitCode: 0, Commits: 2, Output: "v1", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "review", Stderr: "CHANGES_REQUESTED: 1. Missing error handling"},
			{ExitCode: 0, Commits: 4, Output: "v2", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "review2", Stderr: "APPROVED: good"},
		},
	}

	cfg := config.Default()
	cfg.MaxRounds = 4

	var errBuf bytes.Buffer
	app := &App{
		Adapter:     madapter,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         &errBuf,
		Out:         &bytes.Buffer{},
	}

	opts := adapter.DispatchOpts{
		Worktree: dir,
		Prompt:   "Fix the bug",
		Model:    "laguna-free",
		MaxTurns: 10,
	}

	err := runDispatchLoop54(app, 2, "task-2", opts, "Test issue body", nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}
	if !strings.Contains(string(data), `"status": "done"`) {
		t.Error("expected task status 'done'")
	}
	if !strings.Contains(string(data), `"verdict": "approved"`) {
		t.Error("expected verdict 'approved'")
	}
}

func TestReviewLoopMaxCyclesExhausted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	madapter := &multiResultAdapter{
		results: []adapter.SessionResult{
			{ExitCode: 0, Commits: 1, Output: "v1", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r1", Stderr: "CHANGES_REQUESTED: 1. Fix X"},
			{ExitCode: 0, Commits: 2, Output: "v2", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r2", Stderr: "CHANGES_REQUESTED: 2. Fix Y"},
			{ExitCode: 0, Commits: 3, Output: "v3", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r3", Stderr: "CHANGES_REQUESTED: 3. Fix Z"},
			{ExitCode: 0, Commits: 4, Output: "v4", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r4", Stderr: "CHANGES_REQUESTED: 4. Fix W"},
		},
	}

	cfg := config.Default()
	cfg.MaxRounds = 4

	var errBuf bytes.Buffer
	app := &App{
		Adapter:     madapter,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         &errBuf,
		Out:         &bytes.Buffer{},
	}

	opts := adapter.DispatchOpts{
		Worktree: dir,
		Prompt:   "Fix the bug",
		Model:    "laguna-free",
		MaxTurns: 10,
	}

	err := runDispatchLoop54(app, 3, "task-3", opts, "Test issue body", nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}
	if !strings.Contains(string(data), `"status": "error"`) {
		t.Error("expected task status 'error'")
	}
	if !strings.Contains(string(data), `"verdict": "changes_requested"`) {
		t.Error("expected verdict 'changes_requested'")
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "ESCALATION: Review cycle exhausted") {
		t.Error("expected escalation message on stderr")
	}
	if !strings.Contains(errOutput, "Review feedback:") {
		t.Error("expected review feedback summary in escalation")
	}
}

func TestReviewLoopBlockedImmediate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	madapter := &multiResultAdapter{
		results: []adapter.SessionResult{
			{ExitCode: 0, Commits: 1, Output: "produced", Stderr: ""},
			{ExitCode: 1, Commits: 0, Output: "review", Stderr: "BLOCKED: missing API credentials"},
		},
	}

	cfg := config.Default()
	cfg.MaxRounds = 4

	var errBuf bytes.Buffer
	app := &App{
		Adapter:     madapter,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         &errBuf,
		Out:         &bytes.Buffer{},
	}

	opts := adapter.DispatchOpts{
		Worktree: dir,
		Prompt:   "Fix the bug",
		Model:    "laguna-free",
		MaxTurns: 10,
	}

	err := runDispatchLoop54(app, 4, "task-4", opts, "Test issue body", nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}
	if !strings.Contains(string(data), `"status": "error"`) {
		t.Error("expected task status 'error'")
	}
	if !strings.Contains(string(data), `"verdict": "rejected"`) {
		t.Error("expected verdict 'rejected'")
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "ESCALATION: Review cycle aborted") {
		t.Error("expected escalation message on stderr")
	}
}

func TestClassifyResultReviewSignals_54(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		stderr string
		want   domain.Classification
	}{
		{name: "APPROVED signal", code: 1, stderr: "APPROVED: looks great", want: domain.ClassificationOK},
		{name: "BLOCKED signal", code: 0, stderr: "BLOCKED: need credentials", want: domain.ClassificationBlocked},
		{name: "CHANGES_REQUESTED signal", code: 0, stderr: "CHANGES_REQUESTED: 1. Fix X", want: domain.ClassificationChangesRequested},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyResult(tt.code, tt.stderr)
			if got != tt.want {
				t.Errorf("classifyResult(%d, %q) = %q, want %q", tt.code, tt.stderr, got, tt.want)
			}
		})
	}
}
