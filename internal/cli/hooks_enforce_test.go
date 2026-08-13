package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newWorktree creates a real git repository with a linked worktree and
// returns the paths of both. It skips the test if the git binary is
// unavailable. The worktree is created with --detach so that the main
// checkout and the worktree share no branch state.
func newWorktree(t *testing.T) (mainRepo, worktree string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	mainRepo = t.TempDir()
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	runGit(mainRepo, "init", "-q", "-b", "main")
	runGit(mainRepo, "config", "user.email", "test@example.com")
	runGit(mainRepo, "config", "user.name", "Test")
	runGit(mainRepo, "commit", "-q", "--allow-empty", "-m", "init")
	worktree = filepath.Join(t.TempDir(), "wt")
	runGit(mainRepo, "worktree", "add", "-q", "--detach", worktree, "main")
	return mainRepo, worktree
}

func TestInstallRoleEnforceHookAlreadyPresent(t *testing.T) {
	_, worktree := newWorktree(t)

	// Simulate the normal path: the worktree already has an executable
	// .mill/checks/role-enforce (materialised by git worktree add).
	wtEnforce := filepath.Join(worktree, ".mill", "checks", "role-enforce")
	if err := os.MkdirAll(filepath.Dir(wtEnforce), 0o755); err != nil {
		t.Fatalf("mkdir worktree .mill/checks: %v", err)
	}
	preExisting := []byte("#!/bin/sh\npre-existing\n")
	if err := os.WriteFile(wtEnforce, preExisting, 0o755); err != nil {
		t.Fatalf("write pre-existing role-enforce: %v", err)
	}

	if err := installRoleEnforceHook(worktree); err != nil {
		t.Fatalf("installRoleEnforceHook with file already present: %v", err)
	}
	got, err := os.ReadFile(wtEnforce)
	if err != nil {
		t.Fatalf("read worktree role-enforce: %v", err)
	}
	if string(got) != string(preExisting) {
		t.Errorf("already-present role-enforce contents = %q, want unchanged %q", got, preExisting)
	}
}

func TestInstallRoleEnforceHookMissingSource(t *testing.T) {
	_, worktree := newWorktree(t)

	err := installRoleEnforceHook(worktree)
	if err == nil {
		t.Fatal("installRoleEnforceHook returned nil, want error when role-enforce is missing everywhere")
	}
	msg := err.Error()
	wtPath := filepath.Join(worktree, ".mill", "checks", "role-enforce")
	if !strings.Contains(msg, wtPath) {
		t.Errorf("error %q should mention worktree path %q", msg, wtPath)
	}
	if !strings.Contains(msg, "role-enforce") {
		t.Errorf("error %q should mention the role-enforce file", msg)
	}
}

func TestInstallRoleEnforceHookCopyPath(t *testing.T) {
	mainRepo := t.TempDir()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	runGit(mainRepo, "init", "-q", "-b", "main")
	runGit(mainRepo, "config", "user.email", "test@example.com")
	runGit(mainRepo, "config", "user.name", "Test")
	runGit(mainRepo, "commit", "-q", "--allow-empty", "-m", "init")

	// The main repository has .mill/checks/role-enforce; a worktree created
	// from this commit would materialise the tracked file.
	mainEnforce := filepath.Join(mainRepo, ".mill", "checks", "role-enforce")
	if err := os.MkdirAll(filepath.Dir(mainEnforce), 0o755); err != nil {
		t.Fatalf("mkdir main .mill/checks: %v", err)
	}
	want := []byte("#!/bin/sh\nmain-version\n")
	if err := os.WriteFile(mainEnforce, want, 0o755); err != nil {
		t.Fatalf("write main role-enforce: %v", err)
	}
	runGit(mainRepo, "add", ".mill/checks/role-enforce")
	runGit(mainRepo, "commit", "-q", "-m", "add role-enforce")

	worktree := filepath.Join(t.TempDir(), "wt")
	runGit(mainRepo, "worktree", "add", "-q", "--detach", worktree, "main")

	// Simulate a stale worktree that predates the tracked file: remove the
	// materialised copy so the fallback must copy from the main repo.
	wtEnforce := filepath.Join(worktree, ".mill", "checks", "role-enforce")
	if err := os.Remove(wtEnforce); err != nil {
		t.Fatalf("remove tracked role-enforce from worktree: %v", err)
	}

	if err := installRoleEnforceHook(worktree); err != nil {
		t.Fatalf("installRoleEnforceHook copy path: %v", err)
	}
	got, err := os.ReadFile(wtEnforce)
	if err != nil {
		t.Fatalf("read copied role-enforce: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("copied role-enforce contents = %q, want %q", got, want)
	}
	info, err := os.Stat(wtEnforce)
	if err != nil {
		t.Fatalf("stat copied role-enforce: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("copied role-enforce mode = %v, want 0755", perm)
	}
}
