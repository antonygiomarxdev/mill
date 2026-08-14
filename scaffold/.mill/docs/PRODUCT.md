# Mill — Product Vision

## What we're building

Mill is a **multi-agent delegation framework** that turns any AI coding agent
into a Staff Engineer or Product Manager. The agent autonomously classifies
work, delegates to specialized subagents, orchestrates review chains, and
persists state.

## Why

Coding agents today are individuals. They hit context limits, burn tokens on
analysis paralysis, and can't parallelize. Multi-agent systems (like Anthropic's
Research) show 90%+ improvement over single agents by distributing work across
specialized subagents with separate context windows.

Mill brings this pattern to software development — not just research.

## Current state (MVP)

**Done:**
- Framework skill (`skills/mill.md`) — loads in omp sessions
- Role system — 11 roles with delegation chains and frontmatter
- CLI (`mill delegate`, `mill status`, `mill init`, `mill role`)
- Async delegation with retry/backoff
- Ledger + state persistence
- Budget enforcement (time + token limits)
- Bootstrap: `mill init` scaffolds full project

**In progress:**
- CLI coverage ≥90% (#41)
- PM specs for delegation chain and capability enforcement (#42, #46)

**Next:**
- Context window management for long chains
- Artifact system (structured handoff between agents)
- Token cost tracking
- Multi-provider support (Claude adapter)

## Architecture

```
CTO → [Mill · Staff] → Architect → Tech Lead → Sr Dev → QA
CTO → [Mill · PM] → UX → UI → QA
```

Orchestrator-worker pattern. Staff/PM is the lead agent. Roles are specialized
subagents spawned via native `task()` or CLI fallback.

## Key decisions

- [ADR 0001](adr/0001-mill-as-framework.md) — Framework on harness, not adapter-based
- [ADR 0002](adr/0002-budget-enforcement.md) — Budget enforcement in adapter

## How to use

```bash
go install github.com/antonygiomarxdev/mill/cmd/mill@latest
mill init
```

Open in omp. Agent auto-loads Mill skill. Start delegating.

## Active agents

Check `mill status` for running tasks. Check GitHub issues for backlog.
The board at https://github.com/antonygiomarxdev/mill/issues is the source of truth.
