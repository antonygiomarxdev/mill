package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
)

func TestCleanRemovesDoneTask(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")

	// Create state with a done task.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create worktree directory.
	wt := filepath.Join(d, "worktrees", "issue-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean(nil); err != nil {
		t.Fatalf("runClean failed: %v", err)
	}

	// Worktree should be removed.
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("expected done task worktree to be removed")
	}
}

func TestCleanSkipsRunningTask(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")

	// Create state with a running task.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskRunning})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create worktree directory.
	wt := filepath.Join(d, "worktrees", "issue-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean(nil); err != nil {
		t.Fatalf("runClean failed: %v", err)
	}

	// Worktree should NOT be removed (task is running, not done/error).
	if _, err := os.Stat(wt); os.IsNotExist(err) {
		t.Error("expected running task worktree to be preserved")
	}
}

func TestCleanAllRemovesEverything(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")

	// Create state with a mix of tasks.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	s.UpsertTask(domain.Task{ID: "t2", Issue: 2, Status: domain.TaskRunning})
	s.UpsertTask(domain.Task{ID: "t3", Issue: 3, Status: domain.TaskPending})
	statePath := filepath.Join(d, "state.json")
	if err := s.Save(statePath); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create worktree directories for all tasks.
	for i := 1; i <= 3; i++ {
		wt := filepath.Join(d, "worktrees", fmt.Sprintf("issue-%d", i))
		if err := os.MkdirAll(wt, 0755); err != nil {
			t.Fatalf("failed to create worktree dir %d: %v", i, err)
		}
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf, In: strings.NewReader("y\n")}
	if err := app.runClean([]string{"--all"}); err != nil {
		t.Fatalf("runClean --all failed: %v", err)
	}

	// All worktrees should be removed.
	for i := 1; i <= 3; i++ {
		wt := filepath.Join(d, "worktrees", fmt.Sprintf("issue-%d", i))
		if _, err := os.Stat(wt); !os.IsNotExist(err) {
			t.Errorf("expected worktree issue-%d to be removed", i)
		}
	}

	// State files should be removed.
	for _, ext := range []string{"", ".1", ".2"} {
		p := statePath + ext
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", p)
		}
	}
}

func TestCleanAllSkipsAliveProcess(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")

	// Create state with a done task.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create worktree with a PID file pointing to this process.
	wt := filepath.Join(d, "worktrees", "issue-1")
	writePidFile(t, wt, os.Getpid())

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf, In: strings.NewReader("y\n")}
	if err := app.runClean([]string{"--all"}); err != nil {
		t.Fatalf("runClean --all failed: %v", err)
	}

	// Worktree should NOT be removed (current process is alive).
	if _, err := os.Stat(wt); os.IsNotExist(err) {
		t.Error("expected worktree with alive PID to be preserved")
	}

	// Output should mention skipped.
	output := buf.String()
	if !strings.Contains(output, "skipped") {
		t.Errorf("expected output to mention skipped, got: %s", output)
	}
}

