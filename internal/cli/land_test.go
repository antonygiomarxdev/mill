package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsProcessAliveOwnPid(t *testing.T) {
	if !isProcessAlive(os.Getpid()) {
		t.Error("expected own PID to be alive")
	}
}

func TestIsProcessAliveDeadPid(t *testing.T) {
	if isProcessAlive(99999) {
		t.Error("expected PID 99999 to be dead")
	}
}

func TestRunLandForceFlag(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	runGit(t, dir, "checkout", "main")

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("land", "--force", "--worktree", dir, "main")
	if err != nil {
		t.Fatalf("expected --force to bypass stale lock, got: %v", err)
	}
}

// writePidFile creates the .mill/agent.pid file in a worktree directory.
func writePidFile(t *testing.T, worktreeDir string, pid int) {
	t.Helper()
	pidDir := filepath.Join(worktreeDir, ".mill")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatalf("failed to create .mill dir: %v", err)
	}
	pidFile := filepath.Join(pidDir, "agent.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}
}

// runGitOutput runs git in dir and returns trimmed stdout, failing on error.
func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestRunLandActuallyLands(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir) // main with "initial", leaves HEAD on "feature"

	// Target branch must be checked out in the main repo for the merge.
	runGit(t, dir, "checkout", "main")

	// Agent worktree on branch agent/x with a new commit.
	wt := filepath.Join(t.TempDir(), "agent-x")
	runGit(t, dir, "worktree", "add", "-b", "agent/x", wt, "main")
	if err := os.WriteFile(filepath.Join(wt, "agent-file.txt"), []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "agent work")

	before := runGitOutput(t, dir, "rev-parse", "main")
	if err := runLand("main", wt, []string{}, false, false); err != nil {
		t.Fatalf("runLand returned error: %v", err)
	}
	after := runGitOutput(t, dir, "rev-parse", "main")
	if after == before {
		t.Fatal("expected main to advance after landing, but it did not")
	}
	if _, err := os.Stat(filepath.Join(dir, "agent-file.txt")); err != nil {
		t.Errorf("expected landed file on main, got: %v", err)
	}
	log := runGitOutput(t, dir, "log", "--oneline", "-1", "main")
	if !strings.Contains(log, "Land agent/x") {
		t.Errorf("expected 'Land agent/x' in merge commit, got: %q", log)
	}
}

func TestRunLandRefusesWhenTargetNotCheckedOut(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir) // leaves HEAD on "feature"

	wt := filepath.Join(t.TempDir(), "agent-x")
	runGit(t, dir, "worktree", "add", "-b", "agent/x", wt, "main")
	if err := os.WriteFile(filepath.Join(wt, "agent-file.txt"), []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "agent work")

	before := runGitOutput(t, dir, "rev-parse", "main")
	err := runLand("main", wt, []string{}, false, false)
	if err == nil {
		t.Fatal("expected error when target is not checked out, got nil")
	}
	if !strings.Contains(err.Error(), "cannot land onto main") {
		t.Errorf("expected 'cannot land onto main' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "feature") {
		t.Errorf("expected checked-out branch 'feature' named in error, got: %v", err)
	}
	if after := runGitOutput(t, dir, "rev-parse", "main"); after != before {
		t.Errorf("expected main to be unchanged, got %s -> %s", before, after)
	}
}

func TestRunLandRefusesDirtyWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	wt := filepath.Join(t.TempDir(), "agent-x")
	runGit(t, dir, "worktree", "add", "-b", "agent/x", wt, "main")
	if err := os.WriteFile(filepath.Join(wt, "uncommitted.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := runGitOutput(t, dir, "rev-parse", "main")
	err := runLand("main", wt, []string{}, false, false)
	if err == nil {
		t.Fatal("expected error for dirty worktree, got nil")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected 'uncommitted changes' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), wt) {
		t.Errorf("expected worktree path in error, got: %v", err)
	}
	if after := runGitOutput(t, dir, "rev-parse", "main"); after != before {
		t.Errorf("expected main to be unchanged, got %s -> %s", before, after)
	}
}

func TestRunLandFailingGateBlocksMerge(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	wt := filepath.Join(t.TempDir(), "agent-x")
	runGit(t, dir, "worktree", "add", "-b", "agent/x", wt, "main")
	if err := os.WriteFile(filepath.Join(wt, "agent-file.txt"), []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wt, "add", ".")
	runGit(t, wt, "commit", "-m", "agent work")

	before := runGitOutput(t, dir, "rev-parse", "main")
	err := runLand("main", wt, []string{"exit 1"}, false, false)
	if err == nil {
		t.Fatal("expected error for failing gate, got nil")
	}
	if !strings.Contains(err.Error(), "gate") {
		t.Errorf("expected gate-related error, got: %v", err)
	}
	if after := runGitOutput(t, dir, "rev-parse", "main"); after != before {
		t.Errorf("expected main to be unchanged after failing gate, got %s -> %s", before, after)
	}
}
