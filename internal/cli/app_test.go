package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func setupTestGitRepo(t *testing.T, dir string) {
	t.Helper()
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
	err := runLand("main", dir, []string{}, false)
	if err != nil {
		t.Fatalf("runLand with empty gates returned error: %v", err)
	}
	if branch := currentBranch(t, dir); branch != "main" {
		t.Errorf("expected HEAD on 'main', got %q", branch)
	}
}

func TestRunLandGateFailure(t *testing.T) {
	dir := t.TempDir()
	err := runLand("main", dir, []string{"exit 1"}, false)
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

	err := runLand("main", dir, []string{}, true)
	if err != nil {
		t.Fatalf("runLand with confirm=no should return nil, got: %v", err)
	}
	// Checkout should NOT have run; still on 'feature'.
	if branch := currentBranch(t, dir); branch != "feature" {
		t.Errorf("expected HEAD to stay on 'feature', got %q", branch)
	}
}
