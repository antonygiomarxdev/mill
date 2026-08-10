package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// installRoleEnforceHook copies checks/role-enforce to the worktree's
// .git/hooks/pre-commit if it exists. If checks/role-enforce is missing,
// a warning is logged to stderr but the error is not returned (enforcement
// degrades gracefully).
//
// TODO(#42): Call this after installHooks() in runDelegate():
//
//	if err := installHooks(wt); err != nil { ... }
//	if err := installRoleEnforceHook(wt); err != nil { ... }
func installRoleEnforceHook(worktree string) error {
	roleEnforceSrc := "checks/role-enforce"
	hookDir := filepath.Join(worktree, ".git", "hooks")
	preCommitDst := filepath.Join(hookDir, "pre-commit")

	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return err
	}

	enforceData, err := os.ReadFile(roleEnforceSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "installHooks: checks/role-enforce not found, pre-commit enforcement disabled\n")
		return nil
	}

	return os.WriteFile(preCommitDst, enforceData, 0755)
}
