# Mill

Multi-agent delegation framework. Mill turns your AI agent into a Staff
Engineer or Product Manager that autonomously classifies work, delegates to
specialized subagents, orchestrates review chains, and persists state.

## What Mill does

When you open a Mill-managed project in your harness (omp, claude code,
opencode), your agent loads the Mill skill and becomes:

- **[Mill · Staff]** — for technical work: delegates to Architect → Tech
  Lead → Sr Dev. Verifies results, declares merge-readiness.
- **[Mill · PM]** — for product work: delegates to UX → UI → QA/Docs.
  Writes specs, manages priorities.

You speak naturally. Mill handles the rest.

## Quick start

### Prerequisites

- **Go 1.21+** — [install](https://go.dev/dl/) or `brew install go`
- **Git** — for worktree isolation
- **A harness** — omp, claude code, opencode, or GitHub Copilot

### Install

```bash
go install github.com/antonygiomarxdev/mill/cmd/mill@latest
```

If Go is not available, download the binary from
[releases](https://github.com/antonygiomarxdev/mill/releases).

### Initialize a project

```bash
mill init
```

This scaffolds the full project: roles, skills, checks, gates, and context
files. Then open the project in your harness. The agent discovers
`.omp/AGENTS.md` and loads the Mill skill automatically.

**You're done.** Start speaking naturally to your agent. It becomes
[Mill · Staff] or [Mill · PM] and begins delegating.
### For an existing Mill project

Just open the project in your harness. The agent discovers `.omp/AGENTS.md`
and loads the skill automatically. No commands needed.

### Manual CLI (without a harness)

```bash
mill delegate 42 --role sr-dev-be    # async: returns immediately
mill delegate 42 --role sr-dev-be --wait  # sync: blocks until done
mill status                          # show all tasks
mill role get                        # show active role
mill role set pm                     # switch to PM
```

## How it works

```
CTO session (omp / claude / opencode)
  └─ [Mill · Staff]  ← skill loaded at session start
       │
       ├─ Classifies user message → Staff or PM
       ├─ Detects harness → copies context to correct locations
       ├─ Delegates via native task() or CLI fallback
       └─ Orchestrates chain: Architect → Tech Lead → Sr Dev → QA
```

### Delegation chain

```
CTO → Staff → Architect → Tech Lead → Sr Dev (BE/FE/Data)
CTO → Staff → Reviewer → QA/Docs
CTO → Staff → PM
CTO → PM → UX Designer → UI Designer → QA/Docs
```

Each role has a `delegates_to` list in its frontmatter. Chain validation
is mechanical — you can't delegate outside your authorized targets.

### Agent types per role

Roles declare their agent type in `roles/<role>/ROLE.md` frontmatter:

```yaml
agent: task              # full capabilities (default)
agent: scout             # read-only: docs, verification
agent: cavecrew-reviewer # code review, one-line findings
agent: cavecrew-builder  # surgical 1-2 file edits
```

### Async by default

`mill delegate` returns immediately. Agents run in background. Check
progress with `mill status`. Use `--wait` when you need the result
synchronously.

### Blocked workflow

When a subagent can't proceed (ambiguous requirements, missing info):

1. Agent comments on the GitHub issue describing the blocker
2. Delegator resolves the ambiguity
3. Delegator re-spawns the agent with amplified context

The issue is the handoff surface. No DMs, no polling.

## Project structure

```
mill.yml         — project config (models, targets, budget)
roles/            — role definitions with YAML frontmatter
  COMMON.md       — shared rules for all roles
  staff/ROLE.md   — Staff: orchestrator, never writes code
  pm/ROLE.md      — PM: product specs, design delegation
  architect/      — Architect: system design, ADRs
  tech-lead/      — Tech Lead: code review, decomposition
  reviewer/       — Reviewer: spec compliance, verdict
  sr-dev-be/      — Sr Dev Backend: implementation
  sr-dev-fe/      — Sr Dev Frontend
  sr-dev-data/    — Sr Dev Data
  qa-docs/        — QA/Docs: tests, changelogs, docs
  ux-designer/    — UX: flows, wireframes
  ui-designer/    — UI: components, design tokens
checks/           — git hooks (pre-commit, pre-push, role-enforce)
skills/           — agent skills (mill.md is the framework entry point)
docs/adr/         — Architecture Decision Records
.mill/            — runtime state (role, state.json, ledger/, worktrees/)
```

## Configuration

`mill.yml`:

```yaml
name: my-project
models:
  free:
    - provider: commandcode
      model: deepseek-v4-flash
  paid:
    - provider: commandcode
      model: deepseek-v4-pro
  pro:
    - provider: commandcode
      model: claude-sonnet-4-20250514

targets:
  develop:
    budget:
      time_seconds: 300
      max_turns: 50
    gates: [lint, type-check, build]
```

## Architecture Decision Records

- [ADR 0001](docs/adr/0001-mill-as-framework.md) — Mill as framework on harness
- [ADR 0002](docs/adr/0002-budget-enforcement.md) — Budget enforcement design
