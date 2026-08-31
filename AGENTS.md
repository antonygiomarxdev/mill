# Mill — Agent Delegation Harness

A skill plus a policy directory (`docs/adr/0006-mill-is-a-skill-not-a-binary.md`).
The coordinator is the **Product Engineer** (`roles/product-engineer/ROLE.md`).
Delegation goes through the `delegate` skill (`.claude/skills/delegate/SKILL.md`).

## Topology

One coordinator dispatches workers. Workers execute and report; no worker
dispatches another worker. Role files define what each role produces and what
it may write:

- `roles/COMMON.md` — shared rules for all roles
- `roles/<role>/ROLE.md` — one per role under `roles/`

## Verification

After every dispatch the coordinator runs `.mill/checks/mill-verify` from this
repository against the worker's worktree. Full procedure in the Product
Engineer role file.

## Project layout

```
.mill/           policy: roles, checks, skills, docs, phases
scaffold/        the frozen installer template
docs/            ADRs, research, FINDINGS
local/           operator-local state (gitignored)
```