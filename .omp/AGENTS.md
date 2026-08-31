# Mill — Agent Delegation Harness

If the user says "using mill" or wants to delegate, load:

@AGENTS.md
@.claude/skills/delegate/SKILL.md

Then follow its instructions to bootstrap or activate Mill.

## Project layout

```
.mill/            — Mill framework (roles, skills, checks, phases, state)
  roles/          — role definitions (ROLE.md + lessons.md)
  skills/         — agent skills (wayfinder.md is the synced skill here;
                   the coordinator's procedure is .claude/skills/delegate/SKILL.md)
  checks/         — gauntlet hooks + phase gates
  docs/           — ADRs, PRODUCT.md
  phases/         — phase artifacts (frd, spec, tasks, review)
.omp/             — harness config (this file)
```
