package cli

import (
	"bytes"
	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAdapter implements adapter.Adapter for CLI testing.
type fakeAdapter struct {
	dispatched  bool
	opts        adapter.DispatchOpts
	result      adapter.SessionResult
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

func (s *fakeSession) ID() string     { return "fake-session-1" }
func (s *fakeSession) Status() string { return "done" }
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

	err := app.Run("delegate", "--wait", "390")
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
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  1,
			Output:   "NEEDS CHANGES - please fix the formatting",
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

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

	err := app.Run("delegate", "--wait", "99")
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

	err := app.Run("delegate", "--wait", "-model", "deepseek-v4-pro", "555")
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
	originalContent, err := os.ReadFile(filepath.Join(".mill", "checks", "common.sh"))
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
	// No .mill/role file → defaults to staff, no delegation validation
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

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
	fa := &fakeAdapter{
		result: adapter.SessionResult{
			ExitCode: 0,
			Commits:  0,
		},
	}
	buf := new(bytes.Buffer)
	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

	err := app.Run("delegate", "--wait", "7")
	if err != nil {
		t.Fatalf("delegate returned error: %v", err)
	}

	wt := app.worktreePath(7)

	// Verify scaffold files exist
	for _, rel := range []string{
		filepath.Join(".mill", "AGENTS.md"),
		filepath.Join(".omp", "AGENTS.md"),
		filepath.Join(".omp", "RULES.md"),
		filepath.Join(".mill", "roles", "COMMON.md"),
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
			fa := &fakeAdapter{
				result: adapter.SessionResult{
					ExitCode: tt.exitCode,
					Commits:  0,
					Output:   "BLOCKED — budget exceeded",
				},
			}
			buf := new(bytes.Buffer)
			app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf}

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

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "laguna-free"}
	got := app.resolveModel("sr-dev-be", cfg)
	if got != "laguna-free" {
		t.Errorf("expected fallback to config model, got %q", got)
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

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "laguna-free"}
	got := app.resolveModel("sr-dev-be", cfg)
	if got != "laguna-free" {
		t.Errorf("expected fallback to config model for empty tier, got %q", got)
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

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "laguna-free"}
	got := app.resolveModel("sr-dev-be", cfg)
	// paid maps to deepseek/deepseek-v4-pro in modelTier
	if got != "deepseek/deepseek-v4-pro" {
		t.Errorf("expected modelTier mapping for 'paid', got %q", got)
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

	result := buildRolePrompt(1, "sr-dev-be")
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

	result := buildRolePrompt(1, "sr-dev-be")
	if !strings.Contains(result, "1") {
		t.Error("expected output to contain issue number '1'")
	}
	if !strings.Contains(result, "sr-dev-be") {
		t.Error("expected output to contain role name 'sr-dev-be'")
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
