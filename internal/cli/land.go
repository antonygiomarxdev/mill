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
		return fmt.Errorf("checkout failed")
	}
	return nil
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
