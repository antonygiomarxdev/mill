# Review: #63 — Command support gaps (role get + land worktree lock)

## Verdict

**APPROVED**

## Gate Results

- `go build ./...` — PASS
- `go test ./internal/cli/ -run "TestRole" -count=1` — PASS (TestRoleGetPrintsCurrentRole, TestRoleGetNoFileDefaultsToStaff, TestRoleSetValidRole, TestRoleSetPM, TestRoleSetDelegationOnlyRoleRejected, TestRoleSetInvalidRoleRejected, TestRoleSetNoArgsShowsUsage, TestRoleUnknownSubcommand, TestDetectRoleProduct, TestDetectRoleTechnical, TestDetectRoleUnknownDefaultsToStaff, TestDetectRoleEmptyInput, TestRoleGetEmptyFile, TestRoleSetWriteError, TestRoleSetAlreadySet)
- `go test ./internal/cli/ -run "TestLand" -count=1` — PASS (TestRunLandEmptyGates, TestRunLandGateFailure, TestRunLandConfirmNo, TestAppRunLandNoArgs, TestRunLandSuccessWithGates, TestRunLandHelpFlag, TestRunLandParseError, TestRunLandLockedByOtherWorktree, TestRunLandLockedPrintsResolution, TestRunLandCheckoutGenericFailure)

## Acceptance Criteria Verification

### role get

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | Returns "staff" when no `.mill/role` file | ✅ | `os.IsNotExist(err)` branch returns `"staff"` (role.go:43-45). Verified by TestRoleGetNoFileDefaultsToStaff. |
| 2 | Returns "staff" when `.mill/role` is empty | ✅ | `role == ""` after `strings.TrimSpace` → `role = "staff"` (role.go:51-52). Verified by TestRoleGetEmptyFile. |
| 3 | Returns stored role when file has content | ✅ | `fmt.Fprintln(a.Out, role)` prints trimmed content (role.go:53). Verified by TestRoleGetPrintsCurrentRole. |
| 4 | `go test ./internal/cli/ -run TestRoleGet` passes | ✅ | Both TestRoleGetPrintsCurrentRole and TestRoleGetNoFileDefaultsToStaff and TestRoleGetEmptyFile pass. |

### land

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 5 | Locked `main` prints locking worktree path | ✅ | `detectWorktreeLock` parses `git worktree list` output, finds `[<target>]` branch match, extracts path from first whitespace-separated field, returns error containing locking worktree path (land.go:87-90). Verified by TestRunLandLockedByOtherWorktree. |
| 6 | Locked `main` prints resolution suggestion | ✅ | Error message contains `"resolve: cd <lockingPath> && git checkout <other-branch>"` (land.go:88). Verified by TestRunLandLockedPrintsResolution. |
| 7 | Unlocked `main` works as before | ✅ | When checkout succeeds, `err == nil`, skip `detectWorktreeLock`, return nil. Verified by TestRunLandEmptyGates, TestRunLandSuccessWithGates, TestRunLandConfirmNo. |
| 8 | `go test ./internal/cli/ -run TestLand` passes | ✅ | All 10 land tests pass, including 3 new lock-detection tests. |

### delegate/watch

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 9 | Verified by #60 and #61 | ✅ | Both #60 and #61 have their own review.md APPROVED verdicts. No additional work in #63 scope. |

## Architecture Compliance

- **Fix 1 (role get):** Changes only `roleGet()` — `readActiveRole()` already defaults to "staff", now `roleGet` is consistent. ✅
- **Fix 2 (land):** `detectWorktreeLock` is a pure function — no side effects, no automatic resolution. Does NOT forcibly unlock branches. ✅
- **No new types needed** ✅ — both fixes are within existing function signatures.
- **Backward compatibility:** No existing workflow depends on "none" output from `mill role get`. Scripts checking for "no role set" should check file existence. ✅

## Quality Checks

- No `any`, `unknown`, `Record<string, T>`, or `object` types introduced ✅
- `detectWorktreeLock` falls back to generic "checkout failed" when `git worktree list` fails or parsing fails ✅
- `detectWorktreeLock` fallback when `git worktree list` itself fails (land.go:56-58) ✅
- TestRunLandCheckoutGenericFailure verifies that nonexistent branch returns "checkout failed" without lock message ✅
- Both land lock tests clean up the test worktree with `runGit(t, dir, "worktree", "remove", otherDir)` ✅

## Files Reviewed

- `internal/cli/role.go` (MODIFY) — `roleGet()`: two-line fix (lines 43-45, 51-52)
- `internal/cli/role_test.go` (MODIFY) — TestRoleGetNoFileDefaultsToStaff (renamed from TestRoleGetNoFileShowsNone), TestRoleGetEmptyFile updated expectations
- `internal/cli/land.go` (MODIFY) — `runLand` calls `detectWorktreeLock`, `detectWorktreeLock` (NEW function)
- `internal/cli/land_test.go` (NEW) — 3 tests: TestRunLandLockedByOtherWorktree, TestRunLandLockedPrintsResolution, TestRunLandCheckoutGenericFailure

## Notes

- Fix 1 is exactly the two-line change specified in the tasks: line 43→`"staff"`, line 51→`"staff"`.
- Fix 2 adds `detectWorktreeLock` as a separate function for testability — good design choice.
- The `-C worktree` handling in `detectWorktreeLock` correctly applies only when `worktree != ""`, matching the original `runLand` behavior.
