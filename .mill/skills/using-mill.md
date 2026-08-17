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

### After any Orca restart: rebind the Run

The terminal-to-Run binding does not survive a restart, and losing it is silent.
Messages keep arriving at the Run; the coordinator simply stops being told about
them, and only notices by polling — which is the thing this skill forbids.

```bash
orca orchestration run-current            # "No Run is bound to this terminal."
orca orchestration run-use --id <run_id>  # note: --id, not --run
```

Check this whenever notifications stop, and after every Orca update. It cost
several hours here before anyone thought to look, because the symptom is silence.

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
- Its `allowed_files` categories (mapped to file patterns per project in `.mill/role-capabilities`)
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
  orca orchestration worker-read --dispatch <ctx_id> --limit 20 --json
  orca terminal send --terminal <handle> --enter   # submit the draft
  ```
- `worker-start --model` accepts Claude, Codex and Cursor identifiers only — for
  anything else the model is whatever the agent's own configuration selects. See
  **Model selection** below — the tier resolves to an (agent, model) command
  from `.mill/agents`, dispatched through a terminal.

### 5. Wait, and read what came back

The supervision loop is documented in Orca's own guide — `skill-guides/orchestration.md`
in `stablyai/orca`, or `orca skills get orca-cli`. Follow it rather than inventing one:

```bash
orca orchestration check --wait --types worker_done,escalation,question \
  --timeout-ms 900000 --json
```

`check` returns the Run's oldest Delivery and **replays that exact batch until
acknowledged by id**. Process every message, reply to any question, then
acknowledge and keep waiting in one call:

```bash
orca orchestration check --ack <delivery_id> --wait \
  --types worker_done,escalation,question --timeout-ms 900000 --json
```

`--ack` takes the delivery id. Bare `--ack` acknowledges nothing, and the
session is then told it has messages indefinitely.

**What is not a failure**, per the guide:

- a `check --wait` timeout, or `{count: 0}` — that is a checkpoint. Coding tasks
  routinely run 15–60 minutes. Keep waiting with rolling waits.
- heartbeats or visible terminal activity — they mean alive, not done.
- **TUI idle state** — the guide lists it explicitly as a reason *not* to release
  a worker. `terminal wait --for tui-idle` answers "is the interface busy", which
  a parked worker also answers no to. It is not a completion signal.

Stop waiting only on `worker_done` or `escalation`, on the terminal exiting or
disappearing, or when the human says so.

**A valid `worker_done` completes the task and dispatch automatically.** Never
follow it with `task-update --status completed` — a manual update on a settled
task is how one ends up `blocked`.

To see what an agent actually did, read its transcript rather than raw terminal
scrollback:

```bash
orca orchestration worker-read --dispatch <ctx_id> --limit 50 --json
```

`--source auto` returns the hook-reported transcript when Orca can prove the
session, otherwise bounded terminal output with a typed `fallbackReason`.
Continue with the returned `cursor`.

**Never write a shell loop to poll for a result.** Four were written in one
session and all four failed, each waiting on an exact subject line no worker
was obliged to send — one of them for 78 minutes after its task had completed.

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

Before accepting the result, run the verification entry point in the worker's
worktree. It is the gauntlet plus role enforcement in one command — the hook
never was this (see ADR 0009):

```bash
.mill/checks/mill-verify --worktree <worktree-path> --role <role> \
  --files-modified "<comma-separated paths from worker_done>"
