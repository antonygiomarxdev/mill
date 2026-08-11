package cli

import (
	"bytes"
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
	if fa.allOpts[1].Model != "laguna-pro" {
		t.Errorf("expected review model laguna-pro, got %q", fa.allOpts[1].Model)
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
			ExitCode: 3,
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
	if fa.allOpts[0].Model != "laguna-ultra" {
		t.Errorf("expected produce model %q, got %q", "laguna-ultra", fa.allOpts[0].Model)
	}
	// Review dispatch also uses the same flag override
	if fa.allOpts[1].Model != "laguna-ultra" {
		t.Errorf("expected review model %q, got %q", "laguna-ultra", fa.allOpts[1].Model)
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

func TestClassifyResultExitCodes(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		stderr string
		want   domain.Classification
	}{
		{name: "exit 0 is OK", code: 0, want: domain.ClassificationOK},
		{name: "exit 3 is AUTH", code: 3, want: domain.ClassificationAuth},
		{name: "exit -1 is BLOCKED", code: -1, want: domain.ClassificationBlocked},
		{name: "exit -2 is BLOCKED", code: -2, want: domain.ClassificationBlocked},
		{name: "exit 4 is FATAL", code: 4, want: domain.ClassificationFatal},
		{name: "exit 5 is RATE_LIMITED", code: 5, want: domain.ClassificationRateLimited},
		{name: "exit 8 is MAX_TURNS", code: 8, want: domain.ClassificationMaxTurns},
		{name: "exit 10 is NO_CREDIT", code: 10, want: domain.ClassificationNoCredit},
		{name: "unknown exit is FATAL", code: 99, want: domain.ClassificationFatal},
		{name: "stderr blocked: overrides exit 0", code: 0, stderr: "blocked: budget exceeded", want: domain.ClassificationBlocked},
		{name: "stderr 401 signals AUTH", code: 1, stderr: "401 Unauthorized", want: domain.ClassificationAuth},
		{name: "stderr insufficient credits signals NO_CREDIT", code: 1, stderr: "insufficient credits", want: domain.ClassificationNoCredit},
		{name: "stderr timeout signals TRANSIENT", code: 1, stderr: "network timeout", want: domain.ClassificationTransient},
		{name: "stderr approved: signal", code: 1, stderr: "APPROVED: all good", want: domain.ClassificationOK},
		{name: "stderr changes_requested: signal", code: 0, stderr: "CHANGES_REQUESTED: needs work", want: domain.ClassificationChangesRequested},
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

			// Verify ledger classifies as BLOCKED
			ledgerFile := app.ledgerPath(53)
			content, err := os.ReadFile(ledgerFile)
			if err != nil {
				t.Fatalf("failed to read ledger: %v", err)
			}
			if !strings.Contains(string(content), string(domain.ClassificationBlocked)) {
				t.Error("expected ledger to contain BLOCKED classification")
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
	result := buildReviewPrompt54("Fix the login", "PATCH: done", nil, adapter.Capabilities{})
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
	failGate := filepath.Join(checksDir, "test-fail.sh")
	os.WriteFile(failGate, []byte("#!/bin/sh\necho 'FAIL test gate — intentional'\nexit 1\n"), 0755)

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

// TestDispatchLoopRetryTransient verifies that transient failures are retried
// with backoff before succeeding.
func TestDispatchLoopRetryTransient(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	callCount := 0
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			callCount++
			// First 2 calls (produce attempt 1, 2): transient
			// Third call (produce attempt 3): OK
			// Fourth call (review): OK
			if callCount <= 2 {
				return &fakeSession{result: transientResult()}, nil
			}
			return &fakeSession{result: okResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	err := app.Run("delegate", "--wait", "101")
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

	// Verify at least 3 produce dispatches (2 transient + 1 OK)
	if len(fa.allOpts) < 3 {
		t.Errorf("expected at least 3 dispatches (2 produce transient + 1 produce OK + review), got %d", len(fa.allOpts))
	}
}

// TestDispatchLoopNoRetryFatal verifies that FATAL failures skip retry
// and exit immediately.
func TestDispatchLoopNoRetryFatal(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	callCount := 0
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			callCount++
			return &fakeSession{result: fatalResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	err := app.Run("delegate", "--wait", "102")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// FATAL should skip retry: only 1 produce dispatch, 0 review dispatch
	if len(fa.allOpts) != 1 {
		t.Errorf("expected exactly 1 dispatch (no retry on FATAL), got %d", len(fa.allOpts))
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

// TestDispatchLoopMaxRetriesExceeded verifies that after maxRetries transient
// failures, the task is marked as error.
func TestDispatchLoopMaxRetriesExceeded(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	callCount := 0
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			callCount++
			// Always return transient — will exhaust all retries
			return &fakeSession{result: transientResult()}, nil
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader, Backoff: func(time.Duration) {}}

	err := app.Run("delegate", "--wait", "103")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	// Should have exactly 5 produce dispatches (1 initial + 4 retries)
	// and 0 review dispatches (produce never succeeded)
	if len(fa.allOpts) != 5 {
		t.Errorf("expected exactly 5 produce dispatches (1 initial + 4 retries), got %d", len(fa.allOpts))
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

// TestDispatchLoopApprovedAfterRetry verifies that transient failures on
// produce don't prevent review from approving the eventual successful output.
func TestDispatchLoopApprovedAfterRetry(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	callCount := 0
	fa := &fakeAdapter{
		dispatchFn: func(opts adapter.DispatchOpts) (adapter.Session, error) {
			callCount++
			// Produce: 3 transient → 4th succeeds → review: OK
			if callCount <= 3 {
				return &fakeSession{result: transientResult()}, nil
			}
			// callCount == 4: produce succeeds
			// callCount == 5: review succeeds
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

	// Review model should be laguna-pro
	if len(fa.allOpts) >= 5 {
		if fa.allOpts[4].Model != "laguna-pro" {
			t.Errorf("expected review model laguna-pro, got %q", fa.allOpts[4].Model)
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
	cfg := config.Config{Provider: ""}
	if err := validateDelegateBinaries(cfg); err != nil {
		t.Fatalf("expected no error for empty provider, got: %v", err)
	}
}

func TestValidateDelegateBinariesUnknownProvider(t *testing.T) {
	cfg := config.Config{Provider: "nonexistent-binary-xyz"}
	err := validateDelegateBinaries(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider binary not in PATH")
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
