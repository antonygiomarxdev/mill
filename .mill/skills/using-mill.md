---
name: using-mill
# Description design — docs/research/skill-authoring.md §4.5 documents two
# contradicting camps: superpowers says descriptions must be triggers only
# ("what" makes agents shortcut past the body), while Anthropic's published
# guidance says include both "what" and "when". We follow Anthropic: one-line
# "what" opener plus explicit trigger phrases.
description: >-
  Coordinate multi-role work through Mill's delegation framework.
  Use when dispatching a worker role, building a brief from a ROLE.md,
  answering a raised hand, verifying a worker's result against phase
  gates, deciding which role comes next, or when the user says
  "delegate this", "dispatch a worker", "who should do this", "hand
  this to the architect", or asks to build a feature, fix a bug, write
  a spec, or review work where the work should go to a role rather
  than be done in this session.
---

# Using Mill

Dispatch worker roles through Orca's orchestration CLI. One coordinator (Staff)
sequences work; workers execute and report.

The topology, reporting, and raising-a-hand rules live in
`.mill/roles/COMMON.md` — read it first. This skill is the coordinator's
procedure on top of it.

## When to use Mill — and when not to

**Use Mill when the work is delegable:** it is well-scoped, a role owns it, and
its acceptance criteria are countable. That is almost all feature, bug, spec,
design, review, and documentation work.

**Do it yourself when:** the work is one small edit or a simple question, you
are mid-verification and already hold the context, or it is the policy-author
role's own recurring maintenance of `.mill/`. Ask: *would a worker start colder
than I am right now?* If yes, delegate. If the worker would spend more time
reading context than doing the work, do it yourself.

## Glossary

