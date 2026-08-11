package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLandLockedByOtherWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Create a second worktree that holds main checked out.
	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")

	// primary worktree is already on "feature" from setupTestGitRepo.

	err := runLand("main", dir, []string{}, false, false)
	if err == nil {
		// Clean up before failing.
		runGit(t, dir, "worktree", "remove", otherDir)
		t.Fatal("expected error due to locked branch, got nil")
	}
	// Clean up the second worktree.
	runGit(t, dir, "worktree", "remove", otherDir)

	if !strings.Contains(err.Error(), "locked by another worktree") {
		t.Errorf("expected 'locked by another worktree' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), otherDir) {
		t.Errorf("expected locking worktree path %q in error, got: %v", otherDir, err)
	}
}

func TestRunLandLockedPrintsResolution(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")

	err := runLand("main", dir, []string{}, false, false)
	if err == nil {
		runGit(t, dir, "worktree", "remove", otherDir)
		t.Fatal("expected error due to locked branch, got nil")
	}
	runGit(t, dir, "worktree", "remove", otherDir)

	if !strings.Contains(err.Error(), "resolve: cd ") {
		t.Errorf("expected 'resolve: cd ' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "git checkout") {
		t.Errorf("expected 'git checkout' in error, got: %v", err)
	}
}

func TestRunLandCheckoutGenericFailure(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Try to checkout a branch that does not exist — no lock involved.
	err := runLand("nonexistent-branch", dir, []string{}, false, false)
	if err == nil {
		t.Fatal("expected checkout failure error, got nil")
	}
	if !strings.Contains(err.Error(), "checkout failed") {
		t.Errorf("expected 'checkout failed' in error, got: %v", err)
	}
	if strings.Contains(err.Error(), "locked by another worktree") {
		t.Errorf("expected no lock message for nonexistent branch, got: %v", err)
	}
}

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

func TestDetectWorktreeLockLiveProcess(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")

	// Write PID file pointing to our own (alive) process.
	writePidFile(t, otherDir, os.Getpid())

	err := runLand("main", dir, []string{}, false, false)
	if err == nil {
		runGit(t, dir, "worktree", "remove", "--force", otherDir)
		t.Fatal("expected error due to live lock, got nil")
	}
	runGit(t, dir, "worktree", "remove", "--force", otherDir)

	if !strings.Contains(err.Error(), "locked by another worktree") {
		t.Errorf("expected 'locked by another worktree' for live lock, got: %v", err)
	}
	if strings.Contains(err.Error(), "stale") {
		t.Errorf("expected no 'stale' in live lock error, got: %v", err)
	}
}

func TestDetectWorktreeLockStaleProcess(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")

	// Write PID file pointing to a dead PID.
	writePidFile(t, otherDir, 99999)

	err := runLand("main", dir, []string{}, false, false)
	if err == nil {
		runGit(t, dir, "worktree", "remove", "--force", otherDir)
		t.Fatal("expected stale lock error, got nil")
	}
	runGit(t, dir, "worktree", "remove", "--force", otherDir)

	errStr := err.Error()
	if !strings.Contains(errStr, "stale lock") {
		t.Errorf("expected 'stale lock' in error, got: %v", err)
	}
	if !strings.Contains(errStr, "agent pid 99999 not running") {
		t.Errorf("expected 'agent pid 99999 not running' in error, got: %v", err)
	}
	if !strings.Contains(errStr, "Use --force to clear") {
		t.Errorf("expected 'Use --force to clear' in error, got: %v", err)
	}
}

func TestDetectWorktreeLockStaleWithForce(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")

	writePidFile(t, otherDir, 99999)

	err := runLand("main", dir, []string{}, false, true)
	// Force should bypass the stale lock — checkout succeeds.
	runGit(t, dir, "worktree", "remove", "--force", otherDir)
	if err != nil {
		t.Fatalf("expected force to bypass stale lock, got: %v", err)
	}
}

func TestDetectWorktreeLockNoPidFile(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")
	// Do NOT write a PID file — simulate pre-#72 worktree.

	err := runLand("main", dir, []string{}, false, false)
	if err == nil {
		runGit(t, dir, "worktree", "remove", "--force", otherDir)
		t.Fatal("expected error for missing PID file, got nil")
	}
	runGit(t, dir, "worktree", "remove", "--force", otherDir)

	errStr := err.Error()
	if !strings.Contains(errStr, "unknown liveness") {
		t.Errorf("expected 'unknown liveness' in error, got: %v", err)
	}
	if !strings.Contains(errStr, "locked by another worktree") {
		t.Errorf("expected 'locked by another worktree' in error, got: %v", err)
	}
}

func TestDetectWorktreeLockNoMatchingBranch(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// branch "nonexistent" is not held by any worktree.
	err := runLand("nonexistent-branch", dir, []string{}, false, false)
	if err == nil {
		t.Fatal("expected checkout failure error, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "checkout failed") {
		t.Errorf("expected 'checkout failed' in error, got: %v", err)
	}
	if strings.Contains(errStr, "stale") || strings.Contains(errStr, "locked by another worktree") {
		t.Errorf("expected no lock message for nonexistent branch, got: %v", err)
	}
}

func TestRunLandForceFlag(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	otherDir := filepath.Join(t.TempDir(), "other-worktree")
	runGit(t, dir, "worktree", "add", otherDir, "main")

	// Write stale PID file in the locking worktree.
	writePidFile(t, otherDir, 99999)

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("land", "--force", "--worktree", dir, "main")
	runGit(t, dir, "worktree", "remove", "--force", otherDir)
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
