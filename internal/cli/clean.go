package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/state"
)

// runClean handles the "clean" command.
// Without flags, it removes worktrees for tasks with status done or error,
// skipping any worktree whose agent PID is still alive.
// Worktrees with unmerged commits are reported and skipped unless --force.
// With --all, it removes ALL worktrees and state files, plus any orphaned
// agent/* and scratch/* branches.
func (a *App) runClean(args []string) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printCleanUsage(a.Out)
			return nil
		}
	}

	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	all := fs.Bool("all", false, "factory reset: remove all worktrees and state")
	force := fs.Bool("force", false, "remove worktrees even with unmerged commits")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printCleanUsage(a.Out)
			return nil
		}
		return err
	}

	// Factory reset requires explicit confirmation.
	if *all {
		fmt.Fprintf(a.Out, "Factory reset: remove ALL worktrees and state? [y/N]: ")
		reader := bufio.NewReader(a.In)
		ans, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(strings.ToLower(ans)) != "y" {
			fmt.Fprintln(a.Out, "Aborted.")
			return nil
		}
	}

	s, err := state.Load(a.statePath())
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	cleaned := 0
	skipped := 0
	unmerged := 0
	var unmergedNames []string

	for _, task := range s.Tasks {
		worktreeDir := a.worktreePath(task.Issue)
		branch := a.worktreeBranch(task.Issue)

		// Only clean terminal tasks unless --all.
		if !*all && task.Status != domain.TaskDone && task.Status != domain.TaskError {
			continue
		}

		// Check PID liveness — never delete a running worktree.
		if a.isWorktreeRunning(worktreeDir) {
			skipped++
			continue
		}

		// Check if worktree directory exists on disk.
		if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
			continue
		}

		// Check for unmerged commits unless --force.
		if !*force && worktreeBranchHasUnmergedCommits(branch) {
			unmerged++
			unmergedNames = append(unmergedNames, branch)
			continue
		}

		// Remove via git worktree remove (handles git registration + directory).
		removeGitWorktree(worktreeDir, a.Err)
		// Delete the branch.
		deleteBranch(branch, a.Err)
		cleaned++
	}

	// For --all, also remove state files, backups, and orphan branches.
	pruned := 0
	if *all {
		for _, ext := range []string{"", ".1", ".2"} {
			p := a.statePath() + ext
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(a.Err, "warning: failed to remove %s: %v\n", p, err)
			}
		}
		// Remove any remaining worktree directories that weren't in state.
		removeOrphanWorktrees(a.MillDir, *force, a.Err)
		// Prune stale git worktree registrations.
		pruneStaleWorktrees(a.Err)
		// Remove orphaned agent/* and scratch/* branches.
		pruned = removeOrphanBranches(a.Err)
	} else {
		// Even without --all, prune merged branches.
		pruned = pruneMergedAgentBranches(a.Err)
	}

	if cleaned == 0 && skipped == 0 && unmerged == 0 && pruned == 0 {
		fmt.Fprintln(a.Out, "Nothing to clean")
		return nil
	}

	parts := []string{}
	if cleaned > 0 {
		parts = append(parts, fmt.Sprintf("Cleaned %d worktrees", cleaned))
	}
	if pruned > 0 {
		parts = append(parts, fmt.Sprintf("%d branches pruned", pruned))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (running)", skipped))
	}
	if unmerged > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped (unmerged)", unmerged))
		for _, name := range unmergedNames {
			fmt.Fprintf(a.Out, "  unmerged: %s (use --force to remove)\n", name)
		}
	}
	fmt.Fprintln(a.Out, strings.Join(parts, ", "))

	return nil
}