func TestCleanPrintsSummary(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")

	// Create state with two tasks: one done, one error.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	s.UpsertTask(domain.Task{ID: "t2", Issue: 2, Status: domain.TaskError})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create worktree directories.
	for i := 1; i <= 2; i++ {
		wt := filepath.Join(d, "worktrees", fmt.Sprintf("issue-%d", i))
		if err := os.MkdirAll(wt, 0755); err != nil {
			t.Fatalf("failed to create worktree dir %d: %v", i, err)
		}
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean(nil); err != nil {
		t.Fatalf("runClean failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cleaned 2 worktrees") {
		t.Errorf("expected 'Cleaned 2 worktrees', got: %s", output)
	}
}

func TestCleanAllConfirmationDenied(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, ".mill")

	// Create state with a done task.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t1", Issue: 1, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create worktree directory.
	wt := filepath.Join(d, "worktrees", "issue-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf, In: strings.NewReader("n\n")}
	if err := app.runClean([]string{"--all"}); err != nil {
		t.Fatalf("runClean --all failed: %v", err)
	}

	// Should print "Aborted."
	output := buf.String()
	if !strings.Contains(output, "Aborted.") {
		t.Errorf("expected 'Aborted.' in output, got: %s", output)
	}

	// Worktree should NOT be removed (user declined).
	if _, err := os.Stat(wt); os.IsNotExist(err) {
		t.Error("expected worktree to be preserved when user declines")
	}
}

func TestPrintCleanUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	printCleanUsage(buf)
	output := buf.String()
	if !strings.Contains(output, "Usage: mill clean") {
		t.Errorf("expected 'Usage: mill clean' in output, got: %s", output)
	}
	if !strings.Contains(output, "--all") {
		t.Errorf("expected '--all' in output, got: %s", output)
	}
}

func TestCleanPrunesMergedBranches(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	// Create an agent branch, commit something, merge it back to main.
	runGit(t, dir, "checkout", "-b", "agent/42")
	if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte("done"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "work.txt")
	runGit(t, dir, "commit", "-m", "agent work")
	// Merge back to feature (current branch after setupTestGitRepo).
	runGit(t, dir, "checkout", "feature")
	runGit(t, dir, "merge", "agent/42", "--no-ff", "-m", "merge agent/42")

	// Verify branch exists before clean.
	cmd := exec.Command("git", "branch", "--list", "agent/42")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if !strings.Contains(string(out), "agent/42") {
		t.Fatal("expected branch agent/42 to exist before clean")
	}

	d := filepath.Join(dir, ".mill")
	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean(nil); err != nil {
		t.Fatalf("runClean failed: %v", err)
	}

	// Branch should be pruned.
	cmd = exec.Command("git", "branch", "--list", "agent/42")
	cmd.Dir = dir
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list after clean failed: %v", err)
	}
	if strings.Contains(string(out), "agent/42") {
		t.Error("expected branch agent/42 to be pruned after clean")
	}

	// Output should mention pruned branches.
	output := buf.String()
	if !strings.Contains(output, "1 branches pruned") {
		t.Errorf("expected '1 branches pruned' in output, got: %s", output)
	}
}

func TestRunCleanManualHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}

	err := app.runClean([]string{"-h"})
	if err != nil {
		t.Fatalf("runClean -h returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Usage: mill clean") {
		t.Errorf("expected usage text, got: %s", output)
	}
}

func TestRunCleanFlagHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}

	err := app.runClean([]string{"--help"})
	if err != nil {
		t.Fatalf("runClean --help returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Usage: mill clean") {
		t.Errorf("expected usage text, got: %s", output)
	}
}

func TestCleanRefusesUnmergedWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	d := filepath.Join(dir, ".mill")

	// Create a real git worktree on branch agent/7.
	wtDir := filepath.Join(d, "worktrees", "issue-7")
	cmd := exec.Command("git", "worktree", "add", "-b", "agent/7", wtDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}

	// Make an unmerged commit in the worktree.
	if err := os.WriteFile(filepath.Join(wtDir, "unmerged.txt"), []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "unmerged.txt")
	runGit(t, wtDir, "commit", "-m", "unmerged work")

	// Create state with a done task for issue 7.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t7", Issue: 7, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean(nil); err != nil {
		t.Fatalf("runClean failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "unmerged") {
		t.Errorf("expected output to mention unmerged, got: %s", output)
	}
	if !strings.Contains(output, "--force") {
		t.Errorf("expected output to mention --force, got: %s", output)
	}

	// Worktree should still exist.
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Error("expected worktree with unmerged commits to be preserved")
	}

	// Branch should still exist.
	branchCmd := exec.Command("git", "branch", "--list", "agent/7")
	branchCmd.Dir = dir
	out, _ := branchCmd.Output()
	if !strings.Contains(string(out), "agent/7") {
		t.Error("expected branch agent/7 to still exist")
	}
}

func TestCleanForceRemovesUnmergedWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	d := filepath.Join(dir, ".mill")

	// Create a real git worktree on branch agent/8.
	wtDir := filepath.Join(d, "worktrees", "issue-8")
	cmd := exec.Command("git", "worktree", "add", "-b", "agent/8", wtDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}

	// Make an unmerged commit in the worktree.
	if err := os.WriteFile(filepath.Join(wtDir, "unmerged.txt"), []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "unmerged.txt")
	runGit(t, wtDir, "commit", "-m", "unmerged work")

	// Create state with a done task for issue 8.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t8", Issue: 8, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean([]string{"--force"}); err != nil {
		t.Fatalf("runClean --force failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cleaned 1 worktrees") {
		t.Errorf("expected 'Cleaned 1 worktrees', got: %s", output)
	}

	// Worktree should be removed.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("expected worktree to be removed with --force")
	}

	// Branch should be deleted.
	branchCmd := exec.Command("git", "branch", "--list", "agent/8")
	branchCmd.Dir = dir
	out, _ := branchCmd.Output()
	if strings.Contains(string(out), "agent/8") {
		t.Error("expected branch agent/8 to be deleted with --force")
	}
}

func TestCleanRemovesMergedWorktree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	d := filepath.Join(dir, ".mill")

	// Create a real git worktree on branch agent/9.
	wtDir := filepath.Join(d, "worktrees", "issue-9")
	cmd := exec.Command("git", "worktree", "add", "-b", "agent/9", wtDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}

	// Make a commit and merge it back.
	if err := os.WriteFile(filepath.Join(wtDir, "merged.txt"), []byte("done"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "merged.txt")
	runGit(t, wtDir, "commit", "-m", "merged work")
	runGit(t, dir, "merge", "agent/9", "--no-ff", "-m", "merge agent/9")

	// Create state with a done task for issue 9.
	s := state.New()
	s.UpsertTask(domain.Task{ID: "t9", Issue: 9, Status: domain.TaskDone})
	if err := s.Save(filepath.Join(d, "state.json")); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf}
	if err := app.runClean(nil); err != nil {
		t.Fatalf("runClean failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cleaned 1 worktrees") {
		t.Errorf("expected 'Cleaned 1 worktrees', got: %s", output)
	}

	// Worktree should be removed.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("expected merged worktree to be removed")
	}
}

func TestCleanAllRemovesOrphanBranches(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)

	d := filepath.Join(dir, ".mill")

	// Create orphan agent/* and scratch/* branches.
	runGit(t, dir, "checkout", "-b", "agent/100")
	runGit(t, dir, "checkout", "feature")
	runGit(t, dir, "checkout", "-b", "agent/200")
	runGit(t, dir, "checkout", "feature")
	runGit(t, dir, "checkout", "-b", "scratch/100")
	runGit(t, dir, "checkout", "feature")

	// Create empty state (no tasks).
	s := state.New()
	statePath := filepath.Join(d, "state.json")
	if err := s.Save(statePath); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create the worktrees directory so ReadDir doesn't fail.
	os.MkdirAll(filepath.Join(d, "worktrees"), 0755)

	buf := new(bytes.Buffer)
	app := &App{MillDir: d, Out: buf, Err: buf, In: strings.NewReader("y\n")}
	if err := app.runClean([]string{"--all"}); err != nil {
		t.Fatalf("runClean --all failed: %v", err)
	}

	// Orphan branches should be removed.
	for _, branch := range []string{"agent/100", "agent/200", "scratch/100"} {
		cmd := exec.Command("git", "branch", "--list", branch)
		cmd.Dir = dir
		out, _ := cmd.Output()
		if strings.Contains(string(out), branch) {
			t.Errorf("expected orphan branch %s to be removed", branch)
		}
	}
}
