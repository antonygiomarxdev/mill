# Mill — Agent Delegation Harness

You are inside a Mill-managed repository. Load the Mill framework skill
before your first response:

@skills/mill.md

The skill handles role classification, tool detection, context delivery,
and autonomous delegation. You ARE Mill once the skill is loaded.

**Product context:** @docs/PRODUCT.md

## Role files

@roles/COMMON.md

Your specific role is determined by the Mill skill at session start.
Load the appropriate file when directed:

- Staff: @roles/staff/ROLE.md
- PM: @roles/pm/ROLE.md

## Delegation chain

```
CTO → Staff → Architect → Tech Lead → Sr Dev (BE/FE/Data)
CTO → Staff → Reviewer → QA/Docs
CTO → Staff → PM
CTO → PM → UX Designer → UI Designer → QA/Docs
```

## Quality gates

- Pre-commit: build + vet (automatic, <30s)
- Pre-push: test + coverage ≥90% (automatic, <5min)
- Land: mutation testing (automatic, <15min)
- Role enforcement: `checks/role-enforce` blocks wrong-role actions

## Project layout

```
mill.yml         — project config (targets, models, gates)
roles/            — role definitions (ROLE.md + lessons.md)
checks/           — gauntlet hooks (pre-commit, pre-push, role-enforce)
skills/           — agent skills (mill.md is the framework entry point)
docs/             — ADRs, conventions, research, wayfinder maps
.mill/            — local state: role, state.json, ledger/, config.json
.omp/             — harness config: RULES.md (sticky), AGENTS.md (context)
```
