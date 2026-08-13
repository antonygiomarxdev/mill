package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// installRoleEnforceHook ensures the worktree has an executable
// .mill/checks/role-enforce so the pre-commit gauntlet enforces roles.
//
// Git worktree add only materialises tracked files, and .mill/checks/ is
// versioned now, so the normal path is that the file is already present and
// executable — in which case this is a no-op. Only when it is missing is the
// source copied from the main repository. The main repo path is derived from
// `git worktree list --porcelain` because its first entry is always the main
// worktree and the output is fully qualified. `git rev-parse --git-common-dir`
// cannot be used here: when the worktree is not itself a git repository it
// silently falls back to the process's current working directory, which is not
// reliable.
func installRoleEnforceHook(worktree string) error {
	roleEnforceDst := filepath.Join(worktree, ".mill", "checks", "role-enforce")

	if info, err := os.Stat(roleEnforceDst); err == nil && info.Mode().Perm()&0o111 != 0 {
		return nil
	}

	// Locate the main repository from the worktree listing.
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

	roleEnforceSrc := filepath.Join(mainRepo, ".mill", "checks", "role-enforce")
	enforceData, err := os.ReadFile(roleEnforceSrc)
	if err != nil {
		return fmt.Errorf("role-enforce not found: tried worktree %s and main repo %s", roleEnforceDst, roleEnforceSrc)
	}

	if err := os.MkdirAll(filepath.Dir(roleEnforceDst), 0755); err != nil {
		return err
	}
	return os.WriteFile(roleEnforceDst, enforceData, 0755)
}
