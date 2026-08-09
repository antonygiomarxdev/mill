# Mill Architecture

## Design goals

1. **Single binary.** One CLI, not 15 scripts.
2. **Provider agnostic.** CommandCode, OpenCode, Claude — same interface.
3. **Stateful.** Task state persists to disk. Survives crashes and terminal closes.
4. **Event-driven.** Subscribers get notified when tasks change state.
5. **Composable.** Adapters, roles, and workflows are pluggable.

## Adapter interface

Every provider implements:

```typescript
interface Adapter {
  dispatch(worktree: string, prompt: string, model: string): Promise<Session>;
  resume(sessionId: string): Promise<Session>;
  capabilities(): AdapterCapabilities;
}

interface Session {
  id: string;
  status: 'running' | 'done' | 'error';
  subscribe(fn: (event: SessionEvent) => void): () => void;
  wait(): Promise<SessionResult>;
}

interface SessionResult {
  exitCode: number;
  commits: number;
  verdict: 'approved' | 'changes' | 'rejected';
}
```

## Task lifecycle

```
dispatch → produce → review → CHANGES? → rework → review → APPROVED → land
                                ↓
                            max rounds? → REJECTED
```

Each task:
- Gets a git worktree
- Has max N review rounds (default 4)
- Review uses caro model (deepseek-v4-pro)
- Production uses barato model (laguna-free or deepseek-v4-flash)

## State persistence

```
~/.mill/
├── sessions/           # Task sessions (like pi's session format)
│   └── <issue>.jsonl
├── state.json          # Current task states
└── config.json         # Provider config, model preferences
```

State is derived from session files, never from a supervisor process.
If the CLI dies, `mill status` reconstructs state from disk.
