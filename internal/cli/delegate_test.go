package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/recursion"
	"github.com/antonygiomarxdev/mill/internal/state"
)

type fakeAdapter struct {
	dispatched  bool
	opts        adapter.DispatchOpts
	allOpts     []adapter.DispatchOpts
	result      adapter.SessionResult
	dispatchErr error
	// dispatchFn, when non-nil, is called instead of returning result/dispatchErr.
	// Useful for sequencing different results across retry calls.
	dispatchFn func(opts adapter.DispatchOpts) (adapter.Session, error)
}

func (f *fakeAdapter) Dispatch(opts adapter.DispatchOpts) (adapter.Session, error) {
	f.dispatched = true
	f.opts = opts
	f.allOpts = append(f.allOpts, opts)
	if f.dispatchFn != nil {
		return f.dispatchFn(opts)
	}
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

func (f *fakeAdapter) DefaultModel() string {
	return "gpt-5"
}

func (f *fakeAdapter) DefaultFallbackChain() map[string][]string {
	return map[string][]string{
		"free":   {"gpt-5", "deepseek-v4-pro", "gpt-5", "deepseek-v4-pro", "gpt-5", "deepseek-v4-pro", "gpt-5"},
		"paid":   {"deepseek-v4-pro", "gpt-5"},
		"pro":    {"deepseek-v4-pro", "gpt-5"},
		"review": {"deepseek-v4-pro", "gpt-5"},
	}
}

func (f *fakeAdapter) FailureSignals() []domain.Signal { return nil }

func (f *fakeAdapter) BinaryPath() string { return "/usr/local/bin/mill" }

type fakeSession struct {
	result  adapter.SessionResult
	waitErr error
	ctxText string
	ctxErr  error
}

func (s *fakeSession) ID() string     { return "fake-session-1" }
func (s *fakeSession) Status() string { return "done" }
func (s *fakeSession) Wait() (adapter.SessionResult, error) {
	if s.waitErr != nil {
		return adapter.SessionResult{}, s.waitErr
	}
	return s.result, nil
}

func (s *fakeSession) ContextText() (string, error) {
	if s.ctxErr != nil {
		return "", s.ctxErr
	}
	if s.ctxText != "" {
		return s.ctxText, nil
	}
	return "", nil
}

func (s *fakeSession) HeartbeatPath() string { return ".mill/heartbeat" }

// transientResult returns a SessionResult that classifies as TRANSIENT.
func transientResult() adapter.SessionResult {
	return adapter.SessionResult{ExitCode: 6, Output: "transient", Stderr: "connection refused"}
}

// okResult returns a SessionResult that classifies as OK.
func okResult() adapter.SessionResult {
	return adapter.SessionResult{ExitCode: 0, Output: "APPROVED", Stderr: ""}
}

// fatalResult returns a SessionResult that classifies as FATAL.
func fatalResult() adapter.SessionResult {
	return adapter.SessionResult{ExitCode: 4, Output: "fatal", Stderr: "fatal"}
}

// defaultIssueReader returns a stub issue body and empty labels for testing.
func defaultIssueReader(issueNum int) (string, []string, error) {
	return "Test issue body content", nil, nil
}

func TestDelegateValidIssueDispatchesAndRecords(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  2,
			Output:   "APPROVED - task complete",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "390")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Verify adapter was called with correct DispatchOpts
	if !fa.dispatched {
		t.Error("expected adapter Dispatch to be called")
	}
	if len(fa.allOpts) < 2 {
		t.Fatalf("expected at least 2 dispatches (produce + review), got %d", len(fa.allOpts))
	}
	// Produce prompt should be non-empty
	if fa.allOpts[0].Prompt == "" {
		t.Error("expected non-empty produce prompt")
	}
	// Review model should be laguna-pro
	if fa.allOpts[1].Model != "deepseek-v4-pro" {
		t.Errorf("expected review model deepseek-v4-pro, got %q", fa.allOpts[1].Model)
	}
	if fa.allOpts[0].MaxTurns != 100 {
		t.Errorf("expected max turns 100, got %d", fa.allOpts[0].MaxTurns)
	}

	// Verify worktree path includes the issue number
	if !strings.Contains(fa.allOpts[0].Worktree, "issue-390") {
		t.Errorf("expected worktree to contain issue-390, got %q", fa.allOpts[0].Worktree)
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
	if task.Round != 0 {
		t.Errorf("expected round 0, got %d", task.Round)
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
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 137,
			Commits:  0,
			Output:   "REJECTED - something went wrong",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "42")
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
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  1,
			Output:   "NEEDS CHANGES - please fix the formatting",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "7")
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
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 ledger entries, got %d", len(lines))
	}

	// First entry: dispatch
	if !strings.Contains(lines[0], "dispatch") {
		t.Errorf("expected first entry to be dispatch, got: %s", lines[0])
	}
	// Second entry: produce (added by review loop)
	if !strings.Contains(lines[1], "produce") {
		t.Errorf("expected second entry to be produce, got: %s", lines[1])
	}
	// Third entry: review
	if !strings.Contains(lines[2], "review") {
		t.Errorf("expected third entry to be review, got: %s", lines[2])
	}
	// Fourth entry: complete
	if !strings.Contains(lines[3], "complete") {
		t.Errorf("expected fourth entry to be complete, got: %s", lines[3])
	}
}

func TestDelegateDispatchErrorRecordsError(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		dispatchErr: assertError("binary not found"),
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "99")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
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
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
			Output:   "APPROVED",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "-model", "pro", "555")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Produce dispatch uses resolved model from tier override
	if fa.allOpts[0].Model != "deepseek-v4-pro" {
		t.Errorf("expected produce model %q, got %q", "deepseek-v4-pro", fa.allOpts[0].Model)
	}
	// Review dispatch also uses the same flag override
	if fa.allOpts[1].Model != "deepseek-v4-pro" {
		t.Errorf("expected review model %q, got %q", "deepseek-v4-pro", fa.allOpts[1].Model)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }

func TestInstallHooksInRealWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Create a real git worktree
	wt := filepath.Join(dir, "worktree-test")
	runGit(t, dir, "worktree", "add", wt)

	// Ensure .mill/checks exists in the worktree (needed for gate script loop)
	checksDir := filepath.Join(wt, ".mill", "checks")
	if err := os.MkdirAll(checksDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Verify .mill/hooks/pre-commit exists with 0755
	hookPath := filepath.Join(wt, ".mill", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		t.Fatal("expected pre-commit hook to be installed")
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected pre-commit hook to be 0755, got %o", info.Mode().Perm())
	}

	// Verify content matches the preCommitHookScript constant
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read pre-commit hook: %v", err)
	}
	if string(data) != preCommitHookScript {
		t.Error("pre-commit hook content does not match expected script")
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
	app := &App{Adapter: fa, MillDir: filepath.Join(dir, ".mill"), Out: buf, Err: buf, IssueReader: defaultIssueReader}

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
	setupTestGitRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".mill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mill", "role"), []byte("staff"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create role directory so validateDelegation can read ROLE.md
	roleDir := filepath.Join(dir, ".mill", "roles", "staff")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\ndelegates_to:\n  - architect\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  1,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: filepath.Join(dir, ".mill"), Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "390", "--role", "architect")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	if !fa.dispatched {
		t.Error("expected adapter Dispatch to be called")
	}
}

func TestDelegateNoRoleUsesActiveRole(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	// No .mill/role file → defaults to staff, no delegation validation
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "1")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	if !fa.dispatched {
		t.Error("expected adapter Dispatch to be called")
	}
}

func TestDelegateScaffoldsWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "7")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	wt := app.worktreePath(7)

	// Verify scaffold files exist
	for _, rel := range []string{
		filepath.Join(".omp", "AGENTS.md"),
		filepath.Join(".omp", "RULES.md"),
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

func TestClassifyFailure(t *testing.T) {
	tests := []struct {
		name   string
		result domain.SessionResult
		want   domain.FailureClass
	}{
		{name: "exit 0 with no signal is CLASS_OK", result: domain.SessionResult{ExitCode: 0}, want: domain.CLASS_OK},
		{name: "exit 0 with placeholder output is CONTRACT", result: domain.SessionResult{ExitCode: 0, Output: "TODO: implement"}, want: domain.CONTRACT_FAILURE},
		{name: "exit 0 with whitespace output is CONTRACT", result: domain.SessionResult{ExitCode: 0, Output: "   "}, want: domain.CONTRACT_FAILURE},
		{name: "exit 4 is EXECUTION_FAILURE", result: domain.SessionResult{ExitCode: 4}, want: domain.EXECUTION_FAILURE},
		{name: "exit 137 is EXECUTION_FAILURE", result: domain.SessionResult{ExitCode: 137}, want: domain.EXECUTION_FAILURE},
		{name: "exit -1 with blocked stderr is EXECUTION_FAILURE", result: domain.SessionResult{ExitCode: -1, Stderr: "blocked: time budget exceeded"}, want: domain.EXECUTION_FAILURE},
		{name: "exit 1 with gate stderr is GATE_FAILURE", result: domain.SessionResult{ExitCode: 1, Stderr: "gate-spec: missing criteria"}, want: domain.GATE_FAILURE},
		{name: "changes_requested with criterion is RESULT_FAILURE", result: domain.SessionResult{ExitCode: 0, Stderr: "CHANGES_REQUESTED: [criterion: x] fix"}, want: domain.RESULT_FAILURE},
		{name: "connection refused is EXECUTION_FAILURE", result: domain.SessionResult{ExitCode: 1, Stderr: "connection refused"}, want: domain.EXECUTION_FAILURE},
		{name: "network timeout is EXECUTION_FAILURE", result: domain.SessionResult{ExitCode: 1, Stderr: "network timeout"}, want: domain.EXECUTION_FAILURE},
		{name: "env error is ENVIRONMENT_FAILURE", result: domain.SessionResult{EnvError: fmt.Errorf("git not found")}, want: domain.ENVIRONMENT_FAILURE},
		{name: "stale heartbeat with active process is EXECUTION_FAILURE", result: domain.SessionResult{ProcessActive: true, HeartbeatStaleness: time.Minute}, want: domain.EXECUTION_FAILURE},
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

func TestDelegateBlockedExitCodeSetsTaskError(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{name: "exit -1 budget_time", exitCode: -1},
		{name: "exit -2 budget_turns", exitCode: -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			setupTestGitRepo(t, dir)
			fa := &fakeAdapter{
				result: adapter.SessionResult{
					ExitCode: tt.exitCode,
					Commits:  0,
					Output:   "BLOCKED — budget exceeded",
					Stderr:   "blocked: time budget exceeded",
				},
			}
			buf := new(bytes.Buffer)
			app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

			err := app.Run("delegate", "--wait", "53")
			if err != nil {
				t.Fatalf("delegate returned error: %v", err)
			}

			s, _ := state.Load(app.statePath())
			task, ok := s.Task("task-53")
			if !ok {
				t.Fatal("expected task-53 to exist")
			}
			if task.Status != domain.TaskError {
				t.Errorf("expected status %q, got %q", domain.TaskError, task.Status)
			}

			// Verify ledger records the execution failure class
			ledgerFile := app.ledgerPath(53)
			content, err := os.ReadFile(ledgerFile)
			if err != nil {
				t.Fatalf("failed to read ledger: %v", err)
			}
			if !strings.Contains(string(content), string(domain.EXECUTION_FAILURE)) {
				t.Error("expected ledger to contain EXECUTION_FAILURE classification")
			}
		})
	}
}

func TestResolveModelMissingRoleFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "laguna-free", Models: map[string]string{
		"free": "laguna-free", "paid": "laguna-pro", "pro": "laguna-ultra",
	}}
	// No role file → falls through to tier="paid" → "laguna-pro"
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-pro" {
		t.Errorf("expected 'laguna-pro' (default paid tier), got %q", got)
	}
}

func TestResolveModelEmptyModelTier(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel:\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "laguna-free", Models: map[string]string{
		"paid": "laguna-pro", "pro": "laguna-ultra",
	}}
	// Empty model in role → falls through to tier="paid" → "laguna-pro"
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-pro" {
		t.Errorf("expected 'laguna-pro' (default paid tier), got %q", got)
	}
}

func TestResolveModelKnownTier(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: paid\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "laguna-free", Models: map[string]string{
		"free": "laguna-free", "paid": "laguna-pro", "pro": "laguna-ultra",
	}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-pro" {
		t.Errorf("expected 'laguna-pro' for paid tier, got %q", got)
	}
}

func TestDelegateStageLabelToModelProduce(t *testing.T) {
	got := stageLabelToModel("stage:produce")
	if got != "laguna-free" {
		t.Errorf("expected stage:produce → laguna-free, got %q", got)
	}
}

func TestDelegateStageLabelToModelReview(t *testing.T) {
	got := stageLabelToModel("stage:review")
	if got != "laguna-pro" {
		t.Errorf("expected stage:review → laguna-pro, got %q", got)
	}
}

func TestDelegateStageLabelToModelImplement(t *testing.T) {
	got := stageLabelToModel("stage:implement")
	if got != "laguna-free" {
		t.Errorf("expected stage:implement → laguna-free, got %q", got)
	}
}

func TestBuildRolePromptWithSkills(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte(
		"---\nrole: sr-dev-be\nmodel: paid\nskills:\n  - tdd\n  - code-review\n---\n\n# Sr Dev\n",
	), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	result := buildRolePrompt(1, "sr-dev-be", "")
	if !strings.Contains(result, "tdd") {
		t.Error("expected output to contain 'tdd'")
	}
	if !strings.Contains(result, "code-review") {
		t.Error("expected output to contain 'code-review'")
	}
}

func TestBuildRolePromptNoSkills(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte(
		"---\nrole: sr-dev-be\nmodel: paid\n---\n\n# Sr Dev\n",
	), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	result := buildRolePrompt(1, "sr-dev-be", "")
	if !strings.Contains(result, "1") {
		t.Error("expected output to contain issue number '1'")
	}
	if !strings.Contains(result, "sr-dev-be") {
		t.Error("expected output to contain role name 'sr-dev-be'")
	}
}

func TestBuildRolePromptWithBody(t *testing.T) {
	result := buildRolePrompt(42, "sr-dev-be", "Fix the login button")
	if !strings.Contains(result, "**Issue Body:**") {
		t.Error("expected prompt to contain Issue Body section")
	}
	if !strings.Contains(result, "Fix the login button") {
		t.Error("expected prompt to contain the issue body text")
	}
}