```

`mill-verify` runs the configured build, lint and test steps (from
`.mill/gauntlet`) in that worktree, then enforces the role's `allowed_files`
over the change set — `--files-modified` when given (the `worker_done` payload
is the authoritative record), otherwise the git diff since the worktree's base
commit — and rejects any uncommitted work left behind. Pass only with real
output, not the worker's numbers:

1. **Read the report** (or `--report-path` artifact)
2. **Recalculate every quantitative claim** — do not trust the worker's numbers
3. **Run the gates yourself** — `mill-verify` is the command that does; run it
   in the worker's worktree
4. **Check `allowed_files`** — `mill-verify` enforces the role over the change
   set; `git diff --stat` shows you the scope it judged
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

## Model selection — the tier resolves to a command

Expensive models decompose and review; cheap models write. That split is the
reason this project exists, so it has to survive contact with the tooling.

**Select the tier by resolving the role's `model:` field through `.mill/agents`.**
Each tier is an (agent, model) pair; the model rides on the agent's command
line, never on `worker-start --model`.

**The tier follows the work, not the role.** A role that "thinks" still produces
by volume when it writes a document or reads a codebase, and volume is what the
cheap tier is for.

| Work | Tier | Resolved from `.mill/agents` |
|------|------|------------------------------|
| Producing — code, tests, docs, specs, FRDs, research | free | `free_agent` / `free_model` |
| Producing that failed on judgment | paid | `paid_agent` / `paid_model` |
| Judging — reviewing a result, deciding between options, verifying a claim | pro | `pro_agent` / `pro_model` |

An Architect writing 300 lines of research is producing. The same Architect
deciding whether a spec answers its FRD is judging. Dispatch the first cheap and
the second expensive, whatever the role's name.

This is the original economics: cheap models write, expensive models review.
Quality comes from the review step, not from the writer — so that is where the
money goes and nowhere else.

Getting this wrong is expensive and silent. Two document-production tasks were
dispatched to `claude-opus-4-5` because a per-role table said PM and Architect
"think".

### Choosing the tier — the measured route

`worker-start --model` accepts Claude, Codex and Cursor identifiers only. For any
other agent the model is whatever that agent's own configuration selects — a
`command-code` worker reports `xiaomi/mimo-v2.5-pro`, read from the global
`~/.commandcode/config.json`.

The route that works for every agent passes the model to the agent's own
command line, through the terminal the worker runs in. Measured in this repo:

| Approach | Result |
|---|---|
| `worker-start --model <id>` with `--agent command-code` | ignored |
| `.commandcode/config.json` inside the project | not read; the global wins |
| `orca terminal create --command "omp --model <id>"` + `worker-start --terminal` | **works** for any agent |

So the per-model route is open. The procedure:

1. **Read the tier.** `.mill/roles/<role>/ROLE.md` frontmatter, `model:` field:
   `free→paid` or `pro`.
2. **Resolve it through `.mill/agents`.** Plain bash, one (agent, model) pair per
   tier — `free_agent`/`free_model`, `paid_agent`/`paid_model`,
   `pro_agent`/`pro_model`. A `free→paid` role dispatches on `free` first and
   re-dispatches on `paid` when the cheap attempt fails on judgment; a `pro`
   role goes straight to `pro`. The file is per-project and gitignored;
   `.mill/agents.example` is the template.
3. **Create the worktree**, if one is not already selected:
   `orca worktree create --name <name> --repo path:<repo> --setup skip`
4. **Create the terminal with that command**:
   `orca terminal create --worktree name:<name> --title <title> --command "<agent> --model <model>" --json`
5. **Dispatch onto it** — two measured constraints:
   - `--terminal` requires a matching `--worktree` selector, or the start fails
     with `terminal_worktree_mismatch`.
   - `--agent` and `--terminal` are alternatives, not companions.

   ```bash
   orca orchestration worker-start --run <run> --task <task> \
     --worktree name:<name> --terminal <handle>
   ```

6. **Check whether the brief submitted.** `omp` does not submit the injected
   brief — the terminal shows `[Paste #1, +107 lines]` and the worker sits idle
   until the draft is sent. Submit it:

   ```bash
   orca terminal send --terminal <handle> --enter
   ```

   Which agents draft and which submit is in the known-defects table below.

Do **not** rewrite `~/.commandcode/config.json` (or any agent's global config)
between dispatches to fake a tier. It is global, and parallel workers would race
on it — the terminal route makes that hack unnecessary.

Escalate a role to the paid tier when a free dispatch fails on judgment rather
than on execution. Record which tier ran — it is the only evidence the cost
model is working at all.

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
| The brief is not submitted — with `--agent claude`, `omp`, or `pi` | The `=== TASK ===` block sits in the composer as a draft (`[Paste #1, +N lines]`), `0 tokens` sent, and the worker never starts. These agents draft because their registry entries carry `draftPromptEnvVar` (e.g. `ORCA_OMP_PREFILL`) — that field governs, not `promptInjectionMode`. `command-code` has no such field and submits on its own. **Still reproduces on v1.4.183**, which contains the fix from [#14342](https://github.com/stablyai/orca/pull/14342) — verified after updating, not assumed. Upstream issue: [#14505](https://github.com/stablyai/orca/issues/14505). | Read the terminal after dispatch and `orca terminal send --terminal <handle> --enter`. The agent then runs normally |
| Nothing injected at all, with `--agent command-code --worktree current` | The TUI launches on an empty prompt; no TASK block appears. Distinct from the above. | Send the brief yourself: `orca terminal send --terminal <handle> --text "<brief>" --enter` |
| A dead worker looks like a busy one | `task-list` reads `[dispatched]`, `worker-show` reads `[ready] stage=input_accepted`. Nothing distinguishes "thinking" from "died on a provider error". | Read the terminal. `⚠ Error:` with a Trace ID means dead — resume with `--text "continue" --enter` |
| ~~The message counter does not clear~~ | **Not a defect — a usage error.** `check --ack` takes the delivery id: `check --ack <delivery_id>`. Bare `--ack` acknowledges nothing and the Delivery replays forever. | `orca orchestration check --ack <delivery_id> --wait --types worker_done,escalation,question` |
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
| Polling with a shell loop for a result | A wait command exists but the loop is habit | `orca orchestration check --wait --types worker_done,escalation,question` |
| Taking "idle" for "done" | `tui-idle` sounds like completion | The guide says not to act on idle. Wait for `worker_done` |
| Marking a task completed by hand | The status looks stale after a report | `worker_done` already settled it; a manual `task-update` leaves it `blocked` |
| Diagnosing Orca from `--help` | The guide is not in the CLI stub | Read `skill-guides/orchestration.md` in `stablyai/orca` |
| Re-dispatching without changing anything | The failure looked like bad luck | Every re-dispatch changes the brief, the context, or the tier |
| Delegating work that is faster to do yourself | Mill is available, so it is the default | Apply the boundary: small edits and cold-start-heavy work stay local |

## End-to-end example

A concrete cycle, from issue to landing.

**Issue:** #42 — a small backend feature on the API; project gates must pass.

**1. Read the issue.** Stage `stage:dev`, implementation work, backend. Role:
Sr Dev BE. Acceptance: feature implemented, project gates clean.

**2. Build the brief** from `.mill/roles/sr-dev-be/ROLE.md` plus context
(the API has no health endpoint), acceptance criteria
(greps and gate commands, never adjectives), and a "Do not touch" note.

**3. Dispatch** (Orca up first):

```bash
RUN=$(orca orchestration run-create --objective "feature" --json | grep -oE 'run_[a-z0-9]+' | head -1)
TASK=$(orca orchestration task-create --run $RUN --task-title "Add health endpoint" \
  --spec "$(cat brief.md)" --json | grep -oE 'task_[a-z0-9]+' | head -1)
orca orchestration worker-start --run $RUN --task $TASK \
  --agent command-code --worktree new-child --name health-ep --repo path:$(pwd)
orca terminal wait --terminal <handle> --for tui-idle --timeout-ms 300000
```

**4. Verify the result** — do not believe the worker's numbers:

```bash
orca orchestration inbox --limit 5 --full          # the worker_done report
bash .mill/checks/gate-review 42                    # run the gate yourself
```

**5. Land, then close the worker down** — nothing is cleaned up automatically:

```bash
orca orchestration worker-release --dispatch <ctx_id>
orca worktree rm --worktree name:health-ep        # read any refusal before --force
```

**Result:** a feature shipped through the coordinator in one dispatch — the
shape of every Mill run (see ADR 0006).
