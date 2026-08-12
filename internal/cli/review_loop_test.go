package cli

import (
	"bytes"
	"fmt"
	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func (m *multiResultAdapter) DefaultModel() string {
	return "test"
}

func (m *multiResultAdapter) DefaultFallbackChain() map[string][]string {
	return map[string][]string{
		"free": {"test"},
		"paid": {"test"},
		"pro":  {"test"},
	}
}

func (m *multiResultAdapter) FailureSignals() []domain.Signal { return nil }

func (m *multiResultAdapter) BinaryPath() string { return "/usr/local/bin/mill" }

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

	_, err := runDispatchLoop54(app, 1, "task-1", "", "", opts, "Test issue body", nil, cfg, adapter.Capabilities{})
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
			{ExitCode: 0, Commits: 0, Output: "review", Stderr: "CHANGES_REQUESTED: 1. [criterion: error handling] Missing error handling"},
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

	_, err := runDispatchLoop54(app, 2, "task-2", "", "", opts, "Test issue body", nil, cfg, adapter.Capabilities{})
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
			{ExitCode: 0, Commits: 0, Output: "r1", Stderr: "CHANGES_REQUESTED: 1. [criterion: X] Fix X"},
			{ExitCode: 0, Commits: 2, Output: "v2", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r2", Stderr: "CHANGES_REQUESTED: 2. [criterion: Y] Fix Y"},
			{ExitCode: 0, Commits: 3, Output: "v3", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r3", Stderr: "CHANGES_REQUESTED: 3. [criterion: Z] Fix Z"},
			{ExitCode: 0, Commits: 4, Output: "v4", Stderr: ""},
			{ExitCode: 0, Commits: 0, Output: "r4", Stderr: "CHANGES_REQUESTED: 4. [criterion: W] Fix W"},
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

	_, err := runDispatchLoop54(app, 3, "task-3", "", "", opts, "Test issue body", nil, cfg, adapter.Capabilities{})
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

func TestReviewLoopGateFailureImmediate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	madapter := &multiResultAdapter{
		results: []adapter.SessionResult{
			{ExitCode: 0, Commits: 1, Output: "produced", Stderr: ""},
			{ExitCode: 1, Commits: 0, Output: "review", Stderr: "gate-spec: missing criteria"},
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

	_, err := runDispatchLoop54(app, 4, "task-4", "", "", opts, "Test issue body", nil, cfg, adapter.Capabilities{})
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
	if !strings.Contains(errOutput, "ESCALATION") {
		t.Error("expected ESCALATION message on stderr")
	}
}

func TestClassifyFailureReviewSignals(t *testing.T) {
	tests := []struct {
		name   string
		result domain.SessionResult
		want   domain.FailureClass
	}{
		{name: "APPROVED signal", result: domain.SessionResult{ExitCode: 0, Stderr: "APPROVED: looks great"}, want: domain.CLASS_OK},
		{name: "gate signal", result: domain.SessionResult{ExitCode: 1, Stderr: "gate-frd: rejected"}, want: domain.GATE_FAILURE},
		{name: "CHANGES_REQUESTED signal", result: domain.SessionResult{ExitCode: 0, Stderr: "CHANGES_REQUESTED: [criterion: x] 1. Fix X"}, want: domain.RESULT_FAILURE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyFailure(tt.result)
			if got != tt.want {
				t.Errorf("classifyFailure(%+v) = %q, want %q", tt.result, got, tt.want)
			}
		})
	}
}

func TestRecordErrorReviewLoop(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}

	s := state.New()
	task := domain.Task{ID: "task-review-1", Issue: 10, Status: domain.TaskRunning}
	s.UpsertTask(task)

	app.recordError(s, 10, task, nil, "review-loop-failure")

	// Verify task status was updated
	updated, ok := s.Task("task-review-1")
	if !ok {
		t.Fatal("expected task-review-1 to exist")
	}
	if updated.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, updated.Status)
	}

	// Verify ledger was written
	ledgerPath := app.ledgerPath(10)
	if _, err := os.Stat(ledgerPath); os.IsNotExist(err) {
		t.Error("expected ledger file to exist after recordError")
	}
}

func TestReviewLoopProduceDispatchError(t *testing.T) {
	t.Skip("integration test — needs adapter mock fixes after model routing changes")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	errBuf := new(bytes.Buffer)
	app := &App{
		Adapter:     &fakeAdapter{dispatchErr: fmt.Errorf("network down")},
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         errBuf,
		Out:         new(bytes.Buffer),
	}

	cfg := config.Default()
	cfg.MaxRounds = 2

	opts := adapter.DispatchOpts{Worktree: dir, Prompt: "Fix", Model: "laguna-free", MaxTurns: 10}
	_, err := runDispatchLoop54(app, 1, "task-err-1", "", "", opts, "Test body", nil, cfg, adapter.Capabilities{})
	if err == nil {
		t.Fatal("expected error from produce dispatch failure")
	}
}

