# Tasks: Role-based capability enforcement

> **Spec:** .mill/phases/42/spec.md | **ADR:** ADR 0005 — Hook-based role enforcement

Enforcement layers: (1) `roleSet` deny-list blocks delegation-only roles from active use, (2) pre-commit hook `checks/role-enforce` enforces file-type boundaries per role.

## Wave 1 (parallel — independent files)

- [ ] **Tighten `roleSet` to deny delegation-only roles as active** — role: sr-dev-be, deps: none, est: 20m
  FILE: `internal/cli/role.go` (MODIFY), `internal/cli/role_test.go` (MODIFY)
  SPEC: Architecture (Layer 1), Risks (Risk 3), AC 1
  1. Add `delegationOnlyRoles` map: `sr-dev`, `sr-dev-be`, `sr-dev-fe`, `sr-dev-data`, `tech-lead`, `architect`, `ux-designer`, `ui-designer`, `reviewer`, `qa-docs` — roles that appear as delegation targets but are never active
  2. Update `roleSet`: check `delegationOnlyRoles` before `knownRoles`. Error: `"<role> is a delegation-only role, not an active role. Valid: staff, pm"`
  3. Convert existing `TestRoleSetDelegationOnlyRoleRejected` to table-driven: test all 10 delegation-only roles. Each asserts error message contains the role name
  4. Verify: `go test -run "TestRoleSet" ./internal/cli/` — all cases pass, no false rejects for `staff`/`pm`

- [ ] **Add `allowed_files` frontmatter to each ROLE.md** — role: qa-docs, deps: none, est: 15m
  FILES: `.mill/roles/*/ROLE.md` (MODIFY, 11 files)
  SPEC: Architecture (Layer 2 table), Components affected
  1. Add `allowed_files:` YAML list field to each role's frontmatter, placed after `delegates_to:`:
     - `staff`: `[]` (orchestrator — no file enforcement)
     - `pm`: `[.md]` (orchestrator — hook skips enforcement)
     - `architect`: `[.md, .yml, .yaml]`
     - `tech-lead`: `[.md, .go]`
     - `sr-dev-be`, `sr-dev-fe`, `sr-dev-data`: `[.go, .md, .yml, .yaml, .json]`
     - `reviewer`: `[.md]`
     - `qa-docs`: `[.md, .yml]`
     - `ux-designer`: `[.md, .pen]`
     - `ui-designer`: `[.md, .pen]`
  2. Verify: `grep -l 'allowed_files:' .mill/roles/*/ROLE.md | wc -l` returns `11`

## Wave 2 (depends on Wave 1)

- [ ] **Create `checks/role-enforce` with file-type enforcement** — role: sr-dev-be, deps: Task 2 (ROLE.md frontmatter), est: 35m
  FILE: `checks/role-enforce` (NEW)
  SPEC: Architecture (Layer 2, data flow, error messages), Risks (Risk 1, Risk 4), AC 2,3,4,5,6,7
  1. Bash script: `#!/bin/bash`, `set -euo pipefail`
  2. **Pre-commit mode** (no args): runs as git pre-commit hook
     - Reads `.mill/role` for active role; if missing or empty → exit 0 (no enforcement)
     - Skips enforcement for `staff` role → exit 0
     - Parses `.mill/roles/<role>/ROLE.md` frontmatter: extract `allowed_files:` list (awk, same pattern as `gate-route`)
     - Gets staged files via `git diff --cached --name-only --diff-filter=ACMR`
     - Checks each file extension (lowercased) against allowed list
     - On violation: prints error message matching spec format and exits 1:
       ```
       pre-commit: BLOCKED — role '<role>' cannot commit .<ext> files.
         <role> can produce: <ext-list>
         To proceed: switch roles with 'mill role set' or escalate to Staff.
       ```
     - Rejection logged to `.mill/enforcement.log`: `[<ISO-8601>] BLOCKED <role> <file-list>`
  3. **Test mode**: `--test <role> <file>`
     - Simulates enforcement for given role+file without git
     - Exits 0 if allowed, 1 if blocked (with same error message format)
     - No enforcement log entry in test mode
  4. **`--no-verify` bypass**: git sets `GIT_REFLOG_ACTION` or check `--no-verify` flag; honor standard git skip
  5. Verify AC 6: `bash checks/role-enforce --test pm foo.go` exits non-zero
  6. Verify AC 7: `bash checks/role-enforce --test pm foo.md` exits zero
  7. Verify AC 4: hook reads from ROLE.md — test with sr-dev-be role allows `.go`, blocks `.pen`

- [ ] **Update `installHooks` to copy role-enforce as pre-commit** — role: sr-dev-be, deps: Task 3, est: 15m
  FILE: `internal/cli/delegate.go` (MODIFY)
  SPEC: Architecture (hook-based, not runtime), Components affected, Risks (Risk 2)
  1. After existing loop that copies checks to `.git/hooks/`, add: if `checks/role-enforce` exists, copy it to `<worktree>/.git/hooks/pre-commit`
  2. If `checks/role-enforce` is missing, log warning to stderr (`"installHooks: checks/role-enforce not found, pre-commit enforcement disabled"`) — do NOT block delegation (Risk 2: degrade gracefully)
  3. Keep existing behavior: all files from `.mill/checks/` still copied to `.git/hooks/` (stripping `.sh` suffix)
  4. Verify: manual smoke — `installHooks` on a temp worktree creates `.git/hooks/pre-commit` when `checks/role-enforce` exists

## Wave 3 (depends on Wave 2)

- [ ] **Integration test all enforcement paths** — role: sr-dev-be, deps: Tasks 1,2,3,4, est: 25m
  FILE: `internal/cli/role_test.go` (MODIFY)
  SPEC: Acceptance criteria 1,2,3,5,6,7,8
  1. Extend `TestRoleSetDelegationOnlyRoleRejected` to table-driven covering all 10 delegation-only roles (completed in Task 1 if not already)
  2. Add `TestRoleEnforceHookTestMode`: runs `bash checks/role-enforce --test <role> <file>` as subprocess for key cases:
     - `pm foo.go` → non-zero exit (AC 6)
     - `pm foo.md` → zero exit (AC 7)
     - `sr-dev-be main.go` → zero exit
     - `sr-dev-be layout.pen` → non-zero exit
     - `tech-lead main.go` → zero exit
     - `tech-lead config.yml` → non-zero exit
  3. Add `TestRoleEnforceHookStaffBypass`: `staff` role skips enforcement (any file allowed)
  4. Add `TestRoleEnforceHookMissingRole`: no `.mill/role` → exits 0
  5. Verify AC 8: `go test ./internal/cli/ -count=1` full suite passes
  6. Verify: `grep -c "role:" .mill/phases/42/tasks.md` reports ≥ 5 (every task has role assignment)