func TestReadActiveRoleError(t *testing.T) {
	dir := t.TempDir()
	// Create role as a directory so os.ReadFile fails
	rolePath := filepath.Join(dir, "role")
	os.MkdirAll(rolePath, 0o755)

	app := &App{MillDir: dir}
	got := app.readActiveRole()
	if got != "staff" {
		t.Errorf("expected fallback to 'staff', got %q", got)
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	result := buildReviewPrompt54("Fix the login", "PATCH: done", "", "", nil, adapter.Capabilities{})
	if !strings.Contains(result, "APPROVED:") {
		t.Error("expected review prompt to mention APPROVED:")
	}
	if !strings.Contains(result, "CHANGES_REQUESTED:") {
		t.Error("expected review prompt to mention CHANGES_REQUESTED:")
	}
	if !strings.Contains(result, "BLOCKED:") {
		t.Error("expected review prompt to mention BLOCKED:")
	}
	if !strings.Contains(result, "Fix the login") {
		t.Error("expected review prompt to contain issue body")
	}
	if !strings.Contains(result, "PATCH: done") {
		t.Error("expected review prompt to contain produce output")
	}
}

// --- Worktree creation and cleanup tests ---

func TestCreateWorktreeSuccess(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}
	wt, err := app.createWorktree(72)
	if err != nil {
		t.Fatalf("createWorktree failed: %v", err)
	}

	// AC1: worktree exists
	if _, err := os.Stat(wt); os.IsNotExist(err) {
		t.Fatal("worktree directory does not exist")
	}

	// AC3: .git is a file (pointer), not a directory
	gitFile := filepath.Join(wt, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		t.Fatalf("cannot stat .git in worktree: %v", err)
	}
	if info.IsDir() {
		t.Error(".git in worktree is a directory, expected a file")
	}

	// AC2: branch agent/72 exists
	cmd := exec.Command("git", "branch", "--list", "agent/72")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if !strings.Contains(string(out), "agent/72") {
		t.Error("expected branch agent/72 to exist")
	}

	// PID file exists
	pidFile := filepath.Join(wt, ".mill", "agent.pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Error("PID file does not exist")
	}
}

func TestCreateWorktreeCleanup(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}
	wt, err := app.createWorktree(42)
	if err != nil {
		t.Fatalf("createWorktree failed: %v", err)
	}

	// Cleanup
	app.cleanupWorktree(42)

	// AC6: worktree directory removed
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed after cleanup")
	}

	// Branch removed
	cmd := exec.Command("git", "branch", "--list", "agent/42")
	out, _ := cmd.Output()
	if strings.Contains(string(out), "agent/42") {
		t.Error("branch agent/42 should be deleted after cleanup")
	}
}

func TestCreateWorktreeIdempotent(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	// First creation
	wt1, err := app.createWorktree(55)
	if err != nil {
		t.Fatalf("first createWorktree failed: %v", err)
	}

	// Simulate crash: don't call cleanup, just try creating again
	wt2, err := app.createWorktree(55)
	if err != nil {
		t.Fatalf("second createWorktree (idempotent) failed: %v", err)
	}

	// Both worktrees should be valid worktrees
	if _, err := os.Stat(filepath.Join(wt2, ".git")); err != nil {
		t.Error("second worktree does not have .git file")
	}

	// The second creation should have cleaned up the stale worktree
	// and created a fresh one
	if wt1 != wt2 {
		// Paths should be the same (same issue number)
		t.Logf("wt1=%s wt2=%s", wt1, wt2)
	}
}

func TestCreateWorktreeConcurrentIsolation(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	// AC5: create two worktrees for different issues
	wt1, err := app.createWorktree(10)
	if err != nil {
		t.Fatalf("createWorktree for issue 10 failed: %v", err)
	}
	wt2, err := app.createWorktree(20)
	if err != nil {
		t.Fatalf("createWorktree for issue 20 failed: %v", err)
	}

	// Different branches
	cmd := exec.Command("git", "branch", "--list")
	out, _ := cmd.Output()
	branches := string(out)
	if !strings.Contains(branches, "agent/10") || !strings.Contains(branches, "agent/20") {
		t.Error("expected both agent/10 and agent/20 branches to exist")
	}

	// Create a file in wt1, verify it's not visible in wt2
	testFile := filepath.Join(wt1, "isolation-test.txt")
	if err := os.WriteFile(testFile, []byte("hello from 10"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if _, err := os.Stat(filepath.Join(wt2, "isolation-test.txt")); !os.IsNotExist(err) {
		t.Error("file created in worktree 10 is visible in worktree 20 — isolation broken")
	}
}

func TestCreateWorktreeNoGitBinary(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	// Run with empty PATH to simulate missing git
	t.Setenv("PATH", "")

	_, err := app.createWorktree(99)
	if err == nil {
		t.Fatal("expected error when git is not available")
	}
	if !strings.Contains(err.Error(), "git not found") {
		t.Errorf("expected 'git not found' error, got: %v", err)
	}
}

func TestDelegateWithRealWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  1,
			Output:   "APPROVED",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "--wait", "8")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// AC1: worktree is a real git checkout
	wt := app.worktreePath(8)
	gitFile := filepath.Join(wt, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		t.Fatalf("worktree .git does not exist: %v", err)
	}
	if info.IsDir() {
		t.Error(".git in worktree is a directory, expected a file (worktree pointer)")
	}

	// Can run git status inside worktree
	cmd := exec.Command("git", "-C", wt, "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status in worktree failed: %v\n%s", err, out)
	}
}

func TestLandAfterRealWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}
	wt, err := app.createWorktree(72)
	if err != nil {
		t.Fatalf("createWorktree failed: %v", err)
	}

	// Make a commit in the worktree
	commitFile := filepath.Join(wt, "feature.txt")
	if err := os.WriteFile(commitFile, []byte("worktree feature"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", wt, "add", "feature.txt")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", wt, "commit", "-m", "feature from worktree")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// AC4: git -C worktree checkout main works
	cmd = exec.Command("git", "-C", wt, "checkout", "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout main from worktree failed: %v\n%s", err, out)
	}
}

func TestInstallHooksRejectsPlainDir(t *testing.T) {
	// Plain directory without .git file → should error
	dir := t.TempDir()
	err := installHooks(dir)
	if err == nil {
		t.Fatal("expected error for plain directory, got nil")
	}
	if !strings.Contains(err.Error(), "not a git worktree") {
		t.Errorf("expected 'not a git worktree' error, got: %v", err)
	}
}

func TestInstallHooksRejectsGitDir(t *testing.T) {
	// Directory with .git/ subdirectory (regular repo, not worktree) → should error
	dir := t.TempDir()
	t.Chdir(dir)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	err := installHooks(dir)
	if err == nil {
		t.Fatal("expected error for directory with .git/ dir, got nil")
	}
	if !strings.Contains(err.Error(), ".git directory") {
		t.Errorf("expected '.git directory' error, got: %v", err)
	}
}

func TestInstallHooksRejectsMissingGit(t *testing.T) {
	// Directory exists but has no .git at all
	dir := t.TempDir()
	err := installHooks(dir)
	if err == nil {
		t.Fatal("expected error for directory without .git, got nil")
	}
	if !strings.Contains(err.Error(), "not a git worktree") {
		t.Errorf("expected 'not a git worktree' error, got: %v", err)
	}
}

func TestInstallHooksRejectsInvalidGitFile(t *testing.T) {
	// .git exists but doesn't contain "gitdir:" — not a valid worktree reference
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".git"), []byte("not a gitdir ref"), 0644)
	err := installHooks(dir)
	if err == nil {
		t.Fatal("expected error for invalid .git content, got nil")
	}
	if !strings.Contains(err.Error(), "not a valid git worktree reference") {
		t.Errorf("expected 'not a valid git worktree reference' error, got: %v", err)
	}
}

