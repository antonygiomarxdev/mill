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

Dispatching work through Orca's orchestration CLI is what establishes your
identity: `[Mill · Product Engineer] coordinator — you delegate and verify; you
do not write implementation code. Dispatch with <orca> orchestration
worker-start; verify with .mill/checks/mill-verify.`
Workers execute and report; you sequence work. Read `.mill/roles/COMMON.md`
first, then the worker's own `ROLE.md`.

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

Where an agent can switch its own model, prefer that over predicting it here:
`prewalk` is the case that exists today — it moves the agent to the `smol` role
at the first edit/write once the plan's todo list exists, so the model follows
the work as observed rather than as predicted by the coordinator.

The landing commit records what ran:

    Mill-Dispatch: role=<role> agent=<agent> model=<model> task=<task_id>
    Co-Authored-By: ...
    Claude-Session: ...

Two rules, both required, or git parses nothing:
1. Blank line before the block — it is the last paragraph.
2. No blank line inside the block.

Attach the block to the prose and git parses zero trailers (`f22dea7`); leave
a blank line inside and `Mill-Dispatch` is demoted, the other two parse (`24f3019`).

The `model=` field is filled from the session transcript after the worker
settles (`~/.omp/agent/sessions/<worktree-slug>/*.jsonl`), not guessed at
dispatch time. After landing, run
`git log -1 --format="%(trailers:only=true)"` and confirm `Mill-Dispatch`
appears — it would have caught `24f3019` and `f22dea7`.

`.mill/agents` is a catalog of what runs on this machine (gitignored;
`.mill/agents.example` is the tracked template). `mill-dispatch` reads each
agent's `submit:` marker from it — the local copy when present, the example
otherwise — to decide whether the brief needs an explicit enter after it is
pasted: `submit: explicit` agents get one, `submit: self` agents never do. A
missing catalog or a missing marker is a loud refusal (exit 2), never a
silent wrong guess.

## 3. The dispatch commands

Load Orca's coordination guide before any command below. It is version-matched
to the installed binary, and Mill does not restate it — dispatch, messaging,
waiting, releasing and reading a worker are Orca's surface and change between
releases.

```
<orca> skills get orca-cli
<orca> skills get orchestration
```

The guide owns, by name:

- `check` — wait for and acknowledge a worker's report
- `worker-release` — reclaim a settled worker
- `send` — give a live worker follow-up work
- `worker-read` — read a worker's transcript

Read the guide for their syntax; the names above exist only so a reader who
skipped it knows what it covers.

Mill's own dispatch loop is one command:

```
.mill/checks/mill-dispatch --brief <file> --role <role> --agent <agent> \
    --name <slug> --title <title> --writes <path> [--writes <path> ...]
```

It sequences preflight → task → worker → wait → report → release; it never
verifies or lands, which remain the coordinator's own steps (section 5).

`mill-dispatch` blocks until the worker settles. The coordinator must not solve
that by backgrounding it in its own shell: a background job of the harness is
invisible to `<orca> terminal list`, is unreadable with `<orca> terminal read`, and
dies with the session — and when it dies, the worker it was supervising is
never released. The supervisor runs as an Orca terminal:

```
<orca> terminal create --worktree path:<project-root> \
    --title "SUPERVISOR <slug>" \
    --command "<orca> orchestration run-use --id <run_id> && .mill/checks/mill-dispatch --brief <file> --role <role> ..."
```

A supervisor hosted that way appears in `<orca> terminal list`, is readable with
`<orca> terminal read`, and survives the coordinator's session.

The `run-use` is not optional and is the part that is easy to miss. A Run is
bound per terminal, so a freshly created terminal has none, and `task-create`
refuses before anything is created:

    mill-dispatch: task-create failed:
      "code": "run_required",
      "message": "No Run is bound. Use orchestration run-create or run-use first.
                  No effects were applied."

That failure is clean — no task, no worktree, no worker — but it is silent
unless the coordinator reads the supervisor terminal, which is the habit this
section exists to establish.

The supervisor no longer relies only on listening for a worker's message: it
slices its wait into short windows and runs `.mill/checks/mill-liveness
--dispatch <ctx_id>` between them. A worker killed by a provider error sends no
message, so a listen-only loop would wait out the whole `--timeout-ms`
supervising a corpse; the liveness probe notices a dead, parked, or uncommitted
worker within one slice and stops. The single wall-clock deadline from
`--timeout-ms` is unchanged and remains the sole source of truth for "time is
up".

