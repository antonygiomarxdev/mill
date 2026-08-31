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

## 2. Name the agent and model per dispatch

Every dispatch names the agent and the model explicitly. `worker-start` takes
`--agent`; where that agent accepts `--model`, pass it. Where it does not
(command-code), set the model first and say so. There is no default to hide
behind.

Choose from the work, not from the role. Mechanical work — renames, moves,
concatenation, deletion — and work whose design is fully specified in the
brief can go to a cheap model. Work where being wrong is expensive — gates,
permissions, specs, anything that decides what may land — goes to a capable
one. The same rule was violated in both directions on 2026-08-31: `67a738d`
(a two-line CI path edit) ran on the expensive agent, and `4ebcce2` (a change
to how the permission gate resolves its file) ran on the cheap one.

The landing commit records what ran:

    Mill-Dispatch: role=<role> agent=<agent> model=<model> task=<task_id>

`.mill/agents` is a catalog of what exists on this machine. No script consults
it and it decides nothing.

## 3. The dispatch commands

```
.mill/checks/mill-preflight --brief <role> <path> [<path>...]   # refuse a brief that asks for what the role cannot write
orca orchestration task-create --spec "<brief text or path>" --task-title "<short>"
orca orchestration worker-start --task <task_id> --agent <agent> \
    --worktree new-top-level --name mill-<slug>
# `check` reads only deliveries addressed to the bound run; `inbox` shows
# every message across recipients, so an escalation or a worker_done that
# arrived in a different run is not silently dropped.
orca orchestration inbox
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
   `orca orchestration send --type question --subject "<short>" --body "<q>" --task-id <task-id> --dispatch-id <dispatch-id>`.
   A question without `--task-id` and `--dispatch-id` cannot be tied to its
   dispatch and will not be seen by `mill-verify --dispatch`.

Reference files rather than inlining their content. A worker given its
`ROLE.md` and a brief has everything it needs.

## 5. Verify and land

```
.mill/checks/mill-verify --worktree <path> --role <role> --files-modified "<list>"
.mill/checks/mill-verify --dispatch <ctx_id>                   # refuse while a question is unanswered
```

Run it from the **coordinator's** repository, never from the worktree — the
worktree is the thing being judged. `--files-modified` entries may carry a
git status letter (`A`/`M`/`D`/`R`); a `D` is skipped because a deletion is
not a write. Landing requires exit 0 plus the coordinator re-running the
acceptance criteria itself — never trusting the worker's report. Full
procedure: `.mill/roles/product-engineer/ROLE.md`.

## 6. Current vs history

Two rules decide what is authoritative:

1. `AGENTS.md` and `MEMORY.md` describe Mill as it is. Everything else that
   describes Mill — `.mill/phases/`, `docs/research/`, `docs/plans/`,
   superseded ADRs, `LESSONS.md` — is history and may describe a Mill that no
   longer exists.
2. A statement is obsolete when its **subject was deleted**, not when the
   vocabulary changed. Verify with `git`, `ls`, or `gh issue view` before
   calling anything stale.