---
name: using-mill
description: Use when delegating work to specialised roles — dispatching a worker, building a brief from a ROLE.md, answering a raised hand, verifying a worker's result against phase gates, or deciding which role comes next. Triggers on "delegate this", "dispatch a worker", "who should do this", "hand this to the architect", and on any request to build a feature, fix a bug, write a spec or review work where the work should go to a role rather than be done in this session.
---

# Using Mill

Dispatch worker roles through Orca's orchestration CLI. One coordinator (Staff) sequences work; workers execute and report.

## Topology

```
coordinator (Staff)
  ├── PM              — FRDs, backlog, priorities
  ├── Architect       — ADRs, specs, system boundaries
  ├── Tech Lead       — decomposition, code review, quality gates
  ├── Sr Dev BE       — backend implementation
  ├── Sr Dev FE       — frontend implementation
  ├── Sr Dev Data     — database implementation
  ├── Reviewer        — spec compliance, code quality
  ├── QA / Docs       — tests, changelogs, documentation
  ├── UX Designer     — flows, wireframes, interaction specs
  └── UI Designer     — tokens, components, visual specs
```

The coordinator is the hub. It dispatches to a role, receives the result, decides the next step, and dispatches again. No worker dispatches other workers.

## Precondition: Orca must be up

Every dispatch depends on the Orca runtime. When it is down, commands fail with
`Orca is not running. Run 'orca open' first.` — mid-dispatch, after a task may
already have been created.

Check and start it before dispatching anything, not after a failure:

```bash
orca status 2>/dev/null | grep -q "runtimeReachable: true" || orca open
```

Wait for readiness before continuing:

```bash
until orca status 2>/dev/null | grep -q "runtimeReachable: true"; do sleep 2; done
```

`orca open` launches the desktop app. `orca serve` starts a headless runtime and
is the right choice where no desktop session exists — it is untested here
(ADR 0005).

If a dispatch failed because the runtime dropped, check whether the task was
created before re-creating it:

```bash
orca orchestration task-list --run <run_id>
```

A task that exists but was never dispatched is started with `worker-start`; do
not create a second task for the same work.

## The cycle

### 1. Read the issue

Read the issue (or FRD, or task description). Identify:
- What pipeline stage it is in (`stage:spec`, `stage:design`, `stage:dev`, `stage:review`)
- Which role should work on it next
- What the acceptance criteria are (or what they should be, if the role is PM)

### 2. Pick the role

| Stage | Role | Produces |
|-------|------|----------|
| `stage:spec` | PM | FRD with measurable acceptance criteria |
| `stage:design` | Architect | ADR + spec.md |
| `stage:dev` | Tech Lead | Task decomposition + code review |
| `stage:dev` (implementation) | Sr Dev (BE/FE/Data) | Code, tests, commits |
| `stage:review` | Reviewer | APPROVED or CHANGES verdict |
| any | QA/Docs | Tests, changelogs, documentation |
| `stage:design` (UX) | UX Designer | Flows, wireframes, interaction specs |
| `stage:design` (UI) | UI Designer | Tokens, component specs, redlines |

Multi-role sequences for a feature:

```
PM (FRD) → Architect (spec) → Tech Lead (decomposition) → Sr Dev (implementation) → Reviewer (verification)
```

Dispatch one role at a time. Do not dispatch the next until the current one reports back and you verify its output.

### 3. Read the role's ROLE.md

Before building the brief, read the worker's `.mill/roles/<role>/ROLE.md`. It tells you:
- What the role produces
- Its acceptance criteria
- Its `allowed_files`
- Its constraints and rules

The ROLE.md is the worker's contract. Your brief adds the specific context for this task.

### 4. Build the brief

Compose a brief that a worker can execute without asking you for context. Keep it short — reference files instead of inlining content.