// gitAvailable reports whether the git binary is on PATH.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// runGateLoop executes the pre-commit dispatcher's gate loop (the final
// section of preCommitHookScript) against worktree's .mill/checks directory,
// with the given cwd so the glob resolves inside the worktree.
func runGateLoop(t *testing.T, worktree string, gates []string) (string, error) {
	t.Helper()
	// Cut the loop out of the dispatcher; only the section that matters here
	// is executed, so the tests don't need go build/go vet to succeed.
	start := strings.Index(preCommitHookScript, "# Run additional gate scripts if present")
	if start < 0 {
		t.Fatal("gate loop section not found in preCommitHookScript")
	}
	end := strings.Index(preCommitHookScript[start:], "\necho \"mill: pre-commit passed\"")
	loop := preCommitHookScript[start : start+end]
	if len(gates) > 0 {
		loop += "\n" + strings.Join(gates, "\n")
	}
	cmd := exec.Command("bash", "-c", loop)
	cmd.Dir = worktree
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runRoleEnforce executes the pre-commit dispatcher's role-enforce section
// (the block before the gate loop in preCommitHookScript) against worktree's
// .mill/checks directory, with the given cwd so the checks resolve inside
// the worktree.
func runRoleEnforce(t *testing.T, worktree string) (string, error) {
	t.Helper()
	// Cut the role-enforce block out of the dispatcher; only the section
	// that matters here is executed, so the tests don't need go build/go
	// vet to succeed.
	start := strings.Index(preCommitHookScript, "# Role capability enforcement")
	if start < 0 {
		t.Fatal("role-enforce section not found in preCommitHookScript")
	}
	end := strings.Index(preCommitHookScript[start:], "\n# Run additional gate scripts if present")
	if end < 0 {
		t.Fatal("gate loop marker not found after role-enforce section")
	}
	block := preCommitHookScript[start : start+end]
	cmd := exec.Command("bash", "-c", block)
	cmd.Dir = worktree
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestRoleEnforceFailsCommitWhenScriptExitsOne(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}
	dir := t.TempDir()
	checksDir := filepath.Join(dir, ".mill", "checks")
	if err := os.MkdirAll(checksDir, 0755); err != nil {
		t.Fatal(err)
	}
	enforcePath := filepath.Join(checksDir, "role-enforce")
	if err := os.WriteFile(enforcePath, []byte("#!/bin/bash\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runRoleEnforce(t, dir)
	if err == nil {
		t.Fatalf("role-enforce exiting 1 should fail the hook; output:\n%s", out)
	}
	if !strings.Contains(out, "Running role-enforce") {
		t.Errorf("expected 'Running role-enforce' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "FAIL role-enforce") {
		t.Errorf("expected 'FAIL role-enforce' in output, got:\n%s", out)
	}
}

func TestRoleEnforceLetsCommitProceedWhenScriptExitsZero(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}
	dir := t.TempDir()
	checksDir := filepath.Join(dir, ".mill", "checks")
	if err := os.MkdirAll(checksDir, 0755); err != nil {
		t.Fatal(err)
	}
	enforcePath := filepath.Join(checksDir, "role-enforce")
	if err := os.WriteFile(enforcePath, []byte("#!/bin/bash\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runRoleEnforce(t, dir)
	if err != nil {
		t.Fatalf("role-enforce exiting 0 should let the hook continue: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running role-enforce") {
		t.Errorf("expected 'Running role-enforce' in output, got:\n%s", out)
	}
}

func TestRoleEnforceAbsentDoesNotFailHook(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mill", "checks"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runRoleEnforce(t, dir)
	if err != nil {
		t.Fatalf("missing role-enforce should not fail the hook: %v\n%s", err, out)
	}
	if strings.Contains(out, "Running role-enforce") {
		t.Errorf("expected role-enforce not to run when absent, got:\n%s", out)
	}
}

func TestInstallHooksDoesNotLeakHooksPathToMainRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Create a real git worktree
	wt := filepath.Join(dir, "worktree-hooks")
	runGit(t, dir, "worktree", "add", wt)

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// The main repo's core.hooksPath must still be unset — the worktree
	// setting lives in per-worktree config, not the shared config.
	cmd := exec.Command("git", "-C", dir, "config", "core.hooksPath")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		t.Fatalf("main repo core.hooksPath is set to %q; expected unset", strings.TrimSpace(string(out)))
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("git config core.hooksPath failed unexpectedly: %v", err)
	}

	// And the worktree's own hooksPath must point at its hooks directory.
	cmd = exec.Command("git", "-C", wt, "config", "--worktree", "core.hooksPath")
	cmd.Dir = wt
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("failed to read worktree core.hooksPath: %v", err)
	}
	want := filepath.Join(wt, ".mill", "hooks")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("worktree core.hooksPath = %q, want %q", got, want)
	}
}

// TestInstallHooksRelativePathHookFiresByExecution reproduces issue #145.
// In production, MillDir is ".mill" (relative), so the worktree path passed to
// installHooks is relative. A relative core.hooksPath is resolved by git from
// the committer's CWD (the worktree root), which double-nests the path and
// makes the hook unfindable. The fix stores an absolute path.
//
// This test uses a RELATIVE worktree path and verifies by execution — it makes
// a real git commit and asserts the hook fires (the commit is rejected). A test
// that only checked the config string would have passed on the old broken
// behaviour because git accepts any string value for core.hooksPath.
func TestInstallHooksRelativePathHookFiresByExecution(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}

	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Add a valid Go file so `go build` / `go vet` pass in the worktree.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "main.go")
	runGit(t, dir, "commit", "-m", "add main.go")

	// Create a worktree at a RELATIVE path — this is the crux of #145.
	wt := "wt-relative"
	runGit(t, dir, "worktree", "add", wt)

	// Install hooks with the relative worktree path (as runDelegate does).
	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Plant a failing gate so the hook will reject the commit if it fires.
	checksDir := filepath.Join(wt, ".mill", "checks")
	if err := os.MkdirAll(checksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	failGate := filepath.Join(checksDir, "gate-fail")
	if err := os.WriteFile(failGate, []byte("#!/bin/bash\necho 'FAIL test gate'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Stage a harmless file and attempt a commit from inside the worktree.
	if err := os.WriteFile(filepath.Join(wt, "change.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wt, "add", "change.txt")

	cmd := exec.Command("git", "-C", wt, "commit", "-m", "test: hook should block")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("git commit should have been blocked by the pre-commit hook but succeeded — hook did not fire")
	}
	if !strings.Contains(string(out), "FAIL test gate") {
		t.Errorf("expected 'FAIL test gate' in hook output, got: %s", out)
	}
	if !strings.Contains(string(out), "mill: pre-commit gauntlet") {
		t.Errorf("expected gauntlet to run, got: %s", out)
	}
}

// TestRoleEnforceBlocksStaffGoCommitInWorktree end-to-end verifies that a
// staff-roled agent committing internal/**.go in a delegated worktree is
// blocked by role-enforce — proving the hooks path resolves correctly and the
// gauntlet runs the role capability check.
func TestRoleEnforceBlocksStaffGoCommitInWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}

	// Capture project root before setupTestGitRepo changes CWD, so we can
	// copy the real role-enforce script into the worktree.
	root, err := projectRoot()
	if err != nil {
		t.Skipf("cannot find project root: %v", err)
	}
	enforceData, err := os.ReadFile(filepath.Join(root, "checks", "role-enforce"))
	if err != nil {
		t.Fatalf("cannot read role-enforce: %v", err)
	}

	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Add a valid Go file so `go build` / `go vet` pass.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "main.go")
	runGit(t, dir, "commit", "-m", "add main.go")

	// Create a worktree at a RELATIVE path (reproduces #145).
	wt := "wt-role-enforce"
	runGit(t, dir, "worktree", "add", wt)

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Set .mill/role = staff in the worktree.
	millDir := filepath.Join(wt, ".mill")
	if err := os.MkdirAll(millDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(millDir, "role"), []byte("staff"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Staff role definition: only .md files allowed.
	rolesDir := filepath.Join(millDir, "roles", "staff")
	if err := os.MkdirAll(rolesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	roleContent := []byte("---\nrole: staff\nallowed_files:\n  - .md\n---\n")
	if err := os.WriteFile(filepath.Join(rolesDir, "ROLE.md"), roleContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Install the real role-enforce script into the worktree.
	checksDir := filepath.Join(millDir, "checks")
	if err := os.MkdirAll(checksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checksDir, "role-enforce"), enforceData, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a violating file: internal/foo.go (staff cannot commit .go).
	internalDir := filepath.Join(wt, "internal")
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "foo.go"),
		[]byte("package internal\n\nfunc Foo() string { return \"foo\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit(t, wt, "add", "internal/foo.go")

	// Commit should be blocked by role-enforce.
	cmd := exec.Command("git", "-C", wt, "commit", "-m", "test: staff committing go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("commit should have been blocked by role-enforce but succeeded")
	}
	if !strings.Contains(string(out), "BLOCKED") {
		t.Errorf("expected 'BLOCKED' in output, got: %s", out)
	}
	if !strings.Contains(string(out), "role-enforce") {
		t.Errorf("expected 'role-enforce' in output, got: %s", out)
	}
}

func TestGateLoopRunsGatesAndSkipsLibrary(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}
	dir := t.TempDir()
	checksDir := filepath.Join(dir, ".mill", "checks")
	if err := os.MkdirAll(checksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// common.sh is a helper library that would abort the loop if executed:
	// it is not a gate and must be skipped.
	commonPath := filepath.Join(checksDir, "common.sh")
	if err := os.WriteFile(commonPath, []byte("#!/bin/bash\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	// gate-ok is a real gate that passes.
	okGate := filepath.Join(checksDir, "gate-ok")
	if err := os.WriteFile(okGate, []byte("#!/bin/bash\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runGateLoop(t, dir, nil)
	if err != nil {
		t.Fatalf("gate loop failed with only passing gates: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running gate: gate-ok") {
		t.Errorf("expected gate-ok to run, output:\n%s", out)
	}
	if strings.Contains(out, "Running gate: common.sh") {
		t.Errorf("common.sh was executed as a gate — it is a library, not a gate; output:\n%s", out)
	}

	// Add a failing gate: the loop must now fail.
	badGate := filepath.Join(checksDir, "gate-bad")
	if err := os.WriteFile(badGate, []byte("#!/bin/bash\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err = runGateLoop(t, dir, nil)
	if err == nil {
		t.Fatalf("gate loop should have failed with gate-bad present; output:\n%s", out)
	}
	if !strings.Contains(out, "FAIL gate-bad") {
		t.Errorf("expected 'FAIL gate-bad' in output, got:\n%s", out)
	}
}

func TestGateLoopPipefailSurvives(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash binary not available")
	}
	dir := t.TempDir()
	checksDir := filepath.Join(dir, ".mill", "checks")
	if err := os.MkdirAll(checksDir, 0755); err != nil {
		t.Fatal(err)
	}
	// A gate with set -euo pipefail must run via its shebang (bash).
	// Under the old `sh "$gate"` this aborted with "Illegal option -o pipefail".
	gatePath := filepath.Join(checksDir, "gate-pipefail")
	gateBody := "#!/bin/bash\nset -euo pipefail\nexit 0\n"
	if err := os.WriteFile(gatePath, []byte(gateBody), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runGateLoop(t, dir, nil)
	if err != nil {
		t.Fatalf("pipefail gate failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running gate: gate-pipefail") {
		t.Errorf("expected gate-pipefail to run, output:\n%s", out)
	}
}

func TestInstallHooksVerifiesExecutable(t *testing.T) {
	// After installHooks, the hook must be executable (mode 0755)
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	wt := filepath.Join(dir, "worktree-test")
	runGit(t, dir, "worktree", "add", wt)
	os.MkdirAll(filepath.Join(wt, ".mill", "checks"), 0755)

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Verify the hook is executable by checking permission bits
	hookPath := filepath.Join(wt, ".mill", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("cannot stat pre-commit: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("pre-commit hook is not executable after install")
	}
}

func TestInstallHooksEmptyChecksDir(t *testing.T) {
	// Empty .mill/checks/ should not cause installHooks to fail
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	wt := filepath.Join(dir, "worktree-test")
	runGit(t, dir, "worktree", "add", wt)
	// Note: we do NOT create .mill/checks/ — dispatcher handles missing dir gracefully

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks should not fail with missing .mill/checks/: %v", err)
	}

	// Hook should still be installed
	hookPath := filepath.Join(wt, ".mill", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("pre-commit hook should be installed even without .mill/checks/")
	}
}

func TestPreCommitHookPassesOnSuccess(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Add a Go file so go build/vet can find packages
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	runGit(t, dir, "add", "main.go")
	runGit(t, dir, "commit", "-m", "add main.go")

	// Create a real git worktree
	wt := filepath.Join(dir, "worktree-success")
	runGit(t, dir, "worktree", "add", wt)

	// Create .mill/checks with a passing gate
	checksDir := filepath.Join(wt, ".mill", "checks")
	os.MkdirAll(checksDir, 0755)
	passGate := filepath.Join(checksDir, "test-pass.sh")
	os.WriteFile(passGate, []byte("#!/bin/sh\necho 'PASS test gate'\nexit 0\n"), 0755)

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Make a change and stage it so we can commit
	changeFile := filepath.Join(wt, "change.txt")
	os.WriteFile(changeFile, []byte("test change"), 0644)
	runGit(t, wt, "add", "change.txt")

	// git commit should succeed (hook passes)
	cmd := exec.Command("git", "-C", wt, "commit", "-m", "test: hook should pass")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git commit should have passed but failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "mill: pre-commit passed") {
		t.Errorf("expected 'mill: pre-commit passed' in output, got: %s", string(out))
	}
}

func TestPreCommitHookBlocksOnFailure(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Add a Go file so go build/vet can find packages
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	runGit(t, dir, "add", "main.go")
	runGit(t, dir, "commit", "-m", "add main.go")

	// Create a real git worktree
	wt := filepath.Join(dir, "worktree-fail")
	runGit(t, dir, "worktree", "add", wt)

	// Create .mill/checks with a failing gate
	checksDir := filepath.Join(wt, ".mill", "checks")
	os.MkdirAll(checksDir, 0755)
	failGate := filepath.Join(checksDir, "gate-fail")
	os.WriteFile(failGate, []byte("#!/bin/bash\necho 'FAIL test gate — intentional'\nexit 1\n"), 0755)

	if err := installHooks(wt); err != nil {
		t.Fatalf("installHooks returned error: %v", err)
	}

	// Make a change and stage it so we can commit
	changeFile := filepath.Join(wt, "change.txt")
	os.WriteFile(changeFile, []byte("test change"), 0644)
	runGit(t, wt, "add", "change.txt")

	// git commit should be blocked by the failing hook
	cmd := exec.Command("git", "-C", wt, "commit", "-m", "test: hook should block")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("git commit should have been blocked by failing hook but succeeded")
	}
	// Exit code should be 1 (hook failure, not git error)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	}
	// Output should contain the failure message
	if !strings.Contains(string(out), "FAIL test gate") {
		t.Errorf("expected 'FAIL test gate' in output, got: %s", string(out))
	}
}

// --- Retry dispatch tests ---

// TestDispatchLoopRetryTransient verifies that transient failures advance
// to the next model in the fallback chain.
func TestDispatchLoopRetryTransient(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	callCount := 0
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			callCount++
			// First call (produce with gpt-5): transient → advance to deepseek-v4-pro
			// Second call (produce with deepseek-v4-pro): OK
			// Third call (review with deepseek-v4-pro): OK
			if callCount == 1 {
				return &fakeSession{result: transientResult()}, nil
			}
			return &fakeSession{result: okResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	// Use "free" tier to get multi-model chain: ["gpt-5", "deepseek-v4-pro"]
	err := app.Run("delegate", "--wait", "-model", "free", "101")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Verify state: task should be done and approved
	s, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	task, ok := s.Task("task-101")
	if !ok {
		t.Fatal("expected task-101 to exist")
	}
	if task.Status != domain.TaskDone {
		t.Errorf("expected status %q, got %q", domain.TaskDone, task.Status)
	}
	if task.Verdict != domain.VerdictApproved {
		t.Errorf("expected verdict %q, got %q", domain.VerdictApproved, task.Verdict)
	}

	// Verify output.txt was written
	outputPath := filepath.Join(app.worktreePath(101), "output.txt")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected output.txt to exist: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output.txt")
	}

	// 1 transient produce + 1 OK produce + 1 OK review = 3 dispatches
	if len(fa.allOpts) != 3 {
		t.Errorf("expected 3 dispatches (transient produce + OK produce + review), got %d", len(fa.allOpts))
	}
}

// TestDispatchLoopExecutionFailureRetries verifies that EXECUTION_FAILURE
// (e.g. exit 4, formerly FATAL) now retries the fallback chain before the
// task is marked as error.
func TestDispatchLoopExecutionFailureRetries(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			return &fakeSession{result: fatalResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	err := app.Run("delegate", "--wait", "102")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// EXECUTION_FAILURE retries the fallback chain: produceModel + chain = 2 attempts
	if len(fa.allOpts) != 2 {
		t.Errorf("expected 2 produce dispatches (retry on EXECUTION_FAILURE), got %d", len(fa.allOpts))
	}

	s, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	task, ok := s.Task("task-102")
	if !ok {
		t.Fatal("expected task-102 to exist")
	}
	if task.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, task.Status)
	}
	if task.Verdict != domain.VerdictRejected {
		t.Errorf("expected verdict %q, got %q", domain.VerdictRejected, task.Verdict)
	}
}

// TestDispatchLoopChainExhausted verifies that when all models in the
// fallback chain fail with transient errors, the task is marked as error.
func TestDispatchLoopChainExhausted(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			// Always return transient — will exhaust the chain
			return &fakeSession{result: transientResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	err := app.Run("delegate", "--wait", "103")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// produceModel=deepseek-v4-pro (role-resolved) + chain=["gpt-5"] = 2 dispatch attempts
	if len(fa.allOpts) != 2 {
		t.Errorf("expected 2 produce dispatches (produceModel + chain fallback exhausted on transient), got %d", len(fa.allOpts))
	}

	s, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	task, ok := s.Task("task-103")
	if !ok {
		t.Fatal("expected task-103 to exist")
	}
	if task.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, task.Status)
	}
	if task.Verdict != domain.VerdictRejected {
		t.Errorf("expected verdict %q, got %q", domain.VerdictRejected, task.Verdict)
	}
}
func TestDispatchLoopApprovedAfterRetry(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Write config with model alias "free" to get multi-model fallback chain
	cfg := config.Config{Model: "free"}
	if err := cfg.Save(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	callCount := 0
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			callCount++
			// Produce: 3 transient → 4th succeeds → review: OK
			if callCount <= 3 {
				return &fakeSession{result: transientResult()}, nil
			}
			return &fakeSession{result: okResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	err := app.Run("delegate", "--wait", "104")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Should have 5 dispatches: 3 transient produce + 1 OK produce + 1 OK review
	if len(fa.allOpts) != 5 {
		t.Errorf("expected 5 dispatches (3 transient + produce OK + review OK), got %d", len(fa.allOpts))
	}

	// Review model should be deepseek-v4-pro
	if len(fa.allOpts) >= 5 {
		if fa.allOpts[4].Model != "deepseek-v4-pro" {
			t.Errorf("expected review model deepseek-v4-pro, got %q", fa.allOpts[4].Model)
		}
	}

	s, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	task, ok := s.Task("task-104")
	if !ok {
		t.Fatal("expected task-104 to exist")
	}
	if task.Status != domain.TaskDone {
		t.Errorf("expected status %q, got %q", domain.TaskDone, task.Status)
	}
	if task.Verdict != domain.VerdictApproved {
		t.Errorf("expected verdict %q, got %q", domain.VerdictApproved, task.Verdict)
	}

	// Verify output.txt was written with the successful produce output
	outputPath := filepath.Join(app.worktreePath(104), "output.txt")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected output.txt to exist: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output.txt")
	}
}
func TestValidateDelegateBinariesEmptyProvider(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir}
	task := domain.NewTask("task-1", 1)
	s := state.New()
	s.UpsertTask(task)
	cfg := config.Config{Provider: ""}
	if err := app.validateDelegateBinaries(cfg, &task, s); err != nil {
		t.Fatalf("expected no error for empty provider, got: %v", err)
	}
	if task.Phase != domain.TaskPhaseDispatch || task.FailureClass != domain.CLASS_OK {
		t.Errorf("expected task unchanged, got phase=%q class=%q", task.Phase, task.FailureClass)
	}
}

func TestValidateDelegateBinariesUnknownProvider(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir}
	task := domain.NewTask("task-1", 1)
	s := state.New()
	s.UpsertTask(task)
	cfg := config.Config{Provider: "nonexistent-binary-xyz"}
	if err := app.validateDelegateBinaries(cfg, &task, s); err != nil {
		t.Fatalf("expected nil error (failure recorded in state), got: %v", err)
	}
	if task.Phase != domain.TaskPhaseAborted {
		t.Errorf("expected phase %q, got %q", domain.TaskPhaseAborted, task.Phase)
	}
	if task.Status != domain.TaskAborted {
		t.Errorf("expected status %q, got %q", domain.TaskAborted, task.Status)
	}
	if task.FailureClass != domain.ENVIRONMENT_FAILURE {
		t.Errorf("expected failure class %q, got %q", domain.ENVIRONMENT_FAILURE, task.FailureClass)
	}
	if task.AbortReason == "" {
		t.Error("expected non-empty abort reason")
	}
}
func TestRecordError(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}

	app := &App{MillDir: d, Out: new(bytes.Buffer), Err: new(bytes.Buffer)}

	s := state.New()
	task := domain.Task{ID: "task-1", Issue: 1, Status: domain.TaskRunning}
	s.UpsertTask(task)

	app.recordError(s, 1, task, nil, "test-failure")

	// Verify task status was updated
	updated, ok := s.Task("task-1")
	if !ok {
		t.Fatal("expected task-1 to exist")
	}
	if updated.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, updated.Status)
	}
	if updated.Verdict != domain.VerdictRejected {
		t.Errorf("expected verdict %q, got %q", domain.VerdictRejected, updated.Verdict)
	}

	// Verify state was persisted
	statePath := app.statePath()
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("expected state file to exist after Save")
	}

	// Verify ledger entry was written
	ledgerPath := app.ledgerPath(1)
	if _, err := os.Stat(ledgerPath); os.IsNotExist(err) {
		t.Error("expected ledger file to exist")
	}
}

func TestRecordErrorWithExistingState(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}

	app := &App{MillDir: d, Out: new(bytes.Buffer), Err: new(bytes.Buffer)}

	// Pre-populate state with an existing task
	s := state.New()
	existingTask := domain.Task{ID: "task-1", Issue: 1, Status: domain.TaskRunning}
	s.UpsertTask(existingTask)
	s.Save(app.statePath())

	// Reload state
	loaded, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	task, ok := loaded.Task("task-1")
	if !ok {
		t.Fatal("expected task-1 in loaded state")
	}

	app.recordError(loaded, 1, task, nil, "test-error")

	// Verify state was persisted after recordError
	reloaded, err := state.Load(app.statePath())
	if err != nil {
		t.Fatalf("failed to reload state: %v", err)
	}
	updated, ok := reloaded.Task("task-1")
	if !ok {
		t.Fatal("expected task-1 to exist after recordError")
	}
	if updated.Status != domain.TaskError {
		t.Errorf("expected status %q, got %q", domain.TaskError, updated.Status)
	}
}

func TestBuildRolePromptFallback(t *testing.T) {
	// Change to a directory without .mill/roles to trigger the fallback path
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	result := buildRolePrompt(99, "nonexistent-role", "Fix the thing")
	if !strings.Contains(result, "You are mill, an agent delegation harness") {
		t.Error("expected generic fallback prompt")
	}
	if !strings.Contains(result, "#99") {
		t.Error("expected issue #99 in fallback prompt")
	}
	if !strings.Contains(result, "Fix the thing") {
		t.Error("expected issue body in fallback prompt")
	}
}

func TestBuildRolePromptFallbackNoBody(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	result := buildRolePrompt(99, "nonexistent-role", "")
	if !strings.Contains(result, "You are mill, an agent delegation harness") {
		t.Error("expected generic fallback prompt")
	}
	if strings.Contains(result, "Issue Body") {
		t.Error("expected no Issue Body section when body is empty")
	}
}

func TestDelegateWithStageLabel(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Output:   "APPROVED - done",
		},
	}
	buf := new(bytes.Buffer)
	// IssueReader that returns stage labels
	stageIssueReader := func(issueNum int) (string, []string, error) {
		return "Test issue body", []string{"stage:produce", "bug"}, nil
	}
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: stageIssueReader}

	err := app.Run("delegate", "--wait", "99")
	if err != nil {
		t.Fatalf("delegate with stage label returned error: %v", err)
	}
}

