# Mill — Agent Delegation Harness

If the user says "using mill" or wants to delegate, load:

@skills/using-mill.md

Then follow its instructions to bootstrap or activate Mill.

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
- Phase gates: `checks/gate-{frd,spec,tasks,coverage,review}`
- Role enforcement: `checks/role-enforce` blocks wrong-role actions

## Project layout

```
mill.yml         — project config (targets, models, gates)
roles/            — role definitions (ROLE.md + lessons.md)
checks/           — gauntlet hooks + phase gates
skills/           — agent skills (using-mill.md is the entry point)
docs/             — ADRs, conventions, research, PRODUCT.md
.mill/            — local state: role, state.json, ledger/, phases/
.omp/             — harness config: RULES.md (sticky), AGENTS.md (context)
```
