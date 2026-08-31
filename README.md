# Mill

An org chart that executes. Mill is a **skill plus a policy directory** — role
definitions in Markdown, gate scripts in bash — that turns one AI session into a
coordinator dispatching specialised workers.

Mill defines *who does what, in what order, and what "done" means*.
[Orca](https://onorca.dev) runs the workers.

There is no binary. See [ADR 0006](docs/adr/0006-mill-is-a-skill-not-a-binary.md)
for why the Go CLI was retired, and [docs/PRODUCT.md](docs/PRODUCT.md) for the
full product definition.

## How it works

You talk to one session. That session is the **coordinator**. It reads the issue,
picks the role that should do the work next, builds a brief from that role's
`ROLE.md`, dispatches a worker through Orca, verifies what comes back against the
phase gates, and decides what happens next.

```
                    ┌─ PM            FRD, priorities
                    ├─ Architect     ADRs, specs
   you ─→ coordinator ─┼─ Tech Lead     task decomposition
                    ├─ Sr Dev        implementation
                    ├─ Reviewer      verdict
                    └─ QA/Docs, UX, UI
```

**The topology is a star, not a chain.** The coordinator dispatches one-to-N and
holds the sequence. No worker dispatches another worker — which is why workers
need to know nothing about the org chart.

The organisational sequence is preserved:

```
intent → FRD → spec(s) → tasks → implementation → review
          PM    Architect  Tech Lead   Sr Dev      Reviewer
```

Each phase is gated by a script in `.mill/checks/`. No artifact, or an artifact
missing its required sections, and the phase does not pass.

**Blocking is a first-class outcome.** A worker that finds the brief
underspecified does not guess — it says what is missing and stops. The
coordinator answers or escalates to you.

## Prerequisites

- **[Orca](https://onorca.dev)** — the execution substrate: worker spawning,
  supervision, worktree isolation, and the message bus.
- **At least one CLI agent registered with Orca.** Registration is per machine
  and happens once: a login or an API key. Orca ships hooks for Claude,
  Command Code, Codex, Copilot, Cursor, Gemini, Grok and others.
- **Git.**

## Install

Installation is being redesigned for multiple harnesses (Claude Code, Codex, Pi and others). The previous installer — a scaffold `mill-install` copied into a new project — shipped the version of Mill that commit `6166ada` retired and has been removed.

## Usage

A worked example — adding a small backend feature.

**1. Build a brief** from the role's `ROLE.md` and the acceptance criteria. Keep
it short; reference files rather than inlining them.

```markdown
# Task: add a health endpoint to the API

You are acting as **sr-dev-be**. Read your role first:
`.mill/roles/sr-dev-be/ROLE.md`

## Context
The API has no health endpoint. The acceptance criteria in COMMON.md apply:
gates must pass before delivery.

## Acceptance criteria
1. `pnpm test -- src/health` passes
2. `pnpm lint && pnpm type-check` are clean
3. No file outside `src/` is modified

## Before you begin
If anything above is unclear, ask now:
`orca orchestration send --to run:RUN --subject "<short>" --body "<q>" --type question`

## When done
`orca orchestration send --to run:RUN --subject "Done" --body "<real output>" --outcome succeeded`
```

**2. Make sure Orca is up**, before dispatching rather than after a failure:

```bash
orca status | grep -q "runtimeReachable: true" || orca open
```

**3. Dispatch.**

```bash
RUN=$(orca orchestration run-create --objective "coverage" --json | grep -oE 'run_[a-z0-9]+' | head -1)

TASK=$(orca orchestration task-create --run $RUN \
  --task-title "Raise ledger coverage" \
  --spec "$(cat brief.md)" --json | grep -oE 'task_[a-z0-9]+' | head -1)

orca orchestration worker-start --run $RUN --task $TASK \
  --agent command-code \
  --worktree new-child --name ledger-cov --repo path:$(pwd)
```

**4. Watch it work** — reasoning, tool calls and all:

```bash
orca orchestration worker-read --dispatch <ctx_id>
```

**5. Read what it reports.** Workers report through the mailbox with a
structured payload — `outcome`, `filesModified`, `taskId`:

```bash
orca orchestration inbox --limit 5 --full
```

If a worker raised a hand instead, answer it:

```bash
orca orchestration reply --id <msg_id> --body "<your answer>"
```

**6. Verify it yourself.** A report is not evidence. Run the acceptance commands
in the worker's worktree, then the phase gates:

**7. Land it, then close the worker down.** Nothing is cleaned up
automatically, and that is deliberate — a finished worker's terminal and
worktree may hold something you have not looked at yet.

```bash
orca orchestration worker-release --dispatch <ctx_id>
orca worktree rm --worktree name:ledger-cov
```

`worktree rm` refuses when there are uncommitted changes and names the files.
**Read them before passing `--force`.**

## Things measured in practice

- `--agent` must name an agent Orca has configured. `command-code` works;
  `commandcode` and `cmd` are rejected with *"A configured --agent is required"*.
- **The brief is injected but not always submitted.** With `--agent claude` it
  lands as an unsubmitted draft and the worker never starts — reproduced on Orca
  v1.4.183, upstream issue #14505. Confirm after dispatch and submit if needed:
  `orca terminal send --terminal <handle> --enter`.
- `--model` accepts Claude, Codex and Cursor model identifiers only. For other
  agents the model is whatever that agent's own configuration selects, so
  per-dispatch tier selection is not available for them.

## Limits

- **Mill needs a session to drive it.** The coordinator is you plus an agent.
  There is no unattended mode: no cron, no CI without a session.
- **Orca must be running.** A dispatch against a stopped runtime fails partway,
  sometimes after the task has already been created.
- **Nothing forces the coordinator to follow the sequence.** The gates enforce
  what may be committed — the part that matters — but the procedure itself is
  instructions, not a program.

## Project structure

```
.mill/
├── roles/              # 12 role definitions + COMMON.md, with YAML frontmatter
├── checks/             # gate scripts (bash) — incl. mill-verify at the dispatch boundary
├── skills/
│   ├── wayfinder.md    # the synced skill (Product Engineer / PM)
│   ├── README.md
│   └── skills.json     # the manifest
└── docs/

docs/
├── PRODUCT.md          # the product definition
├── FINDINGS-2026-08.md # failure patterns found running Mill against itself
├── adr/                # architecture decisions
└── plans/
```

Every role's `ROLE.md` declares what it produces, its acceptance criteria, its
`allowed_files` capability categories, and how to report.
`.mill/checks/role-enforce` resolves those categories to file patterns through
`.mill/role-capabilities` — the one per-project file that names your languages —
so adding a role requires no code change, and changing language is one line,
not a diff across every role.

## Architecture Decision Records

- [ADR 0005](docs/adr/0005-orca-as-execution-substrate.md) — Orca as the
  execution substrate
- [ADR 0006](docs/adr/0006-mill-is-a-skill-not-a-binary.md) — Mill is a skill,
  not a binary
- [ADR 0007](docs/adr/0007-role-capabilities-by-category.md) — role
  capabilities are declared by category, not file extension
- [ADR 0008](docs/adr/0008-non-destructive-install.md) — install detects what
  is there and never destroys it
- [ADR 0009](docs/adr/0009-gauntlet-at-the-dispatch-boundary.md) — the gauntlet
  runs at the dispatch boundary, not from git hooks