func TestDelegateWithMultipleStageLabels(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Output:   "APPROVED - done",
		},
	}
	buf := new(bytes.Buffer)
	// IssueReader that returns multiple stage labels (should warn)
	multiStageIssueReader := func(issueNum int) (string, []string, error) {
		return "Test issue body", []string{"stage:produce", "stage:review", "bug"}, nil
	}
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: multiStageIssueReader}

	err := app.Run("delegate", "--wait", "100")
	if err != nil {
		t.Fatalf("delegate with multiple stage labels returned error: %v", err)
	}
	// Should have warned about multiple stage labels
	if !strings.Contains(buf.String(), "warning: multiple stage:* labels") {
		t.Error("expected warning about multiple stage:* labels")
	}
}

func TestIsTransientErrorNil(t *testing.T) {
	if isTransientError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsTransientErrorECONNREFUSED(t *testing.T) {
	if !isTransientError(fmt.Errorf("dial tcp: econnrefused")) {
		t.Error("expected true for econnrefused")
	}
}

func TestIsTransientErrorTimeout(t *testing.T) {
	if !isTransientError(fmt.Errorf("request timeout")) {
		t.Error("expected true for timeout")
	}
}

func TestIsTransientErrorTemporaryFailure(t *testing.T) {
	if !isTransientError(fmt.Errorf("temporary failure in name resolution")) {
		t.Error("expected true for temporary failure")
	}
}

func TestIsTransientErrorNetwork(t *testing.T) {
	if !isTransientError(fmt.Errorf("network is unreachable")) {
		t.Error("expected true for network error")
	}
}

func TestIsTransientErrorNonTransient(t *testing.T) {
	if isTransientError(fmt.Errorf("file not found")) {
		t.Error("expected false for non-transient error")
	}
}

func TestDelegateAsyncDispatch(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Output:   "APPROVED - done",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	err := app.Run("delegate", "200")
	if err != nil {
		t.Fatalf("async delegate returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "async") {
		t.Errorf("expected 'async' in output, got: %s", output)
	}
	// Give the async goroutine time to complete before cleanup.
	time.Sleep(200 * time.Millisecond)
}

// --- Uncommitted change preservation tests (#146) ---

// TestCreateWorktreePreservesUncommittedChanges verifies that re-delegating
// onto a worktree with uncommitted changes does not silently discard them.
// The changes should be committed to a scratch/N branch so they survive.
func TestCreateWorktreePreservesUncommittedChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	// First creation
	wt, err := app.createWorktree(61)
	if err != nil {
		t.Fatalf("first createWorktree failed: %v", err)
	}

	// Simulate uncommitted agent output
	outputFile := filepath.Join(wt, "feature.go")
	if err := os.WriteFile(outputFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Re-delegate: createWorktree should preserve changes in a scratch ref
	wt2, err := app.createWorktree(61)
	if err != nil {
		t.Fatalf("second createWorktree failed: %v", err)
	}

	// The file should NOT be in the fresh worktree
	if _, err := os.Stat(filepath.Join(wt2, "feature.go")); !os.IsNotExist(err) {
		t.Error("file should not exist in fresh worktree after re-delegation")
	}

	// The scratch branch should exist with the file content
	scratch := scratchBranchName(61)
	branchCmd := exec.Command("git", "-C", dir, "branch", "--list", scratch)
	out, err := branchCmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if !strings.Contains(string(out), scratch) {
		t.Fatalf("expected scratch branch %s to exist", scratch)
	}

	showCmd := exec.Command("git", "-C", dir, "show", scratch+":feature.go")
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show failed: %v\n%s", err, showOut)
	}
	if string(showOut) != "package main\n" {
		t.Errorf("expected preserved file content 'package main\\n', got: %q", string(showOut))
	}

	// Verify preservation message was printed
	errBuf := app.Err.(*bytes.Buffer)
	if !strings.Contains(errBuf.String(), "Preserving uncommitted changes in branch") {
		t.Error("expected preservation message in stderr")
	}
}

// TestCreateWorktreeNoChangesDoesNotScratch verifies that re-delegating onto
// a clean worktree (no uncommitted changes) does not create a scratch ref.
func TestCreateWorktreeNoChangesDoesNotScratch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	// First creation
	wt, err := app.createWorktree(62)
	if err != nil {
		t.Fatalf("first createWorktree failed: %v", err)
	}

	// Commit the PID file so the worktree is clean
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "initial")

	// Second creation — worktree is clean, no scratch ref should be created
	_, err = app.createWorktree(62)
	if err != nil {
		t.Fatalf("second createWorktree failed: %v", err)
	}

	branchCmd := exec.Command("git", "-C", dir, "branch", "--list", scratchBranchName(62))
	out, _ := branchCmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected no scratch branch for clean worktree, got: %s", out)
	}
}

