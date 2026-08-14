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
- Free models need explicit DO NOT sections. The cheaper the model, the more specific the constraints.

### 5. Dispatch

```bash
# Create the task
orca orchestration task-create --subject "<task name>"

# Start the worker
orca orchestration worker-start \
  --agent <agent-id> \
  --worktree <worktree-path> \
  --model <model-tier> \
  --timeout-ms <timeout>

# Inject the brief
orca terminal send --text "<brief>" --enter

# Monitor output
orca orchestration worker-read

# Check for questions
orca orchestration check --terminal <terminal-id>
```

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

## Model selection

Use the least powerful model that can handle each role:

| Role | Default tier |
|------|-------------|
| PM, Architect, Tech Lead | pro |
| Sr Dev (BE/FE/Data) | free→paid |
| Reviewer | pro |
| QA/Docs | free→paid |
| UX Designer, UI Designer | pro |

Escalate when a free model fails on a task that needs judgment. The coordinator decides per-dispatch.

## Worktree lifecycle

- One worktree per dispatched task
- When PR merges: `git worktree remove <path>` and `git branch -D agent/<issue>`
- Never leave orphan worktrees
- Verify with `git worktree list` before and after cleanup

## Ledger

Track every dispatch in the ledger (`.mill/ledger/<issue>.jsonl`):
- Role dispatched, model used, task ID
- Outcome: succeeded, failed, blocked
- Gate results
- Any questions and answers

The ledger survives compaction. Trust it over your own recollection.
