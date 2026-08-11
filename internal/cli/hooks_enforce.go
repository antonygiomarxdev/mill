package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// installRoleEnforceHook copies checks/role-enforce into the worktree's
// .mill/checks/role-enforce.sh so the pre-commit gauntlet picks it up
// (the gauntlet runs every *.sh in .mill/checks/). If checks/role-enforce
// is missing, a warning is logged but the error is not returned (enforcement
// degrades gracefully).
func installRoleEnforceHook(worktree string) error {
	roleEnforceSrc := "checks/role-enforce"
	checksDir := filepath.Join(worktree, ".mill", "checks")
	roleEnforceDst := filepath.Join(checksDir, "role-enforce.sh")

	if err := os.MkdirAll(checksDir, 0755); err != nil {
		return err
	}

	enforceData, err := os.ReadFile(roleEnforceSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "installHooks: checks/role-enforce not found, pre-commit enforcement disabled\n")
		return nil
	}

	return os.WriteFile(roleEnforceDst, enforceData, 0755)
}
