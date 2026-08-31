---
name: delegate
description: >-
  Dispatch a Mill worker role and verify what it returns.
  Use when the user says delegate, delegar, despachar, dispatch, or asks for
  work to be handed to a role; when choosing which role fits a task; when
  building a brief; when a worker raises a hand; or when verifying and landing
  a worker's branch.
---

# Delegate

Dispatch a worker role through Orca's orchestration CLI. The coordinator (the
Product Engineer) sequences work; workers execute and report. Read
`.mill/roles/COMMON.md` first, then the worker's own `ROLE.md`.

## 1. Picking a role

| Role | Work that goes to it |
|---|---|
| product-engineer | Coordinator. Decides sequence, dispatches, verifies. Not dispatched itself. |
| pm | Functional Requirements Documents from CTO vision. Measurable acceptance criteria, priority. |
| architect | Architecture Decision Records and technical specs from PM's FRDs. |
| tech-lead | Task decomposition for Sr Devs and code review against architecture. |
| sr-dev-be | Server-side code: Deno, Supabase, Edge Functions, REST, database access. |
| sr-dev-fe | UI code: React, React Native, CSS, design tokens, components. |
| sr-dev-data | Database code: Drizzle, PostgreSQL, migrations, query optimisation. |
| reviewer | APPROVED or CHANGES verdict with evidence. Last gate before coordinator verification. |
| qa-docs | Tests, changelogs, documentation. Shared service for any role. |
| ux-designer | User flows, information architecture, interaction specs from PM. |
| ui-designer | Visual specs, design tokens, component specs from UX wireframes. |
| policy-author | Maintenance of `.mill/**` — roles, skills, gates, consistency. Markdown and bash only. |

Multi-role sequence for a feature:
`PM (FRD) → Architect (spec) → Tech Lead (decomposition) → Sr Dev (impl) → Reviewer (verdict)`.
Dispatch one role at a time. Do not dispatch the next until the current
reports back and you have verified its output.

## 2. The model tier follows the work, not the role

Mechanical work (renames, moves, concatenation, deletion) goes to the free
tier. Judgement work (specs, review, ADRs, anything where being wrong is
expensive) goes to the pro tier. A role's `model:` frontmatter is a default,
not a rule. The (agent, model) pair for each tier lives in `.mill/agents`
(gitignored; `.mill/agents.example` is the template).

**Known limit — measured today:** `--agent command-code` rejects `--model`.
The model comes from `~/.commandcode/config.json`, which is global and which
command-code rewrites itself. Two dispatches on different tiers cannot run
concurrently against the same agent. Record which tier ran.

## 3. The dispatch commands

```
orca orchestration task-create --spec "<brief text or path>" --task-title "<short>"
orca orchestration worker-start --task <task_id> --agent <agent> \
    --worktree new-top-level --name mill-<slug>
orca orchestration check
orca orchestration reply
```

Make sure Orca is running before you dispatch:
`orca status | grep -q "runtimeReachable: true" || orca open`.
A dispatch against a stopped runtime fails partway, sometimes after the task
is already created — pre-create the task with the brief as the spec.
Read the dispatch record in Orca, not your memory:
`orca orchestration task-list`, `orca orchestration inbox`, and
`orca orchestration worker-read --dispatch <ctx_id>` for what an agent did.

## 4. Brief structure

Every brief that worked has the same five parts. Write them in order:

1. **Why.** What the work is for and who it is for. A brief without a *why*
   leaves the worker guessing at intent.
2. **What to produce.** The deliverable, named. Open with the imperative.
3. **Do not touch.** Paths the worker must not modify. **When a criterion and
   a DO NOT conflict, DO NOT wins and the worker raises a hand.** State this
   in the brief — prohibitions cost real time when omitted.
4. **Acceptance criteria.** Numbered. Each is a runnable command whose raw
   output the worker pastes. Max 9. Countable — never adjectives.
5. **Raise a hand.** The line a worker sends when the brief is unclear:
   `orca orchestration send --type question --subject "<short>" --body "<q>"`.

Reference files rather than inlining their content. A worker given its
`ROLE.md` and a brief has everything it needs.

## 5. Verify and land

```
.mill/checks/mill-verify --worktree <path> --role <role> --files-modified "<list>"
```

Run it from the **coordinator's** repository, never from the worktree — the
worktree is the thing being judged. `--files-modified` entries may carry a
git status letter (`A`/`M`/`D`/`R`); a `D` is skipped because a deletion is
not a write. Landing requires exit 0 plus the coordinator re-running the
acceptance criteria itself — never trusting the worker's report. Full
procedure: `.mill/roles/product-engineer/ROLE.md`.