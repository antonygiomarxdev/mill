# Mill

A skill plus a policy directory that turns an AI agent into a Staff Engineer or
Product Manager. Mill defines the roles, the phase sequence, and the dispatch
procedure. Orca provides the execution substrate.

## What Mill is

Mill is **not a binary**. It is a set of Markdown role definitions, bash gate
scripts, and a skill file that an agent reads at session start. When loaded, the
agent becomes one of two roles:

- **[Mill · Staff]** — for technical work: delegates to Architect → Tech Lead →
  Sr Dev. Verifies results, declares merge-readiness.
- **[Mill · PM]** — for product work: delegates to UX → UI → QA/Docs. Writes
  specs, manages priorities.

You speak naturally. Mill handles the rest.

See [docs/PRODUCT.md](docs/PRODUCT.md) for the full product definition and
[ADR 0006](docs/adr/0006-mill-is-a-skill-not-a-binary.md) for why the Go CLI
was retired.

## Prerequisites

- **Orca** — the execution substrate. Handles worker spawning, supervision,
  worktree isolation, and the message bus. Install from
  [orca.sh](https://orca.sh).
- **A configured CLI agent** — `command-code`, Claude, or another agent Orca
  supports. The agent must be registered with Orca before it can be dispatched
  (`command-code` works out of the box; other agents need explicit registration).
- **Git** — for worktree isolation and gate enforcement.

## Install

Copy the Mill skill and policy directory into your project:

```
your-project/
├── .mill/
│   ├── roles/          # 11 role definitions (Markdown)
│   ├── checks/         # Gate scripts (bash)
│   ├── map.json        # Role capability map
│   └── role            # Active role file
├── checks/             # Top-level gate scripts
└── .omp/
    └── AGENTS.md       # Agent context (loads the Mill skill)
```

Then open the project in your harness. The agent discovers `AGENTS.md` and loads
the Mill skill automatically.

**You're done.** Start speaking naturally to your agent. It becomes Mill Staff
or Mill PM and begins delegating.

## How it works

```
CTO session (Orca + command-code / claude / etc.)
  └─ Mill skill ← loaded at session start
       ├─ Classifies user message → Staff or PM
       ├─ Delegates via orca orchestration dispatch
       └─ Orchestrates chain: Architect → Tech Lead → Sr Dev → QA
```

### Star topology

One coordinator dispatches to role workers — one-to-N, not a linear chain.
The coordinator holds the state and the mailbox; PM becomes a worker role like
the others. The organisational sequence is preserved: the coordinator decides
who comes next, not the roles themselves.

### Delegation chain

```
CTO → Staff → Architect → Tech Lead → Sr Dev (BE/FE/Data)
CTO → Staff → Reviewer → QA/Docs
CTO → Staff → PM
CTO → PM → UX Designer → UI Designer → QA/Docs
```

Each role has a `delegates_to` list in its frontmatter. Chain validation is
mechanical — you can't delegate outside your authorized targets.

### Phased workflow

```
PM           Architect       Tech Lead      Sr Dev        Reviewer
─────        ──────────      ──────────     ───────       ────────
FRD ──────→ SPEC ─────────→ TASKS ──────→ IMPLEMENT ──→ REVIEW → DONE
  │             │              │              │             │
  ▼             ▼              ▼              ▼             ▼
frd.md       spec.md       tasks.md       (commits)    review.md
```

Each phase is gated by a script in `checks/`. No artifact → blocked.

### Model tiers

| Tier | Used by |
|------|---------|
| free | Sr Dev (first pass), QA/Docs |
| paid | Sr Dev (complex work) |
| pro | Staff, PM, Architect, Tech Lead, Reviewer |

Role frontmatter `model` field selects tier. The coordinator passes the
appropriate `--model` flag when dispatching an Orca worker.

## Project structure

```
.mill/
├── roles/              # Role definitions with YAML frontmatter
│   ├── COMMON.md       # Shared rules for all roles
│   ├── staff/ROLE.md   # Staff: orchestrator, never writes code
│   ├── pm/ROLE.md      # PM: product specs, design delegation
│   ├── architect/      # Architect: system design, ADRs
│   ├── tech-lead/      # Tech Lead: code review, decomposition
│   ├── reviewer/       # Reviewer: spec compliance, verdict
│   ├── sr-dev-be/      # Sr Dev Backend: implementation
│   ├── sr-dev-fe/      # Sr Dev Frontend
│   ├── sr-dev-data/    # Sr Dev Data
│   ├── qa-docs/        # QA/Docs: tests, changelogs, docs
│   ├── ux-designer/    # UX: flows, wireframes
│   └── ui-designer/    # UI: components, design tokens
├── checks/             # Gate validation scripts (bash)
├── map.json            # Role capability map
└── role                # Active role (staff|pm)

checks/                 # Top-level gate scripts (bash)
docs/
├── PRODUCT.md          # Product definition
├── FINDINGS-2026-08.md # Defect patterns from live testing
└── adr/                # Architecture Decision Records
mill.yml                # Project config (targets, gates)
```

## Configuration

`mill.yml`:

```yaml
name: my-project

targets:
  develop:
    gates: [lint, type-check, build]
```

## Architecture Decision Records

- [ADR 0005](docs/adr/0005-orca-as-execution-substrate.md) — Orca as the
  execution substrate
- [ADR 0006](docs/adr/0006-mill-is-a-skill-not-a-binary.md) — Mill is a skill,
  not a binary
