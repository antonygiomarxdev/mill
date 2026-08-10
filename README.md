# Mill

Agent delegation harness. Mill dispatches AI agents to work on GitHub issues,
drives them through a review loop, and lands the result — all from a single
Go binary. Like a foreman on a ranch, it routes tasks to the right worker and
tracks progress from task dispatch to merge.

Instead of a dozen bash scripts glued together, Mill is one CLI:

```bash
mill delegate 390    # dispatch issue #390 to an AI agent
mill status          # show all running/done tasks
mill land main       # run gates and checkout target branch
```

Born from the agent delegation workflow described in `docs/lessons.md`.

## Quick install

Mill is a single static Go binary. Build it from the repository root:

```bash
go build -o mill ./cmd/mill
```

Put the resulting `mill` binary on your `PATH` to run it from anywhere.

## Initialize a project

`mill init` scaffolds a mill project: it generates `mill.yml` and copies
starter files (roles, checks, skills, docs) from the bundled templates.

```bash
mill init              # interactive prompts with defaults
mill init -yes         # non-interactive, use all defaults
mill init -name myapp -provider commandcode -model laguna-free
```

Flags:

| Flag          | Default         | Description                          |
|---------------|-----------------|--------------------------------------|
| `-name`       | current dir     | Project name written to `mill.yml`   |
| `-provider`   | `commandcode`   | AI provider (commandcode\|opencode\|claude) |
| `-model`      | `laguna-free`   | Provider model identifier            |
| `-max-rounds` | `4`             | Max review rounds before REJECTED    |
| `-yes`        | `false`         | Skip prompts, use defaults/flags     |
| `-target`     | `.`             | Target directory for scaffolding    |

## Delegate work

`mill delegate <issue>` dispatches an AI agent to implement the given GitHub
issue number. It creates a git worktree, starts an agent session, waits for it
to finish, classifies the outcome, and persists state plus an append-only
ledger entry.

```bash
mill delegate 390
mill delegate 390 -model claude-sonnet-5 -max-turns 200
```

Flags:

| Flag           | Default | Description                              |
|----------------|---------|------------------------------------------|
| `-model`       | config  | Model to use (overrides `mill.yml`)     |
| `-max-turns`   | `100`   | Maximum conversation turns for the agent |

The agent runs in an isolated worktree under `.mill/worktrees/` and writes
output via the configured provider adapter. On completion Mill prints a
verdict (`approved` / `changes` / `rejected`) and commit count.

## Check status

`mill status` loads persisted state from `.mill/state.json` and prints a
table of all tasks:

```
ID        ISSUE  STATUS  COMMITS  VERDICT
task-390  390    done    3        approved
task-392  392    running 0
```

State is reconstructed from disk every time, so it survives crashes and
terminal closes. `mill status` always exits 0, even when no tasks exist.

## Land changes

`mill land <target> [gates...]` runs gate commands in a worktree and checks
out the target branch. Gates are shell commands executed in order; if any
gate fails, landing is aborted.

```bash
mill land main ./checks/pre-push ./checks/pre-commit
mill land main -confirm ./checks/pre-push
```

Flags:

| Flag       | Default | Description                        |
|------------|---------|------------------------------------|
| `-worktree`| auto    | Worktree directory to land from   |
| `-confirm` | `false` | Prompt before merging to target   |

## Roles

Agent behavior is defined as configuration, not hardcoded. Each role is a
markdown file under `roles/`:

- `roles/COMMON.md` — principles and communication rules shared across all
  roles.
- `roles/<role>/ROLE.md` — specialization-specific instructions (e.g.
  `sr-dev-be` for a senior backend developer).

After `mill init` a default role (`sr-dev-be`) is scaffolded. The `delegate`
command passes the role instructions to the agent along with the issue
prompt, so agents follow consistent conventions on every run.

## Pipeline stages

A task moves through these stages (see `ARCHITECTURE.md` for the full model):

```
dispatch  ->  produce  ->  review  ->  changes?  ->  rework  ->  review
                                               |
                                  max rounds? -> REJECTED
                                               |
                              APPROVED  ->  land
```

1. **dispatch** — agent starts in a fresh worktree on the issue's branch.
2. **produce** — agent implements the feature and commits changes.
3. **review** — outcome is classified. `OK` or `MAX_TURNS` marks the task done;
   other classifications trigger retry or abort.
4. **rework** — if the verdict is `changes`, the agent is re-prompted (up to
   `max-rounds`, default 4).
5. **land** — once `approved`, gate checks run and the worktree is merged to
   the target branch.

Classifications that drive retry/abort:

| Classification   | Behavior                          |
|------------------|-----------------------------------|
| `OK`             | Done, proceed to review            |
| `MAX_TURNS`      | Done, proceeding with what exists  |
| `RATE_LIMITED`   | Backoff and retry                  |
| `TRANSIENT`      | Backoff and retry                   |
| `FATAL`          | Retry (up to 3 attempts)           |
| `AUTH`           | Abort — fix credentials          |
| `NO_CREDIT`      | Abort — insufficient credits     |
| `BLOCKED`        | Persist and stop                   |

Review rounds use the *caro* model (`deepseek-v4-pro`); production dispatch
uses the *barato* model (`laguna-free` or `deepseek-v4-flash`).

## Example workflow

```bash
# 1. Build and initialize a project
go build -o mill ./cmd/mill
mill init -yes

# 2. Delegate an issue to an agent
mill delegate 390

# 3. Check progress
mill status

# 4. Land after approval (runs gates, then merges to main)
mill land main -confirm ./checks/pre-push
```

## Project layout

```
.
├── cmd/mill/                 # Go entry point (build with `go build`)
├── internal/
│   ├── cli/                  # mill subcommands (init, delegate, status, land)
│   ├── adapter/              # Provider adapters (CommandCode, OpenCode, Claude)
│   ├── classify/             # Session outcome classification
│   ├── config/               # mill.yml / config.json loading
│   ├── domain/               # Core types (Task, Session, Verdict, Classification)
│   ├── issue/                # Issue number parsing
│   ├── ledger/               # Append-only event log
│   ├── repair/               # Tool-call input repair pipeline
│   └── state/                # Persistent task state
├── checks/                   # Git hooks (pre-commit, pre-push)
├── docs/
│   ├── lessons.md            # Lessons from RUMAI
│   └── plans/                # Implementation plans
├── ARCHITECTURE.md           # Design goals and interfaces
└── go.mod
```

## Adapters

Mill adapts to different AI providers through an adapter layer. Each adapter
implements `Dispatch`, `Resume`, and `Capabilities`:

| Adapter    | Type         | Use case                     |
|------------|--------------|------------------------------|
| CommandCode| CLI headless | Cheap models via `cmd -p`    |
| OpenCode   | Provider direct | Direct model access        |
| Claude     | Anthropic API | Staff-level reasoning       |

Configuration lives in `.mill/config.json`:

```json
{
  "provider": "commandcode",
  "model": "laguna-free",
  "max_rounds": 4
}
```

## Principles

1. **State persists.** Sessions survive crashes. State is derived from
   artifacts on disk, never from a supervisor process.
2. **Event-driven.** The ledger records every transition; nothing polls.
3. **Provider agnostic.** Same interface, different backends.
4. **Roles as config.** Agent behavior is defined in `roles/`, not hardcoded.
# test
# test