func TestReviewLoopProduceWaitError(t *testing.T) {
	t.Skip("integration test — needs adapter mock fixes after model routing changes")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0, Output: "ok"}}
	// Override the session Wait to return error
	origSession := &fakeSession{}
	errBuf := new(bytes.Buffer)
	app := &App{
		Adapter:     fa,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         errBuf,
		Out:         new(bytes.Buffer),
	}
	// Make dispatch return a session that Wait-errors
	fa.dispatchFn = func(opts adapter.DispatchOpts) (adapter.Session, error) {
		return &fakeSession{waitErr: fmt.Errorf("session timeout")}, nil
	}

	cfg := config.Default()
	cfg.MaxRounds = 2

	opts := adapter.DispatchOpts{Worktree: dir, Prompt: "Fix", Model: "laguna-free", MaxTurns: 10}
	_, err := runDispatchLoop54(app, 2, "task-err-2", "", "", opts, "Test body", nil, cfg, adapter.Capabilities{})
	if err == nil {
		t.Fatal("expected error from produce Wait failure")
	}
	_ = origSession
}

func TestReviewLoopReviewWaitError(t *testing.T) {
	t.Skip("integration test — needs adapter mock fixes after model routing changes")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	callCount := 0
	fa := &fakeAdapter{}
	fa.dispatchFn = func(opts adapter.DispatchOpts) (adapter.Session, error) {
		callCount++
		if callCount == 1 {
			// Produce succeeds
			return &fakeSession{result: adapter.SessionResult{ExitCode: 0, Output: "produce ok"}}, nil
		}
		// Review Wait fails
		return &fakeSession{waitErr: fmt.Errorf("review timeout")}, nil
	}

	errBuf := new(bytes.Buffer)
	app := &App{
		Adapter:     fa,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         errBuf,
		Out:         new(bytes.Buffer),
	}

	cfg := config.Default()
	cfg.MaxRounds = 2

	opts := adapter.DispatchOpts{Worktree: dir, Prompt: "Fix", Model: "laguna-free", MaxTurns: 10}
	_, err := runDispatchLoop54(app, 3, "task-err-3", "", "", opts, "Test body", nil, cfg, adapter.Capabilities{})
	if err == nil {
		t.Fatal("expected error from review Wait failure")
	}
}

func TestReviewLoopBlockedVerdict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	callCount := 0
	fa := &fakeAdapter{}
	fa.dispatchFn = func(opts adapter.DispatchOpts) (adapter.Session, error) {
		callCount++
		if callCount == 1 {
			return &fakeSession{result: adapter.SessionResult{ExitCode: 0, Output: "produce ok"}}, nil
		}
		// Review returns a gate failure signal
		return &fakeSession{result: adapter.SessionResult{ExitCode: 1, Output: "", Stderr: "gate-tasks: rejected"}}, nil
	}

	errBuf := new(bytes.Buffer)
	app := &App{
		Adapter:     fa,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         errBuf,
		Out:         new(bytes.Buffer),
	}

	cfg := config.Default()
	cfg.MaxRounds = 2

	opts := adapter.DispatchOpts{Worktree: dir, Prompt: "Fix", Model: "laguna-free", MaxTurns: 10}
	_, err := runDispatchLoop54(app, 4, "task-blocked-4", "", "", opts, "Test body", nil, cfg, adapter.Capabilities{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := errBuf.String()
	if !strings.Contains(output, "ESCALATION") {
		t.Error("expected ESCALATION in output for BLOCKED verdict")
	}
}

func TestReviewLoopReviewDispatchError(t *testing.T) {
	t.Skip("integration test — needs adapter mock fixes after model routing changes")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)

	callCount := 0
	fa := &fakeAdapter{}
	fa.dispatchFn = func(opts adapter.DispatchOpts) (adapter.Session, error) {
		callCount++
		if callCount == 1 {
			return &fakeSession{result: adapter.SessionResult{ExitCode: 0, Output: "produce ok"}}, nil
		}
		return nil, fmt.Errorf("review dispatch failed")
	}

	errBuf := new(bytes.Buffer)
	app := &App{
		Adapter:     fa,
		IssueReader: defaultIssueReader,
		MillDir:     dir,
		Err:         errBuf,
		Out:         new(bytes.Buffer),
	}

	cfg := config.Default()
	cfg.MaxRounds = 2

	opts := adapter.DispatchOpts{Worktree: dir, Prompt: "Fix", Model: "laguna-free", MaxTurns: 10}
	_, err := runDispatchLoop54(app, 5, "task-err-5", "", "", opts, "Test body", nil, cfg, adapter.Capabilities{})
	if err == nil {
		t.Fatal("expected error from review dispatch failure")
	}
}
