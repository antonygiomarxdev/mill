# Mill — Agent Delegation Harness

If the user says "using mill" or wants to delegate, load:

@.mill/skills/using-mill.md

Then follow its instructions: read `.mill/roles/COMMON.md` first, then run the
dispatch procedure.

## Role files

@.mill/roles/COMMON.md

Your specific role is determined by the coordinator at dispatch time.
Workers read their own role file when the brief says so:

- Staff: @.mill/roles/staff/ROLE.md
- PM: @.mill/roles/pm/ROLE.md

## Topology

One coordinator (Staff) dispatches workers and sequences the work. Workers
execute and report; no worker dispatches another worker.

```
coordinator (Staff)
  ├── PM
  ├── Architect
  ├── Tech Lead
  ├── Sr Dev (BE / FE / Data)
  ├── Reviewer
  ├── QA / Docs
  ├── UX Designer
  ├── UI Designer
  └── Policy Author
```

The organisational sequence is preserved as pipeline stages, not delegation
handoffs: `FRD → spec → tasks → implementation → review`.

## Quality gates

- Phase gates: `.mill/checks/gate-{frd,spec,tasks,coverage,review,handoff}`
- Verification at the dispatch boundary: `.mill/checks/mill-verify` runs the
  gauntlet (build/lint/test) and enforces role permissions over a worker's
  change set — not git hooks (ADR 0009)
- What "build", "test" and "coverage" mean is per project — Mill ships no
  language-specific tooling of its own (ADR 0006)

## Project layout

```
.mill/            — the Mill framework (roles, checks, skills, docs)
  roles/          — role definitions (ROLE.md + lessons.md)
  checks/         — gate scripts + role-enforce + mill-verify
  skills/         — agent skills (using-mill.md is the entry point)
  docs/           — ADRs, PRODUCT.md
  phases/         — phase artifacts (frd, spec, tasks, review)
checks/           — gate scripts shipped to scaffolded projects
scaffold/         — the template copied into a new project
.omp/             — harness config: RULES.md (sticky), AGENTS.md (context)
docs/             — ADRs, research, FINDINGS, ROADMAP
```
