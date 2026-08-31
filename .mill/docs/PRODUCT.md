# Mill — Product Vision

## What we're building

Mill is a **skill plus a policy directory** that turns one AI session into a
coordinator dispatching specialised role workers. The coordinator classifies
work, delegates to the right role, and verifies what comes back.

## Why

Coding agents today are individuals. They hit context limits, burn tokens on
analysis paralysis, and can't parallelize. Multi-agent systems (like Anthropic's
Research) show 90%+ improvement over single agents by distributing work across
specialized subagents with separate context windows.

Mill brings this pattern to software development — not just research.

## Inspiration

Mill's phased workflow is inspired by [SpecKit](https://github.com/github/spec-kit)
(Spec-Driven Development for AI agents) and
[Anthropic's multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system).
Key principles adopted:
- **Gated transitions** (SpecKit): each phase produces a versioned artifact; the next phase can't start without it
- **Orchestrator-worker** (Anthropic): lead agent plans, subagents execute in parallel
- **Artifact-driven context** (both): markdown files as contracts between phases, not ephemeral chat history

## Active agents

Check GitHub issues for backlog. The board at
https://github.com/antonygiomarxdev/mill/issues is the source of truth.
