# Spec: Command support gaps from QA checklist

## Architecture

**Problem:** QA testing on Linux/RUMAI project identified three bugs in mill CLI commands beyond the major gaps covered by #60 (delegate) and #61 (watch):

1. **`mill role get` returns 'none'** instead of defaulting to 'staff' when no role file exists
2. **`mill land` fails when `main` is locked** by another worktree — the checkout fails with no recovery path
3. **`mill delegate` + `mill watch` broken** — tracked separately as #60 and #61

This spec covers the two independent fixes that are NOT covered by other issues.

### Fix 1: `mill role get` default

**Current behavior** (`internal/cli/role.go:40-57`):
```go
func (a *App) roleGet() error {
    // ...
    if err != nil {
        fmt.Fprintln(a.Out, "none")   // ← BUG: returns 'none'
        return nil
    }
    // ...
    if role == "" {
        role = "none"                  // ← BUG: empty file returns 'none'
    }
    // ...
}
```

**Expected behavior:** When no `.mill/role` file exists, `mill role get` should return "staff" — the default active role per `readActiveRole()` in delegate.go (which already defaults to "staff"). The role file is created only when `mill role set <role>` is called. Before the first `set`, the implied role is "staff".

**Fix:** In `roleGet()`, when `os.ReadFile` returns `os.IsNotExist`, return "staff" instead of "none". When the file exists but is empty, also return "staff". This is a two-line change.

**Design decision:** The default is "staff", not "none", because:
- `readActiveRole()` (used by `runDelegate`) already defaults to "staff"
- The architecture doc states Staff is one of two active roles
- "none" is not a valid role — it's a display artifact, not a semantic default
- Consistency across the codebase: `detectRole()` returns "staff" for unknown input

### Fix 2: `mill land` worktree lock handling

**Current behavior** (`internal/cli/land.go`):
```go
func runLand(target string, worktree string, gates []string, confirm bool) error {
    // ... run gates ...
    cmd := exec.Command("git", "-C", worktree, "checkout", target)
    // ... if err, return "checkout failed" — no lock info
}
```

**Expected behavior:** When `git checkout` fails, `mill land` should inspect the failure reason. If the failure is due to a locked branch (another worktree has `main` checked out), `mill land` should:
1. Print which worktree holds the lock (from `git worktree list`)
2. Suggest resolution: `cd <locking-worktree> && git checkout <other-branch>`
3. Exit with a clear error message, not a generic "checkout failed"

**Fix:** After `git checkout` fails, run `git worktree list` to enumerate worktrees. Parse output to find which worktree holds the target branch. Print a specific error:
```
land: cannot checkout 'main' — locked by another worktree
  locking worktree: /path/to/.mill/worktrees/issue-41
  resolve: cd /path/to/.mill/worktrees/issue-41 && git checkout some-other-branch
```
The function does NOT automatically resolve the lock — that's a policy decision left to the user.

**Non-goal:** `mill land` does NOT forcibly unlock branches. That would be dangerous — the locking worktree may have uncommitted changes.

### Fix 3: Delegate + watch

Tracked separately:
- `mill delegate` → #60 (invoke AI with review loop)
- `mill watch` → #61 (block until tasks settle)

No additional work in this spec. The fixes in #60 and #61 resolve these gaps.

## Components affected

| File | Change |
|---|---|
| `internal/cli/role.go` | MODIFY: `roleGet()` returns "staff" on missing/empty role file |
| `internal/cli/role_test.go` | MODIFY: Add test cases for missing role file, empty role file |
| `internal/cli/land.go` | MODIFY: `runLand` inspects worktree lock on checkout failure |
| `internal/cli/land_test.go` | NEW: Tests for lock detection, error message format |

### Files NOT affected
- `internal/cli/delegate.go` — no changes (covered by #60)
- `internal/cli/app.go` — no routing changes
- `internal/cli/watch.go` — no changes (covered by #61)
- `internal/domain/` — no new types needed
- `internal/state/` — no schema changes

## Risks

### Risk 1: `git worktree list` output format change across Git versions
**Severity:** Low. **Mitigation:** The output format of `git worktree list` (detached HEAD, branch refs, path) has been stable since Git 2.5 (2015). The parsing uses whitespace splitting and is robust to extra columns. If parsing fails, the error falls back to the original generic message plus the raw `git worktree list` output for manual inspection.

### Risk 2: `role get` returning 'staff' when user expects 'none'
**Severity:** Low (backward compatibility). **Mitigation:** No existing workflow depends on `mill role get` returning 'none'. Scripts that checked for 'none' to detect "no role set" should check for file existence instead (`test -f .mill/role`). The behavior change is documented in the issue comment.

### Risk 3: `mill land` with no worktree flag
**Severity:** Low. **Mitigation:** When `--worktree` is not specified, `runLand` uses an empty string for the worktree path. The `git checkout` runs in the current directory — lock detection still works (runs `git worktree list` without `-C`). This is existing behavior, unchanged by this fix.

## ADR

No new ADR. These are isolated fixes within single commands — no cross-cutting decisions.

## Acceptance criteria

### role get
1. `mill role get` returns "staff" when no `.mill/role` file exists
2. `mill role get` returns "staff" when `.mill/role` exists but is empty
3. `mill role get` returns the stored role when `.mill/role` has content
4. `go test ./internal/cli/ -run TestRoleGet` passes

### land
5. `mill land main` with `main` locked prints the locking worktree path
6. `mill land main` with `main` locked prints a resolution suggestion
7. `mill land main` with `main` unlocked works as before (gates + checkout)
8. `go test ./internal/cli/ -run TestLand` passes (existing + new tests)

### delegate/watch
9. Verified by #60 and #61 acceptance criteria — no additional work
