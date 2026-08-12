package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/recursion"
)

func TestRunUnknownCommandReturnsError(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRunNoArgsShowsUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run()
	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}
}

func TestRunHelpShowsUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("--help")
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("mill")) {
		t.Error("expected usage output to contain 'mill'")
	}
	if !bytes.Contains(buf.Bytes(), []byte("delegate")) {
		t.Error("expected usage output to contain 'delegate' command")
	}
	if !bytes.Contains(buf.Bytes(), []byte("status")) {
		t.Error("expected usage output to contain 'status' command")
	}
	if !bytes.Contains(buf.Bytes(), []byte("init")) {
		t.Error("expected usage output to contain 'init' command")
	}
}

func TestAppPathMethods(t *testing.T) {
	app := &App{MillDir: "/tmp/milltest"}

	if app.statePath() != "/tmp/milltest/state.json" {
		t.Errorf("statePath = %q, want %q", app.statePath(), "/tmp/milltest/state.json")
	}
	if app.configPath() != "/tmp/milltest/config.json" {
		t.Errorf("configPath = %q, want %q", app.configPath(), "/tmp/milltest/config.json")
	}
	if app.ledgerPath(390) != "/tmp/milltest/ledger/390.jsonl" {
		t.Errorf("ledgerPath(390) = %q, want %q", app.ledgerPath(390), "/tmp/milltest/ledger/390.jsonl")
	}
	if app.worktreePath(390) != "/tmp/milltest/worktrees/issue-390" {
		t.Errorf("worktreePath(390) = %q, want %q", app.worktreePath(390), "/tmp/milltest/worktrees/issue-390")
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app.Adapter == nil {
		t.Error("expected non-nil Adapter")
	}
	if app.Out == nil {
		t.Error("expected non-nil Out")
	}
	if app.Err == nil {
		t.Error("expected non-nil Err")
	}
	if app.MillDir != ".mill" {
		t.Errorf("expected MillDir %q, got %q", ".mill", app.MillDir)
	}
}

func TestInitRecursionEngineWithConfig(t *testing.T) {
	app := &App{MillDir: t.TempDir()}
	myml := &config.MillYML{
		Project:  "test",
		Provider: "commandcode",
		Recursion: config.RecursionConfig{
			View:     "tree",
			MaxDepth: 3,
			Models:   map[string]string{"pro": "deepseek-v4-pro"},
		},
	}
	app.initRecursion(myml)

	if app.Recursion == nil {
		t.Fatal("expected recursion engine to be initialized when recursion config present")
	}
	if app.Recursion.MaxDepth != 3 {
		t.Errorf("expected MaxDepth 3, got %d", app.Recursion.MaxDepth)
	}
	if app.Recursion.Cost == nil || app.Recursion.Cost.Models["pro"] != "deepseek-v4-pro" {
		t.Errorf("expected CostResolver with pro model, got %+v", app.Recursion.Cost)
	}
	wantState := filepath.Join(app.MillDir, "state", "recursion.json")
	if app.Recursion.StatePath != wantState {
		t.Errorf("expected StatePath %q, got %q", wantState, app.Recursion.StatePath)
	}
}

func TestInitRecursionEngineAbsentConfig(t *testing.T) {
	app := &App{MillDir: t.TempDir()}
	app.initRecursion(nil)
	if app.Recursion != nil {
		t.Errorf("expected nil recursion engine for nil config, got %+v", app.Recursion)
	}

	app.Recursion = &recursion.Delegator{RolesRoot: "/tmp"}
	app.initRecursion(&config.MillYML{Project: "test", Provider: "commandcode"})
	if app.Recursion != nil {
		t.Errorf("expected nil recursion engine when recursion section absent, got %+v", app.Recursion)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func init() {
	modelAvailableFn = func(string) bool { return true }
}

func setupTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "checkout", "-b", "feature")
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestRunLandEmptyGates(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	err := runLand("main", dir, []string{}, false, false)
	if err != nil {
		t.Fatalf("runLand with empty gates returned error: %v", err)
	}
	if branch := currentBranch(t, dir); branch != "main" {
		t.Errorf("expected HEAD on 'main', got %q", branch)
	}
}

func TestRunLandGateFailure(t *testing.T) {
	dir := t.TempDir()
	err := runLand("main", dir, []string{"exit 1"}, false, false)
	if err == nil {
		t.Fatal("expected error for failing gate")
	}
	if !strings.Contains(err.Error(), "gate") {
		t.Errorf("expected gate-related error, got: %v", err)
	}
}

func TestRunLandConfirmNo(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	// Simulate user declining by feeding EOF to stdin.
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Close()
	defer func() { os.Stdin = origStdin }()

	err := runLand("main", dir, []string{}, true, false)
	if err != nil {
		t.Fatalf("runLand with confirm=no should return nil, got: %v", err)
	}
	// Checkout should NOT have run; still on 'feature'.
	if branch := currentBranch(t, dir); branch != "feature" {
		t.Errorf("expected HEAD to stay on 'feature', got %q", branch)
	}
}

func TestAppRunLandNoArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("land")
	if err == nil {
		t.Error("expected usage error for land with no target")
	}
}

func TestRunLandSuccessWithGates(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("land", "-worktree", dir, "main", "echo ok")
	if err != nil {
		t.Fatalf("runLand with gates returned error: %v", err)
	}
	if branch := currentBranch(t, dir); branch != "main" {
		t.Errorf("expected HEAD on 'main', got %q", branch)
	}
}

func TestRunLandHelpFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("land", "-h")
	if err != nil {
		t.Fatalf("runLand -h should return nil, got: %v", err)
	}
}

func TestRunLandParseError(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("land", "--nonexistent")
	if err == nil {
		t.Fatal("expected non-nil error for --nonexistent flag")
	}
}

func TestBuildPrompt(t *testing.T) {
	got := buildPrompt(42)
	if got == "" {
		t.Fatal("buildPrompt returned empty string")
	}
	if !strings.Contains(got, "42") {
		t.Errorf("buildPrompt output should contain issue number 42, got: %s", got)
	}
}

func TestVersionPrintsSomething(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	if err := app.Run("version"); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got == "" {
		t.Fatal("version printed nothing")
	}
}

func TestVersionLdflagOverride(t *testing.T) {
	orig := Version
	Version = "v2.0.0"
	defer func() { Version = orig }()

	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	if err := app.Run("version"); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %s", got)
	}
}

func TestResolveVersionFallsBackToDev(t *testing.T) {
	// In a non-git dir, resolveVersion should return "dev".
	td := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(td)
	defer func() { os.Chdir(origDir) }()

	got := resolveVersion()
	if got != "dev" {
		t.Errorf("expected dev in non-git dir, got %s", got)
	}
}
