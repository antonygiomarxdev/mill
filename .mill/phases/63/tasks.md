# Tasks: Command support gaps from QA checklist

All tasks modify/update only files listed in the SPEC Components affected table.

## Wave 1 (parallel — independent areas)

- [ ] **Fix `roleGet` default to "staff"** — role: sr-dev-be, deps: none, est: 15m
  1. In `internal/cli/role.go:46`: change `fmt.Fprintln(a.Out, "none")` → `fmt.Fprintln(a.Out, "staff")` (missing file default)
  2. In `internal/cli/role.go:53`: change `role = "none"` → `role = "staff"` (empty file default)
  3. Update `TestRoleGetNoFileShowsNone` in `internal/cli/role_test.go`: rename to `TestRoleGetNoFileDefaultsToStaff`, change expected output from `"none\n"` to `"staff\n"`
  4. Update `TestRoleGetEmptyFile` in `internal/cli/role_test.go`: change expected output from `"none\n"` to `"staff\n"`
  5. Verify `go test ./internal/cli/ -run TestRoleGet` passes

- [ ] **Add `mill land` worktree lock detection** — role: sr-dev-be, deps: none, est: 30m
  1. Modify `internal/cli/land.go:runLand`: after `git checkout` fails (line 35), run `git worktree list` (with `-C worktree` if worktree is set, else without). Parse the output to find a worktree whose branch matches `refs/heads/<target>` or `[<target>]`. If found, print:
     ```
     land: cannot checkout '<target>' — locked by another worktree
       locking worktree: <path>
       resolve: cd <path> && git checkout <other-branch>
     ```
     Return error with that message (replacing the generic "checkout failed"). If no locking worktree found, keep the original generic error.
  2. Parse `git worktree list` output by splitting each line on whitespace. Format: `<path> <HEAD-hash> [<branch>]` or `<path> <HEAD-hash> (detached HEAD)`. Match the branch portion against the target.
  3. If parsing fails or `git worktree list` itself fails, fall back to the original generic "checkout failed" error.
  4. Create `internal/cli/land_test.go` with:
     - `TestRunLandSuccessPath`: sets up a temp git repo, checks out a non-main branch, calls `runLand("main", dir, []string{}, false)`, asserts no error
     - `TestRunLandLockedByOtherWorktree`: sets up a repo with `main` checked out in a second worktree at `/tmp/land-test-other`, then calls `runLand("main", primaryDir, []string{}, false)` from the primary worktree, asserts error contains "locked by another worktree" and the locking worktree path
     - `TestRunLandLockedPrintsResolution`: same setup, asserts error contains "resolve: cd " and "git checkout"
     - `TestRunLandGateFailure`: calls `runLand("main", dir, []string{"exit 1"}, false)`, asserts error contains "gate .* failed"
     - `TestRunLandCheckoutGenericFailure`: mocks a scenario where checkout fails but no worktree lock is detected (e.g., checkout a nonexistent branch), asserts error contains "checkout failed" (original message)
  5. Verify `go test ./internal/cli/ -run TestLand` passes
