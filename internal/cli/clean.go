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
// With --all, it removes ALL worktrees (same PID check) and deletes state
// files (state.json + backups) for a factory reset.
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

	for _, task := range s.Tasks {
		worktreeDir := a.worktreePath(task.Issue)

		// Only clean terminal tasks unless --all.
		if !*all && task.Status != domain.TaskDone && task.Status != domain.TaskError {
			continue
		}

		// Check PID liveness — never delete a running worktree.
		if a.isWorktreeRunning(worktreeDir) {
			skipped++
			continue
		}

		// Remove the worktree directory.
		if err := os.RemoveAll(worktreeDir); err != nil {
			fmt.Fprintf(a.Err, "warning: failed to remove worktree %s: %v\n", worktreeDir, err)
			continue
		}
		cleaned++
	}

	// For --all, also remove state files and backups.
	if *all {
		for _, ext := range []string{"", ".1", ".2"} {
			p := a.statePath() + ext
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(a.Err, "warning: failed to remove %s: %v\n", p, err)
			}
		}
	}

	// Prune merged agent branches.
	pruned := pruneMergedAgentBranches(a.Err)

	if cleaned == 0 && skipped == 0 && pruned == 0 {
		fmt.Fprintln(a.Out, "Nothing to clean")
	} else {
		parts := []string{fmt.Sprintf("Cleaned %d worktrees", cleaned)}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped (running)", skipped))
		}
		if pruned > 0 {
			parts = append(parts, fmt.Sprintf("%d branches pruned", pruned))
		}
		fmt.Fprintln(a.Out, strings.Join(parts, ", "))
	}

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

// pruneMergedAgentBranches deletes merged agent/* branches.
// It returns the count of successfully pruned branches.
// Errors are logged to errW; the count reflects only confirmed deletions.
func pruneMergedAgentBranches(errW io.Writer) int {
	cmd := exec.Command("git", "branch", "--merged")
	out, err := cmd.Output()
	if err != nil {
		// Not a git repo, or git not available — nothing to prune.
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

func printCleanUsage(w interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(w, `Usage: mill clean [--all]

Remove completed or failed worktrees. Running worktrees (agent PID alive)
are never removed.

  clean            Remove worktrees for done/error tasks
  clean --all      Factory reset: remove ALL worktrees and state files

Running detection: reads .mill/agent.pid and probes with signal 0.
State files removed (--all): state.json, state.json.1, state.json.2
`)
}
