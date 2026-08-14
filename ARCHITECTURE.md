# Mill Architecture

## Design goals

1. **Skill on substrate.** Mill is a skill plus a policy directory loaded into
   CTO sessions. Orca provides the execution substrate.
2. **Role-driven.** 11 roles with delegation chains, frontmatter, and
   capabilities.
3. **Phased + gated.** FRD → SPEC → TASKS → IMPLEMENT → REVIEW → DONE.
   Each phase produces an artifact. Bash gate scripts block progress.
4. **Stateful.** Task state persists to `.mill/state.json`. Ledger is
   append-only JSONL. Survives crashes and terminal closes.

## Star topology

The coordinator is a hub. It dispatches to a role, receives the result, decides
the next step, and dispatches again — one-to-N, not one-to-one-to-one.

```
                          ┌───────────┐
                          │           │
              ┌──────────→│ Architect │
              │           │           │
              │           └───────────┘
              │
┌───────────┐ │           ┌───────────┐
│           │ │           │           │
│ Coordinator├─┼──────────→│  Tech Lead│
│           │ │           │           │
└───────────┘ │           └───────────┘
              │
              │           ┌───────────┐
              │           │           │
              ├──────────→│  Sr Dev   │
              │           │           │
              │           └───────────┘
              │
              │           ┌───────────┐
              │           │           │
              └──────────→│ Reviewer  │
                          │           │
                          └───────────┘
```

The organisational sequence is preserved; no role other than the coordinator
needs to know who comes next. PM becomes a worker role like the others — one
coordinator holds the state and the mailbox.

This dissolves rather than fixes several problems: roles never needing to hand
off to each other (#153), recursion complexity (#109), and fan-out (#154).

## Mill / Orca boundary

**Mill owns** (policy — Markdown and bash):
- the eleven role definitions and their capabilities
- the phase sequence — FRD → spec → tasks → implementation → review
- `role-enforce`: what each role may write
- the phase gates, including acceptance criteria
- brief construction from role definition, issue, and upstream artifact
- **model tier selection per dispatch** — the one substrate capability Orca does
  not provide for non-Claude agents

**Orca owns** (execution substrate):
- spawning and supervising workers
- worktree lifecycle
- the message bus, and the raise-a-hand cycle
- parallelism and its limits

### Where policy lives

```
.mill/
├── roles/          # Role definitions (Markdown + YAML frontmatter)
│   ├── COMMON.md   # Shared rules for all roles
│   └── <role>/
│       ├── ROLE.md     # Role definition, model tier, delegates_to
│       └── lessons.md  # Learned corrections (reference, not required)
├── checks/         # Gate scripts (bash)
│   ├── role-enforce    # Capability enforcement
│   ├── gate-coverage   # Coverage threshold
│   ├── gate-frd        # FRD artifact validation
│   ├── gate-spec       # Spec artifact validation
│   ├── gate-tasks      # Tasks artifact validation
│   ├── gate-handoff    # Delegation completeness
│   └── gate-review     # Review artifact validation
├── map.json        # Role capability map
└── role            # Active role (staff|pm)

checks/             # Top-level git hook gate scripts (bash)
```

All policy is Markdown or bash. No compile step between deciding a rule and
applying it. Enforcement happens in git hooks — `role-enforce` and the phase
gates run at commit time.

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

## How dispatch works

The coordinator (Staff or PM) reads the Mill skill at session start. When it
receives work from the CTO:

1. Classifies the work → selects the appropriate role
2. Constructs a brief from the role definition, issue, and upstream artifact
3. Dispatches via `orca orchestration task-create` + `worker-start`
4. Monitors the worker via `orca orchestration worker-read`
5. Receives results or raised hands via `orca orchestration check`
6. Decides the next step: advance the phase, escalate, or re-delegate

The coordinator does not write code. It orchestrates.

## Model tiers

| Tier | Used by |
|------|---------|
| free | Sr Dev (first pass), QA/Docs |
| paid | Sr Dev (complex work) |
| pro | Staff, PM, Architect, Tech Lead, Reviewer |

Role frontmatter `model` field selects tier. The coordinator passes the
appropriate `--model` flag when dispatching an Orca worker.

## Key decisions

- [ADR 0001](docs/adr/0001-mill-as-framework.md) — Framework on harness
- [ADR 0002](docs/adr/0002-budget-enforcement.md) — Budget enforcement
- [ADR 0005](docs/adr/0005-orca-as-execution-substrate.md) — Orca as the
  execution substrate
- [ADR 0006](docs/adr/0006-mill-is-a-skill-not-a-binary.md) — Mill is a skill,
  not a binary
