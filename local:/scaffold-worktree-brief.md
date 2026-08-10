# Scaffold context files into agent worktree

> **Role:** sr-dev-be | **Model:** free | **Reviewed by:** Staff

## Context

`mill delegate` creates an empty worktree directory and spawns the agent inside it. The agent only receives a text prompt — no `AGENTS.md`, no `.omp/` files, no `roles/`. The mandatory startup sequence defined in those files never executes because the files don't exist in the agent's working directory.

`copyScaffold` in `init.go` already does exactly what we need — it walks the embedded `static/scaffold/` and copies every file to a target directory. We reuse it.

## Acceptance

- [ ] `ls <worktree>/AGENTS.md` exists after delegate
- [ ] `ls <worktree>/.omp/AGENTS.md` exists after delegate
- [ ] `ls <worktree>/.omp/RULES.md` exists after delegate
- [ ] `ls <worktree>/roles/COMMON.md` exists after delegate
- [ ] `cat <worktree>/.mill/role` returns the target role name
- [ ] `go test ./internal/cli/ -run TestDelegate -count=1` passes
- [ ] `go test ./...` passes (full suite)

## Do not touch

- `internal/adapter/` — adapters stay provider-agnostic
- `roles/` directory at repo root
- `go:embed` directive

## Deliverable

- Commits: 1-2
- Files: `internal/cli/delegate.go`, `internal/cli/delegate_test.go` (maybe)

## Steps

- [ ] 1. In `runDelegate`, after `wt := a.worktreePath(issueNum)` and before `installHooks`, call `a.copyScaffold(wt)` to populate context files
- [ ] 2. Write `.mill/role` file with `targetRole` into worktree
- [ ] 3. Fix `installHooks` to create `.git/hooks/` if it doesn't exist
- [ ] 4. Write/update test: verify context files land in worktree after delegate
- [ ] 5. Run `go test ./...` — all green
- [ ] 6. Commit: `fix(delegate): scaffold context files into agent worktree`
