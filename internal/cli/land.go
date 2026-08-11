package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// runLand runs gate commands in a worktree, optionally confirms with the user,
// and checks out the target branch.
func runLand(target string, worktree string, gates []string, confirm bool, force bool) error {
	for _, gate := range gates {
		cmd := exec.Command("sh", "-c", gate)
		cmd.Dir = worktree
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gate %s failed", gate)
		}
	}
	if confirm {
		fmt.Printf("Merge to %s? [y/N]: ", target)
		r := bufio.NewReader(os.Stdin)
		ans, _ := r.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(ans)) != "y" {
			return nil
		}
	}
	cmd := exec.Command("git", "-C", worktree, "checkout", target)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return detectWorktreeLock(worktree, target, err, force)
	}
	return nil
}

// detectWorktreeLock checks if a checkout failure is due to another worktree
// holding the target branch. It distinguishes active locks (agent process alive)
// from stale locks (agent process dead) by reading the .mill/agent.pid file
// from the locking worktree and probing the PID's liveness.
//
// When force is true, stale locks are bypassed with a warning.
// When force is false, stale locks return an actionable error suggesting --force.
// Active locks always block regardless of force.
func detectWorktreeLock(worktree, target string, checkoutErr error, force bool) error {
	listArgs := []string{"worktree", "list"}
	if worktree != "" {
		listArgs = append([]string{"-C", worktree}, listArgs...)
	}
	out, err := exec.Command("git", listArgs...).Output()
	if err != nil {
		return fmt.Errorf("checkout failed")
	}

	// Parse git worktree list output.
	// Format: <path> <HEAD-hash> [<branch>] or <path> <HEAD-hash> (detached HEAD)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Branch tracking lines contain "[<branch>]"
		if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
			continue
		}

		// Extract branch name between [ and ]
		bracketStart := strings.Index(line, "[")
		bracketEnd := strings.Index(line, "]")
		if bracketStart < 0 || bracketEnd <= bracketStart {
			continue
		}
		branch := line[bracketStart+1 : bracketEnd]

		if branch != target {
			continue
		}

		// Extract path (first whitespace-separated field)
		parts := strings.Fields(line[:bracketStart])
		if len(parts) < 1 {
			continue
		}
		lockingPath := parts[0]

		// Check PID liveness for stale-lock detection.
		return classifyLock(lockingPath, target, force)
	}

	return fmt.Errorf("checkout failed")
}

// classifyLock reads the agent PID file from the locking worktree and determines
// whether the lock is active, stale, or of unknown liveness.
func classifyLock(lockingPath, target string, force bool) error {
	pidPath := filepath.Join(lockingPath, ".mill", "agent.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		// No PID file — conservative: block with unknown liveness.
		return fmt.Errorf(
			"land: cannot checkout '%s' — locked by another worktree (unknown liveness)\n  locking worktree: %s\n  resolve: cd %s && git checkout <other-branch>",
			target, lockingPath, lockingPath,
		)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf(
			"land: cannot checkout '%s' — locked by another worktree (unreadable PID in %s)\n  locking worktree: %s\n  resolve: cd %s && git checkout <other-branch>",
			target, pidPath, lockingPath, lockingPath,
		)
	}

	if isProcessAlive(pid) {
		// Active lock — always block.
		return fmt.Errorf(
			"land: cannot checkout '%s' — locked by another worktree\n  locking worktree: %s\n  resolve: cd %s && git checkout <other-branch>",
			target, lockingPath, lockingPath,
		)
	}

	// Stale lock — agent process is dead.
	if force {
		fmt.Fprintf(os.Stderr, "warning: forcing checkout past stale lock (PID %d is dead)\n", pid)
		return nil
	}

	return fmt.Errorf(
		"stale lock from %s (agent pid %d not running). Use --force to clear.",
		lockingPath, pid,
	)
}

// isProcessAlive checks whether a process with the given PID is still running.
// Uses os.FindProcess + Signal(syscall.Signal(0)) — the null signal.
// Returns false if the process doesn't exist, the signal fails, or the check
// takes longer than 2 seconds (non-blocking safety).
func isProcessAlive(pid int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		p, err := os.FindProcess(pid)
		if err != nil {
			done <- false
			return
		}
		err = p.Signal(syscall.Signal(0))
		done <- err == nil
	}()

	select {
	case alive := <-done:
		return alive
	case <-ctx.Done():
		return false
	}
}

// runLand handles the "land" command.
func (a *App) runLand(args []string) error {
	fs := flag.NewFlagSet("land", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var worktree string
	var confirm bool
	var force bool
	fs.StringVar(&worktree, "worktree", "", "worktree directory")
	fs.BoolVar(&confirm, "confirm", false, "prompt before merging")
	fs.BoolVar(&force, "force", false, "bypass stale worktree locks")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	fsArgs := fs.Args()
	if len(fsArgs) < 1 {
		fs.Usage()
		return fmt.Errorf("usage: mill land <target>")
	}

	target := fsArgs[0]
	gates := fsArgs[1:]
	return runLand(target, worktree, gates, confirm, force)
}
