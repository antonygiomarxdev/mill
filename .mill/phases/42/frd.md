# FRD: Role-based capability enforcement

## User need

Mill's delegation chain depends on every role staying within its boundaries. Without mechanical enforcement, agents and humans act outside their role — Staff writes code, PM touches architecture, a Reviewer spawns a Sr.Dev. This breaks the delegation trust model and wastes expensive-model tokens on activity the role should not perform.

The CTO must be able to trust that when a role is active, its capabilities are bounded. The enforcement must be automatic, not advisory.

## Functional requirements

1. **Role detection at session start.** When a CTO session begins, Mill must classify the first interaction as product (→ PM active) or technical (→ Staff active). Classification is based on the first user message content — questions about features, priorities, backlog, or UX classify as product; questions about code, architecture, infrastructure, or bugs classify as technical.

2. **Active role pool.** Only Staff and PM are selectable as the active role for direct CTO interaction. Sr. Dev, Tech Lead, Architect, UX Designer, UI Designer, Reviewer, and QA/Docs are delegation-only — they may never be the active role in a CTO session.

3. **Capability mapping from ROLE.md frontmatter.** Every `.mill/roles/<role>/ROLE.md` defines a `skills:` and `delegates_to:` list in its frontmatter. The enforcement layer reads this at startup and builds a capability matrix: which skills each role may invoke, and which roles it may delegate to.

4. **Pre-action enforcement.** Before any tool invocation or skill call, the enforcement hook checks: (a) the active role is in the allowed pool, (b) the requested skill is in the role's `skills:` list, and (c) if delegating, the target role is in the source role's `delegates_to:` list. A blocked action returns a clear error — "Role PM cannot invoke skill: code-review. Available skills: wayfinder, grilling, domain-modeling, brainstorming."

5. **Role switch is explicit and visible.** Switching the active role requires an explicit command or CTO directive. The system must emit a visible message: "Switched to PM" or "Switched to Staff". Implicit or silent role changes are prohibited.

6. **`.mill/role` as single source of truth.** The active role is stored in `.mill/role` (a plain file containing the role name). All enforcement reads from this file. No environment variable, session variable, or runtime override may bypass it.

7. **Bootstrap: pre-commit hook.** The enforcement layer installs a pre-commit hook in every Mill-managed worktree. The hook rejects commits where the committer's active role is not authorized to produce the changed file types. Staff is blocked from writing `.go` files. PM is blocked from writing `.go` files and YAML/JSON config files.

8. **Error messages are actionable.** When a capability check fails, the error message includes: what was attempted, which role is active, which capability is missing, and what the role can do instead. No generic "permission denied" messages.

## Out of scope

- Full runner integration. Bootstrap uses git hooks; the full runner will integrate enforcement into its own pipeline in a follow-up.
- Role-specific tool filtering (e.g., removing tools from the agent's available toolset). Bootstrap uses pre-commit hooks only.
- Dynamic capability changes at runtime. Role capabilities are loaded once at startup from ROLE.md frontmatter.
- User authentication or identity verification. The active role is declared, not authenticated.

## Priority

**P0** — blocks process reliability. Without mechanical enforcement, every session violates role boundaries, the delegation chain is advisory-only, and the CTO cannot trust that agents stay within their assigned scope.
