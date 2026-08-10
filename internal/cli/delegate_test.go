package cli

import (
	"bytes"
	"os"
	"path/filepath"
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
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 ledger entries, got %d", len(lines))
	}

	// First entry: dispatch
	if !strings.Contains(lines[0], "dispatch") {
		t.Errorf("expected first entry to be dispatch, got: %s", lines[0])
	}
	// Second entry: classify
	if !strings.Contains(lines[1], "classify") {
		t.Errorf("expected second entry to be classify, got: %s", lines[1])
	}
	// Third entry: complete
	if !strings.Contains(lines[2], "complete") {
		t.Errorf("expected third entry to be complete, got: %s", lines[2])
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

func TestInstallHooksCreatesDirAndCopiesFiles(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(origDir, "..", "..")
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	worktree := t.TempDir()
	if err := installHooks(worktree); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Verify .git/hooks directory was created.
	hookDir := filepath.Join(worktree, ".git", "hooks")
	info, err := os.Stat(hookDir)
	if os.IsNotExist(err) {
		t.Fatal("expected .git/hooks directory to be created")
	}
	if !info.IsDir() {
		t.Fatal("expected .git/hooks to be a directory")
	}

	// Verify common.sh was copied as "common" (extension stripped).
	commonHook := filepath.Join(hookDir, "common")
	if _, err := os.Stat(commonHook); os.IsNotExist(err) {
		t.Error("expected common hook to be copied")
	}

	// Verify content matches the original.
	originalContent, err := os.ReadFile(filepath.Join("checks", "common.sh"))
	if err != nil {
		t.Fatalf("failed to read original common.sh: %v", err)
	}
	copiedContent, err := os.ReadFile(commonHook)
	if err != nil {
		t.Fatalf("failed to read copied common: %v", err)
	}
	if !bytes.Equal(originalContent, copiedContent) {
		t.Error("expected copied common to match original common.sh content")
	}
}

func TestDelegateStaffToSrDevRejected(t *testing.T) {
	dir := t.TempDir()
	// Write .mill/role as staff
	if err := os.MkdirAll(filepath.Join(dir, ".mill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mill", "role"), []byte("staff"), 0644); err != nil {
		t.Fatal(err)
	}


	fa := &fakeAdapter{}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: filepath.Join(dir, ".mill"), Out: buf, Err: buf}

	err := app.Run("delegate", "41", "--role", "sr-dev-be")
	if err == nil {
		t.Fatal("expected delegation rejection")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "staff delegates to") {
		t.Errorf("expected delegation chain error, got: %v", errMsg)
	}
	if !strings.Contains(errMsg, "not sr-dev-be") {
		t.Errorf("expected mention of sr-dev-be, got: %v", errMsg)
	}
}

func TestDelegateStaffToArchitectAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mill", "role"), []byte("staff"), 0644); err != nil {
		t.Fatal(err)
	}

	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  1,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: filepath.Join(dir, ".mill"), Out: buf, Err: buf}

	err := app.Run("delegate", "390", "--role", "architect")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	if !fa.dispatched {
		t.Error("expected adapter Dispatch to be called")
	}
}

func TestDelegateNoRoleUsesActiveRole(t *testing.T) {
	dir := t.TempDir()
	// No .mill/role file → defaults to staff, no delegation validation
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "1")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	if !fa.dispatched {
		t.Error("expected adapter Dispatch to be called")
	}
}

func TestDelegateScaffoldsWorktree(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "7")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	wt := app.worktreePath(7)

	// Verify scaffold files exist
	for _, rel := range []string{
		"AGENTS.md",
		filepath.Join(".omp", "AGENTS.md"),
		filepath.Join(".omp", "RULES.md"),
		filepath.Join("roles", "COMMON.md"),
	} {
		p := filepath.Join(wt, rel)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected scaffold file %s to exist", rel)
		}
	}

	// Verify .mill/role content
	roleFile := filepath.Join(wt, ".mill", "role")
	data, err := os.ReadFile(roleFile)
	if err != nil {
		t.Fatalf("failed to read .mill/role: %v", err)
	}
	if string(data) != "staff" {
		t.Errorf("expected .mill/role to be 'staff', got %q", string(data))
	}
}
