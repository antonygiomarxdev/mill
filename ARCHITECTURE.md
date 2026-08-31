# Mill Architecture

Mill is a **skill plus a policy directory** (ADR 0006): role definitions in
Markdown, enforcement in bash, no binary. Orca provides the execution
substrate (ADR 0005). The coordinator is the **Product Engineer**, the single
head in a star topology.

## Design goals

1. **Skill on substrate.** Mill carries the policy — roles, the phase
   sequence, brief construction, the dispatch procedure — as Markdown and
   bash. Orca spawns and supervises the workers.
2. **Role-driven.** Twelve role briefings under `.mill/roles/` (`COMMON.md`
   plus one `ROLE.md` per role), each declaring its model tier and the file
   categories it may write.
3. **Single coordinator.** One head — the Product Engineer — dispatches,
   receives, and decides the next step. No worker dispatches another worker.
4. **Verified at the dispatch boundary.** Verification is not one gate script
   per phase; it is one command — `.mill/checks/mill-verify` — that the
   coordinator runs against a worker's worktree after it reports done
   (ADR 0009).

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
needs to know who comes next. PM is a worker role like the others — one
coordinator holds the state and the mailbox.

This dissolves rather than fixes several problems: roles never needing to hand
off to each other (#153), recursion complexity (#109), and fan-out (#154).

## Mill / Orca boundary

**Mill owns** (policy — Markdown and bash):

- the twelve role definitions and their capabilities
- the phase sequence — FRD → spec → tasks → implementation → review
- `role-enforce`: what each role may write — role categories resolved to file
  patterns through `.mill/role-capabilities`
- brief construction from role definition, issue, and upstream artifact
- model tier selection per dispatch — the one substrate capability Orca does
  not provide for non-Claude agents

**Orca owns** (execution substrate):

- spawning and supervising workers
- worktree lifecycle
- the message bus, and the raise-a-hand cycle
- parallelism and its limits

### Where policy lives

```
.mill/
├── roles/                   # Role definitions (Markdown + YAML frontmatter)
│   ├── COMMON.md            # Shared rules for all roles
│   └── <role>/ROLE.md       # Role definition, model tier, reviewed_by
├── checks/                  # Gate scripts (bash): role-enforce, mill-verify,
│                            # mill-preflight, mill-role-guard, pre-commit,
│                            # pre-push, common.sh
├── gauntlet                 # Per-project build/lint/test commands
└── role-capabilities.example  # Category → file-pattern map (template)
```

All policy is Markdown or bash. No compile step between deciding a rule and
applying it. `.mill/role-capabilities` and `.mill/agents` are per-project and
gitignored; each ships a tracked `.example` template. `.mill/gauntlet` is
per-project too, tracked here as this repository's own.

Enforcement reads that policy in two places, neither of which a worker can
skip: the git hooks (`pre-commit` runs `role-enforce` plus the build/lint
steps; `pre-push` runs the test/coverage steps), and `mill-verify`, the
command the coordinator runs against a worker's worktree at the dispatch
boundary.

## Phased workflow

```
PM           Architect       Tech Lead      Sr Dev        Reviewer
─────        ──────────      ──────────     ───────       ────────
FRD ──────→ SPEC ─────────→ TASKS ──────→ IMPLEMENT ──→ REVIEW → DONE
  │             │              │              │             │
  ▼             ▼              ▼              ▼             ▼
frd.md       spec.md       tasks.md       (commits)    review.md
```

Each phase still produces an artifact (`frd.md`, `spec.md`, `tasks.md`, the
commits, `review.md`), but there is no per-phase gate script. Progress is
enforced at the dispatch boundary: the coordinator verifies each worker's
worktree with `mill-verify` before dispatching the next role in the sequence.

## How dispatch works

The coordinator — the Product Engineer — reads the delegate skill at session
start. When it receives work from the CTO:

1. Classifies the work → selects the appropriate role
2. Constructs a brief from the role definition, issue, and upstream artifact
3. Dispatches via `orca orchestration task-create` + `worker-start`
4. Monitors the worker via `orca orchestration worker-read`
5. Receives results or raised hands via `orca orchestration check`
6. Verifies the worker's worktree with `mill-verify`, then decides the next
   step: advance the phase, escalate, or re-delegate

The coordinator does not write code. It orchestrates.

## Model tiers

Each role declares a tier in its `ROLE.md` frontmatter (`model:`):

| Declared | Dispatched on | Roles |
|---|---|---|
| `pro` | pro tier, directly | Product Engineer, PM, Architect, Tech Lead, Reviewer, UX, UI, Policy Author |
| `free→paid` | free tier first; escalated to paid when the free attempt fails on judgement | Sr Dev BE/FE/Data, QA/Docs |

`.mill/agents` is a catalog of what runs on this machine (gitignored;
`.mill/agents.example` is the template). No script reads it; every dispatch
names its agent and model. See `.claude/skills/delegate/SKILL.md` section 2.

## Key decisions

- [ADR 0005](docs/adr/0005-orca-as-execution-substrate.md) — Orca as the
  execution substrate
- [ADR 0006](docs/adr/0006-mill-is-a-skill-not-a-binary.md) — Mill is a skill,
  not a binary
- [ADR 0007](docs/adr/0007-role-capabilities-by-category.md) — role
  capabilities by category
- [ADR 0009](docs/adr/0009-gauntlet-at-the-dispatch-boundary.md) — verification
  at the dispatch boundary
- [ADR 0011](docs/adr/0011-versioned-copy-install.md) — Mill installs as a
  versioned copy