// TestCommitWorktreeOnComplete verifies that changes are committed to the
// agent branch after delegation completes.
func TestCommitWorktreeOnComplete(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	wt, err := app.createWorktree(63)
	if err != nil {
		t.Fatalf("createWorktree failed: %v", err)
	}

	// Write uncommitted changes
	featureFile := filepath.Join(wt, "feature.go")
	if err := os.WriteFile(featureFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit on completion
	app.commitWorktreeOnComplete(63, wt)

	// Verify the file is committed
	cmd := exec.Command("git", "-C", wt, "log", "--oneline", "-1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "commit agent output") {
		t.Errorf("expected commit message about agent output, got: %s", out)
	}

	// Verify worktree is now clean (ignoring .mill/ bookkeeping)
	statusCmd := exec.Command("git", "-C", wt, "status", "--porcelain")
	statusOut, _ := statusCmd.Output()
	trackedChanges := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		trackedChanges = append(trackedChanges, line)
	}
	if len(trackedChanges) > 0 {
		t.Errorf("expected clean worktree after commit, got: %s", statusOut)
	}
}

// TestDelegateReDelegationPreservesUncommittedChanges is the acceptance
// test (#146): delegate, leave changes uncommitted, delegate again, assert
// the changes survive.
func TestDelegateReDelegationPreservesUncommittedChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	writeOnce := false
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			if !writeOnce {
				writeOnce = true
				outputFile := filepath.Join(opts.Worktree, "agent_output.go")
				if err := os.WriteFile(outputFile, []byte("package main\n// agent output\n"), 0644); err != nil {
					return nil, err
				}
			}
			return &fakeSession{result: okResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}

	// First delegation
	if err := app.Run("delegate", "--wait", "116"); err != nil {
		t.Fatalf("first delegate returned error: %v", err)
	}

	wt := app.worktreePath(116)

	// Leave changes uncommitted after delegation
	extraFile := filepath.Join(wt, "uncommitted_extra.go")
	if err := os.WriteFile(extraFile, []byte("package main\n// extra\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second delegation — should preserve uncommitted changes
	buf.Reset()
	if err := app.Run("delegate", "--wait", "116"); err != nil {
		t.Fatalf("second delegate returned error: %v\nstderr: %s", err, buf.String())
	}

	// The uncommitted file should be preserved in the scratch ref
	scratch := scratchBranchName(116)
	branchCmd := exec.Command("git", "-C", dir, "branch", "--list", scratch)
	out, err := branchCmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if !strings.Contains(string(out), scratch) {
		t.Fatalf("expected scratch branch %s to exist after re-delegation", scratch)
	}

	showCmd := exec.Command("git", "-C", dir, "show", scratch+":uncommitted_extra.go")
	showOut, err := showCmd.Output()
	if err != nil {
		t.Fatalf("git show failed for preserved file: %v\n%s", err, showOut)
	}
	if string(showOut) != "package main\n// extra\n" {
		t.Errorf("expected preserved file content, got: %q", string(showOut))
	}

	// Verify preservation message was printed
	if !strings.Contains(buf.String(), "Preserving uncommitted changes in branch") {
		t.Error("expected preservation message in output")
	}
}

func TestRunRecursionHandoffNilEngineIsNoop(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	// Recursion is left nil
	err := app.runRecursionHandoff("architect", t.TempDir(), 153)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestRunRecursionHandoffMissingRoleReturnsError(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	app.Recursion = &recursion.Delegator{RolesRoot: filepath.Join(dir, "roles")}

	err := app.runRecursionHandoff("architect", t.TempDir(), 153)
	if err == nil {
		t.Fatal("expected error for missing role file")
	}
	if !strings.Contains(err.Error(), "cannot read role architect") {
		t.Errorf("expected error message containing 'cannot read role architect', got: %v", err)
	}
}

func TestRunRecursionHandoffLeafRoleIsNoop(t *testing.T) {
	dir := t.TempDir()
	roleDir := filepath.Join(dir, "roles", "sr-dev-be")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatal(err)
	}

	roleContent := `---
role: sr-dev-be
model: cheap
agent: task
reviewed_by: tech-lead
allowed_files:
  - .go
---

# Role: Backend Dev
`
	if err := os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte(roleContent), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	app.Recursion = &recursion.Delegator{RolesRoot: filepath.Join(dir, "roles")}

	err := app.runRecursionHandoff("sr-dev-be", t.TempDir(), 153)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestRunRecursionHandoffRecordsChildDispatchInLedger(t *testing.T) {
	dir := t.TempDir()

	// Set up role files: architect (non-leaf) → tech-lead, qa-docs
	rolesDir := filepath.Join(dir, "roles")
	archDir := filepath.Join(rolesDir, "architect")
	techDir := filepath.Join(rolesDir, "tech-lead")
	qaDir := filepath.Join(rolesDir, "qa-docs")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(techDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(qaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	archContent := `---
role: architect
model: pro
agent: task
reviewed_by: staff
delegates_to:
  - tech-lead
  - qa-docs
allowed_files:
  - .md
skills:
  - codebase-design
---
# Role: Architect
`
	if err := os.WriteFile(filepath.Join(archDir, "ROLE.md"), []byte(archContent), 0o644); err != nil {
		t.Fatal(err)
	}

	techContent := `---
role: tech-lead
model: pro
agent: task
reviewed_by: architect
delegates_to:
  - qa-docs
allowed_files:
  - .go
  - .md
skills:
  - tdd
---
# Role: Tech Lead
`
	if err := os.WriteFile(filepath.Join(techDir, "ROLE.md"), []byte(techContent), 0o644); err != nil {
		t.Fatal(err)
	}

	qaContent := `---
role: qa-docs
model: free→paid
agent: task
reviewed_by: delegator
delegates_to: []
allowed_files:
  - .md
skills:
  - verification-before-completion
---
# Role: QA / Docs
`
	if err := os.WriteFile(filepath.Join(qaDir, "ROLE.md"), []byte(qaContent), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	app.Recursion = &recursion.Delegator{RolesRoot: rolesDir}

	wt := t.TempDir()
	err := app.runRecursionHandoff("architect", wt, 153)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Verify child dispatch events were recorded in the ledger
	ledgerPath := filepath.Join(dir, "ledger", "153.jsonl")
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("ledger file not created: %v", err)
	}
	ledgerStr := string(data)

	// Each child should produce a dispatch entry with parent_issue and role
	for _, expectedChild := range []string{"tech-lead", "qa-docs"} {
		if !strings.Contains(ledgerStr, `"role":"`+expectedChild+`"`) {
			t.Errorf("ledger missing child dispatch for role %s", expectedChild)
		}
	}
	if !strings.Contains(ledgerStr, `"parent_issue":153`) {
		t.Error("ledger entries missing parent_issue field")
	}
	for _, line := range strings.Split(ledgerStr, "\n") {
		if strings.Contains(line, `"event":"dispatch"`) && strings.Contains(line, `"parent_issue":153`) {
			if !strings.Contains(line, `"depth":1`) {
				t.Errorf("child dispatch line missing depth:1: %s", line)
			}
		}
	}
}