```markdown
# [Task Name]

> **Role:** <role-name> | **Model:** <tier>

## Context
<!-- what the agent needs to know: relevant files, decisions, constraints -->

## Acceptance
<!-- measurable criteria — numbers, greps, counts, never adjectives -->
<!-- max 9 criteria. Each is a command + expected output -->
- [ ] `grep -c "thing" src/file.ts` returns `3`
- [ ] `pnpm test -- path/to/test` passes

## Do not touch
<!-- files or patterns explicitly out of scope -->
- `src/legacy/`

## Deliverable
<!-- what artifact proves completion -->
- Commits: ≥N
- Files: `<paths>`

## Steps
<!-- one action per step, 2-5 min, TDD where applicable -->
- [ ] 1. Write failing test
- [ ] 2. Run test → FAIL
- [ ] 3. Implement → test PASS
- [ ] 4. Commit: `feat(scope): description`
- [ ] 5. Gate: `pnpm lint && pnpm type-check && pnpm build`
```

Rules:
- Criteria are countable. Numbers, greps, measurements — never adjectives.
- Open with the deliverable in the imperative.
- Briefs must be short. A worker given its `ROLE.md` and a brief has everything it needs.
- State requirements rather than prohibitions where you can. Whether prohibitions
  underperform is under investigation (#160); what is measured here is that a
  brief saying "keep role-enforce and the gate loop exactly as they are, do not
  touch them" produced a result that deleted both. The reference implementer
  template Orca ships is 142 lines with no prohibition section at all.

### 5. Dispatch

```bash
# Create the task — the brief IS the spec
TASK=$(orca orchestration task-create --run <run_id> \
  --task-title "<short title>" \
  --spec "$(cat brief.md)" --json | grep -oE 'task_[a-z0-9]+' | head -1)

# Start a supervised worker in its own worktree
orca orchestration worker-start --run <run_id> --task $TASK \
  --agent command-code \
  --worktree new-child --name <name> --repo path:<repo>

# Watch it work — reasoning, tool calls and all
orca orchestration worker-read --dispatch <ctx_id>

# Read reports and questions
orca orchestration inbox --limit 5 --full
```

Notes measured in practice:

- `--agent` must be an identifier Orca has configured. `command-code` works;
  `commandcode` and `cmd` are rejected with "A configured --agent is required".
- With `--worktree new-child` the brief is injected and submitted automatically.
  With `--worktree current` it is not — send it yourself:
  `orca terminal send --terminal <handle> --text "<brief>" --enter`
- `--model` accepts Claude, Codex and Cursor identifiers only. For other agents
  the model is whatever the agent's own configuration selects. See
  **Model selection** below — the tier is chosen by picking the agent.

### 6. Handle questions

If a worker raises a hand (`--type question`), answer clearly and completely:

```bash
orca orchestration reply --id <message-id> --body "<your answer>"
```

Do not rush the worker. If the question reveals ambiguity in the brief, answer and let the worker proceed.

### 7. Receive the result

The worker reports back with `worker_done`. Read:
- `--body`: 3-sentence executive summary
- `--report-path`: path to full artifact (if present)
- `--outcome`: succeeded or failed
- `--files-modified`: paths changed

### 8. Verify

Before accepting the result:

1. **Read the report** (or `--report-path` artifact)
2. **Recalculate every quantitative claim** — do not trust the worker's numbers
3. **Run the gates** — lint, type-check, build, test. Run them yourself.
4. **Check `allowed_files`** — verify `git diff --stat` against the role's file permissions
5. **Check scope** — nothing outside the brief was touched

If the role was non-leaf (PM, Architect, Tech Lead), decide what role dispatches next based on the pipeline stage.

### 9. Phase gates

Before marking work approved, run the appropriate gate:

| Gate | When | Script |
|------|------|--------|
| `gate-frd` | PM completes an FRD | `checks/gate-frd <issue>` |
| `gate-spec` | Architect completes a spec | `checks/gate-spec <issue>` |
| `gate-tasks` | Tech Lead completes decomposition | `checks/gate-tasks <issue>` |
| `gate-coverage` | Before merge | `checks/gate-coverage <issue>` |
| `gate-review` | Reviewer completes | `checks/gate-review <issue>` |
| `gate-handoff` | Non-leaf role completes | `checks/gate-handoff <issue>` |

If a gate fails, dispatch the role back with the failure details.

### 10. Handle failures

If a worker reports `--outcome failed` or BLOCKED:
- Read the body for specifics
- If it is a context problem, provide more context and re-dispatch
- If it requires more capability, re-dispatch with a more capable model
- If the brief was wrong, fix the brief and re-dispatch
- If the blocker is beyond delegation, escalate to the CTO

Never ignore a failure. Never re-dispatch without changing something.

## Model selection — the tier is the agent

Expensive models decompose and review; cheap models write. That split is the
reason this project exists, so it has to survive contact with the tooling.

**Select the tier by choosing the agent, not by passing a model flag.**

| Role | Tier | Dispatch with |
|------|------|---------------|
| PM, Architect, Tech Lead, Reviewer | thinks | `--agent claude --model <id>` |
| Sr Dev (BE/FE/Data), QA/Docs | writes | `--agent command-code` |
| UX Designer, UI Designer | thinks | `--agent claude --model <id>` |

### Why not `--model` on every dispatch

`worker-start --model` accepts Claude, Codex and Cursor identifiers only. For any
other agent the model is whatever that agent's own configuration selects — a
`command-code` worker reports `xiaomi/mimo-v2.5-pro`, read from the global
`~/.commandcode/config.json`.

Measured, so nobody re-derives it:

| Approach | Result |
|---|---|
| `worker-start --model <id>` with `--agent command-code` | ignored |
| `.commandcode/config.json` inside the project | not read; the global wins |
| `command-code --config model=<id>` on the command line | **works** |
| Orca passing extra args through to the agent | no mechanism |

So the per-model route is closed until Orca can pass arguments through. Choosing
the agent is open, gives the two tiers the cost model needs, and requires nothing
from anyone else.

Do **not** rewrite `~/.commandcode/config.json` between dispatches to fake a
tier. It is global, and parallel workers would race on it.

Escalate a role to the thinking tier when a cheap dispatch fails on judgment
rather than on execution. Record which tier ran — it is the only evidence the
cost model is working at all.

## Worktree and worker lifecycle

Orca owns both. A finished worker is not cleaned up automatically, and that is
deliberate: its terminal may hold context worth reading, and its worktree may
hold work the coordinator has not verified yet. **Close it explicitly, after
verifying.**

```bash
orca orchestration worker-release --dispatch <ctx_id>   # archives output, frees the terminal
orca worktree rm --worktree name:<name>                 # removes the worktree
```

`worktree rm` refuses when the worktree has uncommitted changes, naming the
files. **Never pass `--force` without looking at them first.** A refusal means
something is there that was not landed — read it, land it or discard it
deliberately, and only then force.

`worker-retain` keeps a terminal alive when a failure is worth investigating.
`worker-abandon` fences a worker whose state is uncertain without claiming it
stopped.

Left undone, this accumulates: 13 worktrees and 63 MB built up in a single
session before anyone looked.

## The record

Orca holds the dispatch record. It survives compaction; trust it over your own
recollection.

```bash
orca orchestration task-list --run <run_id>    # every task and its state
orca orchestration inbox --limit N --full      # reports, questions, answers
orca orchestration worker-read --dispatch <id> # what an agent did, live or after
```

Worker reports carry a structured payload — `outcome`, `filesModified`,
`taskId`, `dispatchId` — which is what makes a result checkable rather than
narrated.

**A report is not evidence.** Read the files and run the commands yourself
before landing anything. Reports have claimed passing acceptance criteria while
the central document still contradicted the decision it was meant to implement,
and have described a file as never having existed when it was simply untracked.
