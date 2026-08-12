package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// runLand runs gate commands in a worktree, optionally confirms with the user,
// then merges the worktree's branch into the target branch from the main
// repository. The merge runs in the main worktree — never inside the agent's
// worktree — so the target branch need not be checked out anywhere.
//
// The main repo path is derived from `git worktree list --porcelain` because
// its first entry is always the main worktree and the output is fully
// qualified. `git rev-parse --git-common-dir` cannot be used here: when the
// worktree is not itself a git repository it silently falls back to the
// process's current working directory, which is not reliable.
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
	// Refuse to land a worktree with uncommitted changes.
	status, err := exec.Command("git", "-C", worktree, "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("cannot check worktree status: %w", err)
	}
	if len(strings.TrimSpace(string(status))) != 0 {
		return fmt.Errorf("refusing to land: worktree %s has uncommitted changes", worktree)
	}
	// Resolve the branch to land; a detached HEAD is an error.
	branchOut, err := exec.Command("git", "-C", worktree, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("cannot resolve worktree branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch == "" || branch == "HEAD" {
		return fmt.Errorf("cannot land: worktree %s is in detached HEAD state", worktree)
	}
	if confirm {
		fmt.Printf("Merge to %s? [y/N]: ", target)
		r := bufio.NewReader(os.Stdin)
		ans, _ := r.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(ans)) != "y" {
			return nil
		}
	}
	// Locate the main repository from the worktree listing and merge there.
	listOut, err := exec.Command("git", "-C", worktree, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("cannot locate main repository: %w", err)
	}
	entries := strings.Split(string(listOut), "\n\n")
	if len(entries) == 0 {
		return fmt.Errorf("cannot locate main repository: no worktree entries")
	}
	mainRepo := ""
	for _, line := range strings.Split(entries[0], "\n") {
		if strings.HasPrefix(line, "worktree ") {
			mainRepo = strings.TrimPrefix(line, "worktree ")
			break
		}
	}
	if mainRepo == "" {
		return fmt.Errorf("cannot locate main repository: malformed worktree list")
	}
	// `git merge <branch>` merges into whatever is checked out in mainRepo,
	// so the target branch must be the current branch there. Never switch a
	// human's branch underneath them — refuse instead.
	headOut, err := exec.Command("git", "-C", mainRepo, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("cannot read current branch of %s: %w", mainRepo, err)
	}
	current := strings.TrimSpace(string(headOut))
	if current != target {
		return fmt.Errorf("cannot land onto %s: repository at %s has %s checked out", target, mainRepo, current)
	}
	msg := fmt.Sprintf("Land %s", branch)
	merge := exec.Command("git", "merge", "--no-ff", branch, "-m", msg)
	merge.Dir = mainRepo
	merge.Stdout = os.Stderr
	merge.Stderr = os.Stderr
	if err := merge.Run(); err != nil {
		return fmt.Errorf("merge of %s into %s failed; resolve conflicts in %s and commit manually", branch, target, mainRepo)
	}
	return nil
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
