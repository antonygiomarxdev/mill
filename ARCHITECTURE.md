# Mill Architecture

## Design goals

1. **Framework on harness.** Mill is a skill loaded into CTO sessions, not a
   standalone spawner. Uses native `task()` when available, CLI as fallback.
2. **Role-driven.** 11 roles with delegation chains, frontmatter, and agent types.
3. **Phased + gated.** FRD → SPEC → TASKS → IMPLEMENT → REVIEW → DONE.
   Each phase produces an artifact. Mechanical gate scripts block progress.
4. **Stateful.** Task state persists to `.mill/state.json`. Ledger is
   append-only JSONL. Survives crashes and terminal closes.
5. **Async by default.** `mill delegate` returns immediately. Agents run in
   background. `--wait` for sync.

## Architecture layers

```
CTO session (omp / claude / opencode)
  └─ Mill skill (skills/mill.md)
       ├─ Tool detection → copies context to harness-specific paths
       ├─ Role classification → Staff or PM
       ├─ Phase gating → checks/gate-{frd,spec,tasks,coverage,review}
       └─ Delegation → native task() or CLI fallback
```

## Delegation

Two paths, same interface:

| Path | When | Mechanism |
|------|------|-----------|
| Native `task()` | omp harness, agent type available | Harness-managed, async, auto-notify |
| `mill delegate` CLI | Cascade chains, no harness, worktree needed | OS process, goroutine, state/ledger |

## Phased workflow

```
PM           Architect       Tech Lead      Sr Dev        Reviewer
─────        ──────────      ──────────     ───────       ────────
FRD ──────→ SPEC ─────────→ TASKS ──────→ IMPLEMENT ──→ REVIEW → DONE
  │             │              │              │             │
  ▼             ▼              ▼              ▼             ▼
frd.md       spec.md       tasks.md       (commits)    review.md
```

Each phase gated by a script in `checks/`. No artifact → `exit 1` → blocked.

## Adapter

Single implementation: `internal/adapter/commandcode.go`. Interface in
`adapter.go` (Dispatch, Resume, Capabilities). OpenCode deleted per ADR 0001.
The harness handles tool validation natively — `internal/repair/` deleted.

## State persistence

```
.mill/
├── state.json          # Task states (running/done/error)
├── config.json         # Provider/model config
├── role                # Active role (staff|pm)
├── ledger/             # Append-only event log per issue
├── worktrees/          # Git worktrees per delegated task
├── phases/             # Phase artifacts (frd, spec, tasks, review)
├── artifacts/          # Structured handoff between agents
└── memory/             # Context window checkpoints
```

Created at `mill init`. `.mill/` is gitignored. State is derived from files
on disk, never from a supervisor process.

## Model tiers

| Tier | Model | Used by |
|------|-------|---------|
| free | deepseek-v4-flash | Sr Dev (first pass), QA/Docs |
| paid | deepseek-v4-pro | Sr Dev (complex work) |
| pro | deepseek-v4-pro / claude-sonnet | Staff, PM, Architect, Tech Lead, Reviewer |

Role frontmatter `model` field selects tier. `resolveModel()` maps to actual
model name. `free→paid` starts cheap, escalates on complexity.

## Key decisions

- [ADR 0001](docs/adr/0001-mill-as-framework.md) — Framework on harness
- [ADR 0002](docs/adr/0002-budget-enforcement.md) — Budget enforcement