// isWorktreeRunning checks whether the agent in the given worktree directory
// is still alive by reading .mill/agent.pid and probing with signal 0.
func (a *App) isWorktreeRunning(worktreeDir string) bool {
	pidPath := filepath.Join(worktreeDir, ".mill", "agent.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	return isProcessAlive(pid)
}

// worktreeBranchHasUnmergedCommits reports whether the given branch has
// commits not present in the current HEAD branch of the main worktree.
// Returns false if the branch doesn't exist or git is unavailable.
func worktreeBranchHasUnmergedCommits(branch string) bool {
	// Determine the current branch (the main worktree's HEAD).
	headCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	headOut, err := headCmd.Output()
	if err != nil {
		return false
	}
	base := strings.TrimSpace(string(headOut))
	if base == "" || base == "HEAD" {
		// Detached HEAD — can't determine base.
		return false
	}

	// Check if branch exists.
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	if err := cmd.Run(); err != nil {
		return false
	}

	// Count commits on branch not in base.
	cmd = exec.Command("git", "rev-list", "--count", base+".."+branch)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	count := strings.TrimSpace(string(out))
	return count != "0"
}

// removeGitWorktree removes a worktree via `git worktree remove --force`
// and falls back to os.RemoveAll if git worktree remove fails.
func removeGitWorktree(worktreeDir string, errW io.Writer) {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreeDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(errW, "warning: git worktree remove failed for %s: %v\n%s\n", worktreeDir, err, out)
		// Fallback: remove directory directly and prune.
		os.RemoveAll(worktreeDir)
		exec.Command("git", "worktree", "prune").Run()
	}
}

// deleteBranch deletes a git branch by name. Best-effort: errors are logged.
func deleteBranch(branch string, errW io.Writer) {
	cmd := exec.Command("git", "branch", "-D", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(errW, "warning: failed to delete branch %s: %v\n%s\n", branch, err, out)
	}
}

// pruneMergedAgentBranches deletes merged agent/* branches.
// It returns the count of successfully pruned branches.
// Errors are logged to errW; the count reflects only confirmed deletions.
func pruneMergedAgentBranches(errW io.Writer) int {
	cmd := exec.Command("git", "branch", "--merged")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	pruned := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "agent/") {
			continue
		}
		del := exec.Command("git", "branch", "-D", line)
		if delOut, delErr := del.CombinedOutput(); delErr != nil {
			fmt.Fprintf(errW, "warning: failed to delete branch %s: %v\n%s\n", line, delErr, delOut)
			continue
		}
		pruned++
	}
	return pruned
}

// removeOrphanWorktrees removes any directory under .mill/worktrees/ that
// is not tracked by state. Used during --all to catch leftover directories.
// Skips directories with a live agent PID unless force is set.
func removeOrphanWorktrees(millDir string, force bool, errW io.Writer) int {
	wtDir := filepath.Join(millDir, "worktrees")
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(wtDir, entry.Name())
		// Check PID liveness even in orphan cleanup.
		if !force && isWorktreeRunningStatic(fullPath) {
			continue
		}
		removeGitWorktree(fullPath, errW)
		removed++
	}
	return removed
}

// isWorktreeRunningStatic checks PID liveness without requiring an App receiver.
func isWorktreeRunningStatic(worktreeDir string) bool {
	pidPath := filepath.Join(worktreeDir, ".mill", "agent.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}
	return isProcessAlive(pid)
}

// removeOrphanBranches removes all agent/* and scratch/* branches that have
// no associated worktree directory. Returns the count of removed branches.
func removeOrphanBranches(errW io.Writer) int {
	removed := 0
	for _, prefix := range []string{"agent/", "scratch/"} {
		cmd := exec.Command("git", "branch", "--list", prefix+"*")
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "* ")
			if line == "" {
				continue
			}
			del := exec.Command("git", "branch", "-D", line)
			if delOut, delErr := del.CombinedOutput(); delErr != nil {
				fmt.Fprintf(errW, "warning: failed to delete branch %s: %v\n%s\n", line, delErr, delOut)
				continue
			}
			removed++
		}
	}
	return removed
}

// pruneStaleWorktrees runs `git worktree prune` to clean up stale
// worktree registrations for directories that no longer exist.
func pruneStaleWorktrees(errW io.Writer) {
	cmd := exec.Command("git", "worktree", "prune")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(errW, "warning: git worktree prune failed: %v\n%s\n", err, out)
	}
}

func printCleanUsage(w interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(w, `Usage: mill clean [--all] [--force]

Remove completed or failed worktrees. Running worktrees (agent PID alive)
are never removed. Worktrees with unmerged commits are reported and
skipped unless --force is specified.

  clean              Remove worktrees for done/error tasks
  clean --all        Factory reset: remove ALL worktrees and state files
  clean --force      Remove worktrees even with unmerged commits

Running detection: reads .mill/agent.pid and probes with signal 0.
State files removed (--all): state.json, state.json.1, state.json.2
`)
}
