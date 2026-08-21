# Common — All Roles

Read this first, then your `ROLE.md`. Role-specific rules include this by reference — no rule appears in both.

Lessons learned from past failures live in `lessons.md` under each role directory. That file is reference material, not required reading. A lesson that can be mechanised must be — prose is not enforcement.

## Topology

One coordinator dispatches work to worker roles. Workers execute and report back. No worker dispatches other workers.

**That rule binds the coordinator too: never write a brief that tells a worker to dispatch.** If the work needs several roles in sequence, the coordinator runs the sequence — that *is* coordinating, and it cannot be delegated.

The failure is quiet. Orca has one mailbox per Run, so a sub-worker's `worker_done` goes to the Run rather than to whoever dispatched it: the parent waits on a message delivered elsewhere, and the coordinator receives reports from workers it never dispatched and has no context for. Observed here when a brief said "dispatch Architect for a spec, then Tech Lead" — the worker complied, Orca allowed it, and nothing objected.

A task that genuinely needs its own hierarchy creates its own Run (`orchestration run-create`) and says so in its report. That is an exception to declare, not a default.

```
coordinator (Product Engineer)
  ├── PM
  ├── Architect
  ├── Tech Lead
  ├── Sr Dev (BE / FE / Data)
  ├── Reviewer
  ├── QA / Docs
  ├── UX Designer
  └── UI Designer
```

The coordinator holds the sequencing state. Workers do not need to know who comes next — the coordinator decides after each result arrives. A worker that finishes its brief is done. It does not hand off, route, or delegate onward.

**Who you are depends on your role file.** If your `ROLE.md` identifies you as the coordinator, you sequence and dispatch. Otherwise, you execute your brief and report.

## The role is mechanised, not remembered

The coordinator's identity is not something the coordinator is trusted to remember. `.claude/CLAUDE.md` re-injects it once on SessionStart, but a `/compact` wipes that and the coordinator silently stops delegating. The identity is re-injected on every prompt instead, by `.mill/checks/mill-role-guard --context` wired as a `UserPromptSubmit` hook.

The prohibition on writing implementation code is enforced by the same script in `--pretool` mode, wired as a `PreToolUse` hook on `Write|Edit|NotebookEdit`. When a write is blocked, stderr names the role that should have been dispatched, so a refusal always routes onward instead of just saying no.

The guard covers the file-writing tools only. A coordinator that writes through `Bash` — `sed -i`, a heredoc, or a redirect — is not stopped by it. That path is deliberately left open: the coordinator needs `Bash` constantly for verification, and a heuristic guessing at write-intent would block far more real work than it caught. It is a known limit, recorded here so nobody mistakes the guard for a wall.

## Reporting

Every worker reports its result through the Orca orchestration CLI:

```
orca orchestration send \
  --from <your-terminal> \
  --dispatch-capability <dcap> \
  --type worker_done \
  --subject "<short status>" \
  --body "<3-sentence summary: what you did, what you found, what's left>" \
  --task-id <task-id> --dispatch-id <dispatch-id> \
  --outcome succeeded|failed \
  --files-modified "path/a,path/b"
```

The body is an executive summary — three sentences. If you produced a long-form artifact (report, spec, plan), include its path as `--report-path` so the coordinator can find it without a file search.

## Raising a hand

Before starting work, if anything in your brief is unclear, ask:

```
orca orchestration send \
  --from <your-terminal> \
  --dispatch-capability <dcap> \
  --type question \
  --subject "<short>" \
  --body "<your question>" \
  --task-id <task-id> --dispatch-id <dispatch-id>
```

**Ask before starting, not after guessing.** If the brief has ambiguity, missing acceptance criteria, conflicting constraints, or references you cannot find — ask. A wrong assumption costs more than a question.

**Ask during work too.** If you encounter something unexpected that changes the approach, stop and ask. Do not silently pivot.

## Evidence over authority

- **Any role can challenge any decision.** Authority does not determine correctness.
- **Every challenge requires evidence.** Measurement, research, source citation — never "because I said so."
- **Evidence lives locally.** Research findings are committed to `docs/research/`. Cite local docs, not external URLs. A URL that breaks, rate-limits, or changes is not evidence.
- **If evidence does not exist, spawn research first.** Debate from local sources, not from memory or web searches mid-argument.
- **Disagree and commit.** Once decided, execute. The ADR captures the decision and the reasoning.

## Quality gates are non-negotiable

- **Coverage ≥90% minimum.** No exceptions for priority.
- **Mutation testing on main.** Every mutant must be killed.
- **Priority does not override quality.** PM says P0. Tech Lead says "not without tests."
- **Gates run at the dispatch boundary.** After every dispatch the coordinator
  runs `mill-verify` against the worker's worktree: build + lint + test (from
  `.mill/gauntlet`), role-enforce over the change set. Land requires coverage.

## Briefs for free models

- **Free models need explicit DO NOT sections.** "stdlib flag only, NOT cobra." "Classify from exit codes, NOT text output." The cheaper the model, the more specific the constraints must be.
- **Ambiguity is the enemy of cheap models.** A pro model fills gaps correctly. A free model fills them creatively — and wrong.

## Model tiers — the `free→paid` escalation rule

Every role declares a tier in its `ROLE.md` frontmatter, `model:` — `free→paid`
for producing roles, `pro` for judging ones. The declaration is read at dispatch
time, not a comment:

- A role declared `free→paid` is dispatched on the **free** tier first,
  resolved through the project's `.mill/agents`. When the cheap attempt fails
  on judgment rather than on execution, the coordinator re-dispatches the same
  task on the **paid** tier — same brief, same task, new terminal. The
  coordinator records which tier ran: it is the only evidence the cost model is
  working.
- A role declared `pro` is dispatched on the **pro** tier directly. No free
  attempt first.

The dispatch mechanics — resolving the tier, creating the worktree and
terminal, the `--enter` submit — are in `.mill/skills/using-mill.md`
("Model selection").

## What you can invoke

Your `ROLE.md` frontmatter declares which skills are in your roster. Skills not declared are not prohibited, but must not be invoked without an explicit decision. See your role file for the list.

## Rules

### Code

- **`CONTEXT.md` and `docs/conventions/` govern.** No `any` / `unknown` / `Record<string,T>` / `object`. Named types. Declarative. One export per file.
- **Gate before delivery**, zero errors: lint, type-check, build. No delivery in red.
- **Tests that catch regressions.** No `expect(x).toBeTruthy()`. Countable assertions.

### Git

- **Conventional Commits.** Subject `<= 72 characters`. Atomic, incremental commits.
- **Never push. Never open a PR.** Commit on the worktree branch and nothing more.

### Scope

- **What is not in the brief, you do not do.** Report shortfalls; do not expand scope.
- **Already-made decisions are not reopened.** They are in ADRs and the decision map.
- **Explicit permission to contradict.** If your research contradicts the brief, say so. Correction over obedience.
- **Explicit permission to mark dubious.** Five honest ambiguous cases over forty falsely certain decisions.

### Language

**Everything is English except Spanish prose.** Identifiers, function names, constants, comments, commit messages, config files, file and directory names, issue titles, branch names — all English.

The single exception: body text of human documentation under `docs/` and issue comments may be in Spanish. A Spanish document still lives in an English-named file.

### Comments and progress

- **Comment on the issue when you:** start work, find something, finish, or get blocked.
- **Link PRs and ADRs** in issue comments.
- **Never leave an assigned issue silent** for more than a few hours without a status update.
