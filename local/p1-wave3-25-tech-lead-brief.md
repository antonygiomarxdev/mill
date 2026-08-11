# Issue #25 — Role Contract Enforcement

> **Role:** tech-lead | **Model:** pro | **Reviewed by:** architect

## Context

#74 (ROLE.md frontmatter) is fixed. All 11 files parse correctly. `AllowedFiles []string` is populated. Now we wire the enforcement.

The `checks/role-enforce` script exists and works in test mode. The Go function `installRoleEnforceHook` exists but is NOT called. The current script writes to `.git/hooks/pre-commit` but `installHooks` redirects `core.hooksPath` to `.mill/hooks/` — the gauntlet bypasses `.git/hooks/`.

Design posted: https://github.com/antonygiomarxdev/mill/issues/25#issuecomment-5259260455

## Acceptance

- [ ] `grep -c "installRoleEnforceHook" internal/cli/delegate.go` returns `1` (wired)
- [ ] `go test ./internal/role/... -run TestRoleEnforce -count=1` passes
- [ ] `go test ./internal/cli/... -run TestInstallRoleEnforce -count=1` passes (new tests)
- [ ] `checks/role-enforce --test sr-dev-be main.go` exits 0 (.go allowed)
- [ ] `checks/role-enforce --test sr-dev-be ROLE.md` exits 1 (forbidden pattern)
- [ ] `checks/role-enforce --test staff main.go` exits 1 (staff blocked from .go)
- [ ] `checks/role-enforce --test pm config.yml` exits 1 (pm blocked from .yml)
- [ ] `go test ./internal/issue/... -run TestAddLabel -count=1` passes
- [ ] Full suite: `go test ./internal/cli/... ./internal/role/... ./internal/issue/... -count=1` all green

## Do not touch

- `checks/gate-*` — out of scope
- `internal/cli/delegate.go` pre-commit gauntlet script content (lines 722-741) — only wire the call
- Any ROLE.md files not listed below
- `.mill/roles/staff/lessons.md` and other non-ROLE.md files in role dirs
- `docs/` directory

## Deliverable

- Commits: ≥4 (one per logical change group)
- Files:
  - `internal/cli/hooks_enforce.go` — rewrite to target `.mill/checks/`
  - `internal/cli/delegate.go` — wire call
  - `checks/role-enforce` — add forbidden_patterns, tighten semantics
  - `internal/role/role.go` — add ForbiddenPatterns
  - `.mill/roles/staff/ROLE.md` — add allowed_files
  - `.mill/roles/sr-dev-be/ROLE.md` — add forbidden_patterns
  - `.mill/roles/sr-dev-fe/ROLE.md` — add forbidden_patterns
  - `.mill/roles/sr-dev-data/ROLE.md` — add forbidden_patterns
  - `internal/issue/reader.go` — add AddLabel
  - `internal/cli/delegate.go` — post-dispatch enforcement log check
  - Test files as needed

## Steps

### Task 1: Fix installRoleEnforceHook + wire it
- [ ] 1. Rewrite `installRoleEnforceHook` in `hooks_enforce.go`: copy `checks/role-enforce` → `<worktree>/.mill/checks/role-enforce.sh` with 0755. Drop `.git/hooks/pre-commit` path entirely.
- [ ] 2. Wire at `internal/cli/delegate.go:179`: add `if err := installRoleEnforceHook(wt); err != nil { ... }` after the `installHooks(wt)` error check.
- [ ] 3. Write `TestInstallRoleEnforceHook` — verify script lands in `.mill/checks/role-enforce.sh` with 0755.
- [ ] 4. Gate: `go test ./internal/cli/... -run TestInstallRoleEnforce -count=1`
- [ ] 5. Commit: `feat(cli): wire installRoleEnforceHook into delegate workflow`

### Task 2: Update checks/role-enforce — forbidden_patterns + tightened semantics
- [ ] 1. Add `forbidden_patterns` parsing in `checks/role-enforce` — parallel to `allowed_files` parsing, same awk pattern.
- [ ] 2. Check staged file basenames against forbidden_patterns in both pre-commit and test mode. Match on exact basename (e.g., `ROLE.md`). Forbidden check runs BEFORE allowed_files check.
- [ ] 3. Tighten empty `allowed_files`: if allowed_files list is empty, treat as "nothing allowed" (exit 1). Previously empty meant "no restrictions".
- [ ] 4. Verify with test mode:
  - `checks/role-enforce --test sr-dev-be main.go` → exit 0
  - `checks/role-enforce --test sr-dev-be ROLE.md` → exit 1
  - `checks/role-enforce --test staff main.go` → exit 1 (staff now has no .go in allowed_files)
  - `checks/role-enforce --test pm config.yml` → exit 1
  - `checks/role-enforce --test staff README.md` → exit 0 (staff allowed .md)
- [ ] 5. Gate: `go test ./internal/role/... -run TestRoleEnforce -count=1`
- [ ] 6. Commit: `feat(checks): add forbidden_patterns and tighten empty allowed_files`

### Task 3: Add ForbiddenPatterns to role parser
- [ ] 1. Add `ForbiddenPatterns []string` to `Frontmatter` struct in `internal/role/role.go`.
- [ ] 2. Add `case "forbidden_patterns":` in parser, identical pattern as `allowed_files`.
- [ ] 3. Extend existing test to verify forbidden_patterns parsing for sr-dev roles.
- [ ] 4. Gate: `go test ./internal/role/... -count=1`
- [ ] 5. Commit: `feat(role): add ForbiddenPatterns to frontmatter parser`

### Task 4: Update ROLE.md files
- [ ] 1. `.mill/roles/staff/ROLE.md`: Change empty `allowed_files:` to `allowed_files:\n  - .md`
- [ ] 2. `.mill/roles/sr-dev-be/ROLE.md`: Add after `allowed_files:` block:
  ```
  forbidden_patterns:
    - ROLE.md
  ```
- [ ] 3. Same for `sr-dev-fe/ROLE.md` and `sr-dev-data/ROLE.md`.
- [ ] 4. Gate: `go test ./internal/role/... -run TestParseAllRoleFiles -count=1` — all 11 still parse, ForbiddenPatterns populated for sr-dev-*.
- [ ] 5. Commit: `feat(roles): add allowed_files for staff, forbidden_patterns for sr-dev`

### Task 5: Add AddLabel + auto-label needs:rework
- [ ] 1. Add `func AddLabel(issueNum int, label string) error` to `internal/issue/reader.go`. Wraps `gh issue edit <num> --add-label <label>`.
- [ ] 2. Write `TestAddLabel` (skip if `gh` unavailable, like existing reader tests).
- [ ] 3. In `internal/cli/delegate.go`: after dispatch completes (in `runDispatchLoop` or after it returns), check if `.mill/enforcement.log` exists in worktree and has any BLOCKED entries. If yes, call `AddLabel(issueNum, "needs:rework")`.
- [ ] 4. Gate: `go test ./internal/issue/... -run TestAddLabel -count=1`
- [ ] 5. Gate: `go test ./internal/cli/... -count=1`
- [ ] 6. Commit: `feat(cli): auto-label needs:rework on role-enforce violations`

## Decomposition Notes

- Task 1 must come first (unblocks the wiring)
- Tasks 2 and 3 are independent (can parallelize)
- Task 4 depends on Task 3 (parser must support forbidden_patterns before ROLE.md files use it)
- Task 5 depends on Task 1 (needs enforcement log location)
- Parallel dispatch: Tasks 2 + 3 after Task 1; then Task 4 + 5 in parallel