| Term | Meaning |
|------|---------|
| **FRD** | Functional Requirements Document. PM's product spec: user need, numbered functional requirements, out-of-scope, priority. Gate: `gate-frd`. |
| **ADR** | Architecture Decision Record. A cross-cutting technical decision with context, decision, alternatives, consequences. Lives in `docs/adr/`, immutable once accepted. |
| **Spec** | Technical specification (the Architect's `spec.md`): architecture, components affected, risks, ADR links. Gate: `gate-spec`. |
| **Brief** | The task description a coordinator writes from a `ROLE.md` plus issue context. The brief is the spec — `task-create --spec "$(cat brief.md)"`. |
| **Stage** | Pipeline position: `stage:spec` → `stage:design` → `stage:dev` → `stage:review`. Decides which role is next. |
| **Gate** | A mechanical check in `.mill/checks/` that blocks a phase transition until it passes. A gate that is never exercised is documentation, not enforcement. |

## Picking a role

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
| `.mill/**` | Policy Author | Coherent policy, no contradictions |

The table maps stages to roles, but real dispatch decisions are more nuanced.
Walk the questions instead of matching labels:

```
Is the work product code, or a decision?
├── A decision / document the CTO or user asked for
│   ├── Product decision? ────────────────→ PM
│   ├── Cross-cutting technical decision? ──→ Architect
│   └── Task decomposition / review plan? ─→ Tech Lead
├── Product code
│   ├── Backend? ──→ Sr Dev BE
│   ├── Frontend? ──→ Sr Dev FE
│   └── Data / DB? ─→ Sr Dev Data
├── Reviewing work? ────────────────────────→ Reviewer
├── Tests, changelogs, or docs? ────────────→ QA/Docs
├── User flows / wireframes? ───────────────→ UX Designer
├── Tokens / component specs / redlines? ───→ UI Designer
└── Maintaining .mill/** (roles, skill, gates)? ─→ Policy Author
```

Multi-role sequences for a feature:

```
PM (FRD) → Architect (spec) → Tech Lead (decomposition) → Sr Dev (implementation) → Reviewer (verification)
```

Dispatch one role at a time. Do not dispatch the next until the current one
reports back and you verify its output.

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
is the right choice where no desktop session exists — it is untested here.

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

### 2. Read the role's ROLE.md

Before building the brief, read the worker's `.mill/roles/<role>/ROLE.md`. It tells you:
- What the role produces
- Its acceptance criteria
- Its `allowed_files`
- Its constraints and rules

The ROLE.md is the worker's contract. Your brief adds the specific context for this task.

### 3. Build the brief

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

### 4. Dispatch

```bash
# Create the task — the brief IS the spec
TASK=$(orca orchestration task-create --run <run_id> \
  --task-title "<short title>" \
  --spec "$(cat brief.md)" --json | grep -oE 'task_[a-z0-9]+' | head -1)

# Start a supervised worker in its own worktree
orca orchestration worker-start --run <run_id> --task $TASK \
  --agent command-code \
  --worktree new-child --name <name> --repo path:<repo>

# Wait natively — never poll with a shell loop
orca terminal wait --terminal <handle> --for tui-idle --timeout-ms 300000

# Watch it work — reasoning, tool calls and all
orca orchestration worker-read --dispatch <ctx_id>

# Read reports and questions
orca orchestration inbox --limit 5 --full
```

Notes measured in practice:

- `--agent` must be an identifier Orca has configured. `command-code` works;
  `commandcode` and `cmd` are rejected with "A configured --agent is required".
- **`--agent claude` does not submit the brief** — it lands as a draft and the
  worker never starts (upstream #14505). `--agent command-code` submits on its
  own with `--worktree new-child`, and injects nothing at all with
  `--worktree current`. Always confirm, and submit if needed:

  ```bash
  orca terminal wait --terminal <handle> --for tui-idle --timeout-ms 60000
  orca orchestration worker-read --dispatch <ctx_id>   # is the TASK block still unsent?
  orca terminal send --terminal <handle> --enter        # submit it
  ```
- `--model` accepts Claude, Codex and Cursor identifiers only. For other agents
  the model is whatever the agent's own configuration selects. See
  **Model selection** below — the tier is chosen by picking the agent.

### 5. Read the state — idle is not done

`terminal wait --for tui-idle` returning `satisfied: true` means **the TUI is
not busy**. It does not mean the work finished. Read the terminal and decide
which of three states you are in:

| State | How it looks | What to do |
|---|---|---|
| Delivered | a message in `inbox` with an `outcome` | verify it |
| Waiting for input | prompt empty, or the TASK block still unsent, or `Type "continue" to try again` | send it: `orca terminal send --terminal <handle> --enter` |
| Died | `⚠ Error: …` with a Trace ID in the output | resume with `--text "continue" --enter`, or re-dispatch |

```bash
orca orchestration worker-read --dispatch <ctx_id> | tail -20
```

Nothing surfaces a dead worker on its own. A provider connection error leaves
`task-list` reading `[dispatched]` and `worker-show` reading `[ready]
stage=input_accepted` — identical to a worker that is thinking. One agent sat
in that state after a `Connection error` and was only found by reading its
terminal.

**Never write a shell loop to poll for a result.** Four were written in one
session and all four failed, each waiting on an exact subject line a worker was
never obliged to send — one of them for 78 minutes after its task had already
completed. Use `terminal wait`, then read.

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

If a worker reports `--outcome failed` or BLOCKED, fix the cause — never
re-dispatch unchanged, never ignore:

| Failure | Fix |
|---------|-----|
| Context problem — "which file?", "what does this do?" | Provide the missing context in the brief, re-dispatch |
| Capability problem — "this needs a pro model" | Re-dispatch with a more capable model (switch tier) |
| Wrong brief — acceptance criteria contradict the intent | Fix the brief, re-dispatch |
| Blocker beyond delegation — product/scope call | Escalate to the CTO |
| Died silently — `Connection error`, TUI stuck | Read the terminal; resume with `--text "continue" --enter` or re-dispatch |

### 11. Decision thresholds — when a result is good enough

| Situation | Verdict | Do |
|-----------|---------|----|
| All acceptance criteria verify, gates pass, `allowed_files` respected | **Good enough** | Land it; do not polish |
| Criteria verify but a gate fails | **Not done** | Dispatch the role back with the gate failure |
| Free-tier worker fails on judgment (not execution) | **Escalate tier** | Re-dispatch on the thinking tier; record which tier ran |
| Two workers fail the same task | **Escalate** | To the CTO — do not burn a third dispatch |
| Contradiction between a document and an ADR | **ADR wins** | Fix the document, cite the ADR |

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

## Known Orca defects

Found by running it. None lose work; all cost time if you do not know them.

| Defect | Symptom | What to do |
|---|---|---|
| The brief is not submitted, with `--agent claude` | The `=== TASK ===` block sits in the composer as a draft, `0 tokens` sent, and the worker never starts. **Still reproduces on v1.4.183**, which contains the fix from [#14342](https://github.com/stablyai/orca/pull/14342) — verified after updating, not assumed. `--agent command-code` submits on its own. Upstream issue: [#14505](https://github.com/stablyai/orca/issues/14505). | Read the terminal after dispatch and `orca terminal send --terminal <handle> --enter`. The agent then runs normally |
| Nothing injected at all, with `--agent command-code --worktree current` | The TUI launches on an empty prompt; no TASK block appears. Distinct from the above. | Send the brief yourself: `orca terminal send --terminal <handle> --text "<brief>" --enter` |
| A dead worker looks like a busy one | `task-list` reads `[dispatched]`, `worker-show` reads `[ready] stage=input_accepted`. Nothing distinguishes "thinking" from "died on a provider error". | Read the terminal. `⚠ Error:` with a Trace ID means dead — resume with `--text "continue" --enter` |
| The message counter does not clear | The session is told "You have N orchestration messages" indefinitely. `check --ack`, replying, and closing the originating task all fail to consume the delivery. | **Ignore the counter; read the inbox.** `orca orchestration inbox --limit 5 --full` is accurate and ordered |
| `skills get` is unreachable while Orca runs | `[single-instance] Another Orca instance is already running` — from both `orca` and `orca-ide` | Ask the human to run `orca skills get orca-cli` from an Orca-managed terminal |
| `orchestration ask` outside a worker | `The Dispatch capability is missing` | Expected: `ask` is for dispatched workers. Coordinators use `reply` |
| Mail to a bare terminal handle has no reader | `send --to term_...` returns `ok: true` and the recipient can never read it, once its pane is bound to a Run — which every worker's is. Upstream [stablyai/orca#13656](https://github.com/stablyai/orca/issues/13656). | Address workers by `--to dispatch:<ctx_id>`, or reply to a message they sent. Never by bare terminal handle |

**Check the version and the tracker before diagnosing an Orca behaviour.**
Every item above was rediscovered here by experiment, and the first one was
already fixed and released before it was investigated. The order that would
have cost minutes instead of hours:

```bash
grep -ao '"version": *"[0-9.]*"' /tmp/.mount_orca*/resources/app.asar | head -1
gh search issues --repo stablyai/orca "<symptom>" --state all
gh release list --repo stablyai/orca --limit 3
```

Others filed upstream: mailbox addressing [#13656](https://github.com/stablyai/orca/issues/13656),
a related stdin defect [#12630](https://github.com/stablyai/orca/issues/12630).

A counter that always reads the same number is not a signal. Read the inbox.

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

## Common mistakes

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Dispatching the next role before verifying the current result | The cycle looks sequential; the result looks done | Verify (step 8) before you dispatch again |
| Trusting the worker's report instead of re-running the gates | The report says "all pass" and it is cheaper to believe it | Recalculate every quantitative claim and run the gates yourself |
| Polling with a shell loop for a result | A wait command exists but the loop is habit | Use `terminal wait --for tui-idle`, then read the terminal |
| Taking "idle" for "done" | `tui-idle` sounds like completion | Read the terminal: delivered, waiting for input, or died |
| Re-dispatching without changing anything | The failure looked like bad luck | Every re-dispatch changes the brief, the context, or the tier |
| Delegating work that is faster to do yourself | Mill is available, so it is the default | Apply the boundary: small edits and cold-start-heavy work stay local |

## End-to-end example

A concrete cycle, from issue to landing.

**Issue:** #42 — `internal/ledger` is at 77.1% coverage; project minimum is 90%.

**1. Read the issue.** Stage `stage:dev`, implementation work, backend. Role:
Sr Dev BE. Acceptance: coverage ≥90% in `internal/ledger`, gates clean.

**2. Build the brief** from `.mill/roles/sr-dev-be/ROLE.md` plus context
("the package sits at 77.1%, COMMON.md requires 90%"), acceptance criteria
(greps and gate commands, never adjectives), and a "Do not touch" note.

**3. Dispatch** (Orca up first):

```bash
RUN=$(orca orchestration run-create --objective "coverage" --json | grep -oE 'run_[a-z0-9]+' | head -1)
TASK=$(orca orchestration task-create --run $RUN --task-title "Raise ledger coverage" \
  --spec "$(cat brief.md)" --json | grep -oE 'task_[a-z0-9]+' | head -1)
orca orchestration worker-start --run $RUN --task $TASK \
  --agent command-code --worktree new-child --name ledger-cov --repo path:$(pwd)
orca terminal wait --terminal <handle> --for tui-idle --timeout-ms 300000
```

**4. Verify the result** — do not believe the worker's numbers:

```bash
orca orchestration inbox --limit 5 --full          # the worker_done report
bash .mill/checks/gate-coverage                    # run the gate yourself
```

**5. Land, then close the worker down** — nothing is cleaned up automatically:

```bash
orca orchestration worker-release --dispatch <ctx_id>
orca worktree rm --worktree name:ledger-cov        # read any refusal before --force
```

**Result:** coverage from 77.1% to 94.3% in one dispatch — the run that
demonstrated Mill's policy layer working with Orca's substrate (see ADR 0006).
