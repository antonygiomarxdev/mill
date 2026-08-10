package cli

import (
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

	err := runLand("main", dir, []string{}, false)
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

	err := runLand("main", dir, []string{}, false)
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
	err := runLand("nonexistent-branch", dir, []string{}, false)
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
