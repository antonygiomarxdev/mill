# Mill

Agent harness for delegating work to specialized AI agents. Like a foreman on a ranch, it routes tasks to the right worker and tracks progress.

## What it replaces

Instead of 15 bash scripts glued together, Mill is a single CLI:

```bash
mill delegate 390    # dispatch issue to correct agent
mill status          # show all running/pending tasks
mill review 392      # trigger code review
mill land 388        # merge with gates
```

## Providers

Mill adapts to different AI providers through an adapter layer:

| Adapter | Type | Use case |
|---------|------|----------|
| CommandCode | CLI headless | Cheap models via `cmd -p` |
| OpenCode | Provider direct | Direct model access |
| Claude | Anthropic API | Staff-level reasoning |

## Architecture

```
mill/
├── adapters/           # Provider-specific adapters
│   ├── commandcode.ts
│   ├── opencode.ts
│   └── claude.ts
├── core/
│   ├── session.ts      # Task lifecycle
│   ├── dispatch.ts     # Route to correct agent
│   └── state.ts        # Persistence
├── roles/              # Agent role definitions
│   ├── common.md
│   ├── staff.md
│   ├── senior-dev.md
│   ├── designer.md
│   └── researcher.md
└── cli.ts              # Entry point
```

## Principles

1. **State persists.** Sessions survive crashes. Like `~/.pi/agent/sessions/`.
2. **Event-driven.** Subscribe to changes. Never poll.
3. **Provider agnostic.** Same interface, different backends.
4. **Roles as config.** Agent behavior defined in `roles/`, not hardcoded.

## Related

Born from [RUMAI](https://github.com/rumai-labs/rumai)'s agent delegation workflow. See `docs/lessons.md` for what we learned building it.