After creating the supervisor, the coordinator launches `<orca> terminal wait
--terminal <handle> --for exit` so it is woken when the dispatch ends instead
of polling. The supervisor renames its own tab on every exit to encode the
verdict (`done`, `dead`, `parked`, `uncommitted`, `timeout`, `failed`), so a
glance at the tab tells the coordinator what happened without opening it.

## 4. Brief structure

Every brief that worked has the same five parts. Write them in order:

1. **Why.** What the work is for and who it is for. A brief without a *why*
   leaves the worker guessing at intent.
2. **What to produce.** The deliverable, named. Open with the imperative.
3. **Do not touch.** Paths the worker must not modify. **When a criterion and
   a DO NOT conflict, DO NOT wins and the worker raises a hand.** State this
   in the brief — prohibitions cost real time when omitted.
4. **Acceptance criteria.** Numbered. Each is a runnable command whose raw
   output the worker pastes. Max 9. Countable — never adjectives. Every brief
   carries these two commands verbatim as its last acceptance criterion:

   ```
   git log --oneline main..HEAD    # at least one commit
   git status --short              # empty
   ```

   `git diff --stat`, which several briefs have used, passes identically on an
   uncommitted tree and cannot tell the two apart.
5. **Raise a hand.** The line a worker sends when the brief is unclear:
   `<orca> orchestration send --type question --subject "<short>" --body "<q>" --task-id <task-id> --dispatch-id <dispatch-id>`.
   A question is tied to its dispatch by the sender's own terminal —
   `mill-verify --dispatch` resolves the handle from the dispatch record, and
   a payload-less question carries nothing else. `--task-id` and
   `--dispatch-id` put the ids in the payload, which keeps the question
   attributable once the terminal no longer resolves.

Reference files rather than inlining their content. A worker given its
`ROLE.md` and a brief has everything it needs.

Before dispatching, run `mill-preflight --brief <role> <path>...` with the paths the brief asks the worker to write.

## 5. Verify and land

Before `mill-verify`, run `mill-liveness` and refuse to judge any worktree with
zero commits ahead of `main`, whatever the worker's report says:

    .mill/checks/mill-liveness --dispatch <ctx_id>

Exit 0 working, 10 finished, 20 parked on a human prompt, 30 dead, 40 sitting
on uncommitted work, 2 usage or unresolvable. This check does not depend on the
worker having read anything — it reads the worktree's git state and the
terminal's cursor directly — which is why it is the one that matters (#214).

```
.mill/checks/mill-verify --project-root <path> --worktree <path> --role <role> --files-modified "<a,b,c>"
.mill/checks/mill-verify --dispatch <ctx_id>                   # refuse while a question is unanswered
```

`--project-root` names the project whose `.mill/gauntlet` and
`.mill/role-capabilities` judge the worktree; it has no default because inside
the script the working directory is the tree under judgement — a guess there
would read the permissions file from the tree being judged. The rule is ADR
0014 (`docs/adr/0014-two-roots-install-and-project.md`), not restated here.

Run it from the **coordinator's** repository, never from the worktree — the
worktree is the thing being judged. `--files-modified` entries may carry a
git status letter (`A`/`M`/`D`/`R`); a `D` is skipped because a deletion is
not a write. Landing requires exit 0 plus the coordinator re-running the
acceptance criteria itself — never trusting the worker's report. Full
procedure: `.mill/roles/product-engineer/ROLE.md`.

### 5.1 Write back to the issue

A landing writes one comment to its issue only when the landing commit closes
it. The trigger is mechanical, not judged:

- the subject carries `(#N)`, or
- the body carries `Closes #N`, `Fixes #N`, or `Resolves #N`.

A bare `#N`, `Refs #N`, or a `(#N)` in the body is not a close — it writes
nothing. On 2026-08-31 this selected 4 of 38 landings (#184, #159, #132,
#176); the rest named no issue or merely touched one. A comment per landing is
noise.

The issue number and commit come from the close signal, never re-typed:

    gh issue comment <N> --body "Closed by <sha>. <what remains>"

`<sha>` is the landing commit; `<what remains>` is the one thing `git log`
cannot give a reader — empty for a clean close, or "kept open for <X>" when the
landing closed only part of the issue. Restate nothing the body already says.
A close with no close signal in the commit is not written back automatically.

## 6. Current vs history

Two rules decide what is authoritative:

1. `AGENTS.md` and `MEMORY.md` describe Mill as it is. Everything else that
   describes Mill — `.mill/phases/`, `docs/research/`, `docs/plans/`,
   superseded ADRs, `LESSONS.md` — is history and may describe a Mill that no
   longer exists.
2. A statement is obsolete when its **subject was deleted**, not when the
   vocabulary changed. Verify with `git`, `ls`, or `gh issue view` before
   calling anything stale.