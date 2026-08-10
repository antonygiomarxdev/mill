package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runLand runs gate commands in a worktree, optionally confirms with the user,
// and checks out the target branch.
func runLand(target string, worktree string, gates []string, confirm bool) error {
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
		return detectWorktreeLock(worktree, target, err)
	}
	return nil
}

// detectWorktreeLock checks if a checkout failure is due to another worktree
// holding the target branch. If so, it returns a detailed error with the
// locking worktree path and resolution suggestion. Otherwise, it returns
// a generic "checkout failed" error.
func detectWorktreeLock(worktree, target string, checkoutErr error) error {
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

		return fmt.Errorf(
			"land: cannot checkout '%s' — locked by another worktree\n  locking worktree: %s\n  resolve: cd %s && git checkout <other-branch>",
			target, lockingPath, lockingPath,
		)
	}

	return fmt.Errorf("checkout failed")
}

// runLand handles the "land" command.
func (a *App) runLand(args []string) error {
	fs := flag.NewFlagSet("land", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var worktree string
	var confirm bool
	fs.StringVar(&worktree, "worktree", "", "worktree directory")
	fs.BoolVar(&confirm, "confirm", false, "prompt before merging")

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
	return runLand(target, worktree, gates, confirm)
}
