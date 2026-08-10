package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
)

// fakeAdapter implements adapter.Adapter for CLI testing.
type fakeAdapter struct {
	dispatched bool
	opts       adapter.DispatchOpts
	result     adapter.SessionResult
	dispatchErr error
}

func (f *fakeAdapter) Dispatch(opts adapter.DispatchOpts) (adapter.Session, error) {
	f.dispatched = true
	f.opts = opts
	if f.dispatchErr != nil {
		return nil, f.dispatchErr
	}
	return &fakeSession{result: f.result}, nil
}

func (f *fakeAdapter) Resume(sessionID string) (adapter.Session, error) {
	return &fakeSession{result: f.result}, nil
}

func (f *fakeAdapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Models: []string{"gpt-5", "deepseek-v4-pro"}}
}

type fakeSession struct {
	result adapter.SessionResult
}

func (s *fakeSession) ID() string      { return "fake-session-1" }
func (s *fakeSession) Status() string  { return "done" }
func (s *fakeSession) Wait() (adapter.SessionResult, error) {
	return s.result, nil
}

func TestDelegateValidIssueDispatchesAndRecords(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  2,
			Output:   "APPROVED - task complete",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "390")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Verify adapter was called with correct DispatchOpts
	if !fa.dispatched {
		t.Error("expected adapter Dispatch to be called")
	}
	if fa.opts.Prompt == "" {
		t.Error("expected non-empty prompt")
	}
	if fa.opts.Model == "" {
		t.Error("expected model to be set")
	}
	if fa.opts.MaxTurns != 100 {
		t.Errorf("expected max turns 100, got %d", fa.opts.MaxTurns)
	}

	// Verify worktree path includes the issue number
	if !strings.Contains(fa.opts.Worktree, "issue-390") {
		t.Errorf("expected worktree to contain issue-390, got %q", fa.opts.Worktree)
	}

	// Verify state
	s, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task, ok := s.Task("task-390")
	if !ok {
		t.Fatal("expected task-390 to exist")
	}
	if task.Status != domain.TaskDone {
		t.Errorf("expected status %q, got %q", domain.TaskDone, task.Status)
	}
	if task.Verdict != domain.VerdictApproved {
		t.Errorf("expected verdict %q, got %q", domain.VerdictApproved, task.Verdict)
	}
	if task.Commits != 2 {
		t.Errorf("expected commits 2, got %d", task.Commits)
	}

	// Verify started/updated timestamps
	if task.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
	if task.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestDelegateNoArgsReturnsError(t *testing.T) {
	app := &App{MillDir: t.TempDir()}
	err := app.Run("delegate")
	if err == nil {
		t.Fatal("delegate with no args should return error")
	}
}

func TestDelegateInvalidIssueReturnsError(t *testing.T) {
	app := &App{MillDir: t.TempDir()}
	err := app.Run("delegate", "abc")
	if err == nil {
		t.Fatal("delegate with invalid issue should return error")
	}
}

func TestDelegateExitCodeErrorSetsTaskError(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 3,
			Commits:  0,
			Output:   "REJECTED - something went wrong",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "42")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	s, _ := state.Load(app.statePath())
	task, ok := s.Task("task-42")
	if !ok {
		t.Fatal("expected task-42 to exist")
	}
	if task.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, task.Status)
	}
}

func TestDelegateCreatesLedgerEntry(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  1,
			Output:   "NEEDS CHANGES - please fix the formatting",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "7")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	ledgerFile := app.ledgerPath(7)
	if _, err := os.Stat(ledgerFile); os.IsNotExist(err) {
		t.Fatal("expected ledger file to be created")
	}

	content, err := os.ReadFile(ledgerFile)
	if err != nil {
		t.Fatalf("failed to read ledger: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 ledger entries, got %d", len(lines))
	}

	// First entry: dispatch
	if !strings.Contains(lines[0], "dispatch") {
		t.Errorf("expected first entry to be dispatch, got: %s", lines[0])
	}
	// Second entry: complete
	if !strings.Contains(lines[1], "complete") {
		t.Errorf("expected second entry to be complete, got: %s", lines[1])
	}
}

func TestDelegateDispatchErrorRecordsError(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{
		dispatchErr: assertError("binary not found"),
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "99")
	if err == nil {
		t.Fatal("expected error from dispatch failure")
	}

	s, _ := state.Load(app.statePath())
	task, ok := s.Task("task-99")
	if !ok {
		t.Fatal("expected task-99 to exist")
	}
	if task.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, task.Status)
	}
}

func TestDelegateModelFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
			Output:   "APPROVED",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "-model", "deepseek-v4-pro", "555")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	if fa.opts.Model != "deepseek-v4-pro" {
		t.Errorf("expected model %q, got %q", "deepseek-v4-pro", fa.opts.Model)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
