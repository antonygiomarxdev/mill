# Spec: Role-based capability enforcement

## Architecture

**Problem:** The delegation chain is advisory-only. No mechanical enforcement prevents role boundary violations. Staff can write code. PM can touch architecture. The `validateDelegation` function exists in `internal/cli/delegate.go` but only checks delegation chains via `delegates_to` — it does not restrict what a role can *do* within a worktree.

**Solution:** A two-layer enforcement architecture:

### Layer 1: Role pool validation (existing, extended)
The `validActiveRoles` map in `internal/cli/role.go` already restricts active roles to `{staff, pm}`. This is extended: `roleSet` rejects delegation-only roles (sr-dev, tech-lead, architect, ux, ui, reviewer, qa-docs). The `runRole` handler already has this logic; it is tightened with an explicit deny list.

### Layer 2: Pre-commit capability enforcement (NEW)
A pre-commit hook script `checks/role-enforce` is installed in every Mill worktree. The hook reads `.mill/role` from the worktree root, then checks the changed files against the role's authorized file types:

| Role | Allowed file types |
|---|---|
| PM | `.md` only (specs, docs) |
| Architect | `.md`, `.yml`, `.yaml` (specs, ADRs, config) |
| Tech Lead | `.md`, `.go` (tasks, code review) |
| Sr. Dev (all) | `.go`, `.md`, `.yml`, `.yaml`, `.json` |
| Reviewer | `.md` only (reviews, reports) |
| QA/Docs | `.md`, `.yml` (docs, changelog) |
| UX/UI | `.md`, `.pen` (design docs) |

Staff and PM are NOT subject to pre-commit enforcement (they are orchestrators, not producers). Their enforcement is at the delegation level — they may not produce code, but they produce briefs and decisions.

### Architecture decision: hook-based, not runtime

The hook is a bash script, not a Go binary. Rationale:
- Git hooks are universal — no new binary dependency
- The hook runs *before* commit, not at session time
- Subagents spawn with `git commit` and the hook catches them at commit time
- The existing `installHooks` function in `delegate.go` already copies hooks — this extends it

### Data flow
```
agent spawns → git adds files → pre-commit fires → 
  reads .mill/role → reads .mill/roles/<role>/ROLE.md → 
  checks file extensions → rejects or allows commit
```

The ROLE.md frontmatter is parsed by the hook script using simple YAML parsing (the `skills:` and `delegates_to:` fields). The hook does NOT read Go code — it reads the frontmatter only.

### Error messages
When enforcement blocks a commit:
```
pre-commit: BLOCKED — role 'PM' cannot commit .go files.
  PM can produce: .md
  To proceed: switch roles with 'mill role set' or escalate to Staff.
```

## Components affected

| File | Change |
|---|---|
| `internal/cli/role.go` | MODIFY: Tighten `roleSet` to deny delegation-only roles as active |
| `internal/cli/role_test.go` | MODIFY: Add test for rejecting delegation-only roles |
| `checks/role-enforce` | MODIFY: Add file-type enforcement based on `.mill/role` |
| `internal/cli/delegate.go` | MODIFY: `installHooks` copies updated `role-enforce` |
| `.mill/roles/*/ROLE.md` | MODIFY: Add `allowed_files:` frontmatter field to each role |

### Files NOT affected
- `internal/adapter/` — no changes
- `internal/state/` — no schema changes
- `internal/domain/` — no new types

## Risks

### Risk 1: Pre-commit hook blocks legitimate work due to misclassification
**Severity:** Medium. **Mitigation:** The hook allows `--no-verify` bypass for emergency situations. The hook logs every rejection with the role, files, and timestamp to `.mill/enforcement.log`. Staff can review the log and adjust role permissions. The deny list is permissive: roles can produce *all* allowed file types, not just one.

### Risk 2: hook must be installed in worktree BEFORE agent starts
**Severity:** Low. **Mitigation:** `installHooks` runs during `runDelegate` scaffolding, before the goroutine dispatches. The hook is in place before `git commit` is ever called. If the hook fails to install, `installHooks` logs a warning but does not block delegation — enforcement degrades gracefully.

### Risk 3: Delegation-only roles used as active via `mill role set`
**Severity:** Low (already partially enforced). **Mitigation:** `roleSet` already checks `validActiveRoles`. This spec tightens it with an explicit deny list for delegation-only roles. The deny list is derived from `delegates_to` entries across all ROLE.md files — any role that appears ONLY as a delegation target is deny-listed for active use.

### Risk 4: hook script is bash, not Go — harder to test
**Severity:** Low. **Mitigation:** The hook has a test mode: `checks/role-enforce --test <role> <file>` simulates the check without a git environment. Go tests call the hook as a subprocess with test arguments. This matches the existing pattern for `checks/gate-*` scripts.

## ADR

**NEW ADR: ADR 0005 — Hook-based role enforcement.** Role capability enforcement uses git pre-commit hooks rather than runtime interception. Rationale:
- Git hooks are the natural enforcement point for "what gets committed"
- No runtime overhead or session-state dependency
- Works with any AI provider (not coupled to harness)
- Degrades gracefully: hook failure → warning, not blocked delegation
- Separates concerns: enforcement at commit time, delegation validation at dispatch time

## Acceptance criteria

1. `mill role set sr-dev` returns an error: "sr-dev is a delegation-only role"
2. Pre-commit hook rejects `.go` file commits when `.mill/role = pm`
3. Pre-commit hook allows `.md` file commits when `.mill/role = pm`
4. Pre-commit hook reads allowed file types from ROLE.md frontmatter
5. `--no-verify` bypasses hook for emergency use
6. `checks/role-enforce --test pm foo.go` exits non-zero
7. `checks/role-enforce --test pm foo.md` exits zero
8. `go test ./internal/cli/` passes (role tests updated)
