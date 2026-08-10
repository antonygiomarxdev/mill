# ADR 0002: Budget Enforcement

**Date:** 2026-08-10
**Status:** accepted
**Deciders:** Architect, Tech Lead, CTO

## Context

Mill delegates work to AI agents via subprocesses. Without budget limits, an
agent can burn unlimited time and tokens on a task that should take three
minutes. Two incidents make this concrete:

- **Issue #22:** 12 minutes (budget 5 min) — stuck in a commit message loop,
  no useful output after the first commit.
- **Issue #32:** 6 minutes (budget 3 min) — analysis paralysis, zero file
  writes, repeated reconsideration of the same design decision.

The runner must **enforce**, not suggest. Budget exceeded means kill the
subprocess and flag the delegation as BLOCKED so Staff can intervene.

The design must also detect **analysis paralysis** — an agent that produces
thinking tokens without ever writing a file — and treat it as a budget
violation.

## Decision

**Runner sets policy, Adapter enforces, Domain classifies.**

The enforcement lives inside the adapter because the adapter owns the
subprocess lifecycle and the NDJSON stream. The runner reads budget config
and passes it to the adapter. The domain classifies the result as BLOCKED.

### Enforcement mechanisms

| Mechanism | Detection | Kill | Exit code |
|---|---|---|---|
| Time budget | `time.After(timeout)` in `Wait()` | `cmd.Process.Kill()` | `-1` |
| Token budget | NDJSON frame size ratio ≥ threshold | `cmd.Process.Kill()` | `-2` |
| Analysis paralysis | N consecutive `"type":"thinking"` frames | `cmd.Process.Kill()` | `-2` |

- **Time budget:** Wall-clock timeout from subprocess start to result frame.
  Configured per-target in `mill.yml`, overridable per brief.
- **Token budget:** Accumulated ratio of thinking-token frame size to total
  output. When the ratio exceeds 80% and total tokens exceed the configured
  maximum, the subprocess is killed.
- **Analysis paralysis:** A counter of consecutive `"type":"thinking"` NDJSON
  frames. When it exceeds the configured threshold (`max_consecutive_thinks`),
  the subprocess is killed. This catches loops where the agent reconsiders the
  same question without making progress.

### Exit code mapping

| Exit code | Meaning | Classification |
|---|---|---|
| `-1` | Time budget exceeded | `BLOCKED` |
| `-2` | Token budget or analysis paralysis | `BLOCKED` |
| Other negative | Reserved for future budget types | `BLOCKED` (future) |
| `0` | Normal completion | Normal |
| `> 0` | Subprocess error | Normal (retry logic applies) |

Negative exit codes avoid collision with real process error codes.
`BLOCKED` already skips retry in `runDispatchLoop`, so no new retry
logic is needed.

### Budget configuration

```yaml
# mill.yml — per-target budget with fall-through semantics
targets:
  sr-dev-be:
    budget:
      timeout: 30m
      max_tokens: 200000
      max_consecutive_thinks: 5

  qa-docs:
    budget:
      timeout: 5m
      max_tokens: 50000
```

- Budget is **optional per target**. When absent, enforcement is disabled
  (zero values → fast-path `cmd.Wait()` with no monitoring goroutine).
- **Partial overrides:** unspecified fields fall through to defaults (zero).
  A target that only sets `timeout` gets no token or paralysis enforcement.
- **Brief overrides:** a per-delegation brief can override target defaults,
  same fall-through semantics.

### Internal structure

```
Config (mill.yml)              Adapter (internal/adapter/)
  │                               │
  ├─ Budget struct                ├─ DispatchOpts.Budget
  └─ per-target defaults          │
         │                        │
         ▼                        ▼
    CLI delegate ────────►   Wait() with select:
       loads budget              ├─ time.After(timeout)
       passes to DispatchOpts    ├─ reader goroutine (token + think count)
                                 └─ cmd.Wait() (fast path when zero)
                                           │
                                           ▼
                                    classifyResult(-1/-2)
                                           │
                                           ▼
                                       BLOCKED
```

### Components affected

| Component | Change |
|---|---|
| `mill.yml` | Add optional `budget` block per target |
| `internal/config/config.go` | Add `Budget` struct with `Timeout`, `MaxTokens`, `MaxConsecutiveThinks` |
| `internal/adapter/adapter.go` | Add `Budget` field to `DispatchOpts` |
| `internal/adapter/commandcode.go` | Real-time NDJSON monitoring in `Wait()`; `select` with timeout and threshold channels |
| `internal/cli/delegate.go` | Load budget from config; map exit codes `-1`/`-2` to `BLOCKED` |

## Alternatives considered

- **Event-stream / pub-sub for budget events:** Emit budget-exceeded events
  to a subscriber. Rejected because no consumer exists and event plumbing adds
  complexity without a clear use case. The adapter can kill the subprocess
  directly — no need to notify anyone else first.

- **Post-hoc output analysis:** Wait for the subprocess to finish, then
  analyze the full output for budget violations. Rejected because it wastes
  the entire budget before detection. An agent with a 30-minute budget that
  loops at minute 3 would burn the full 30 minutes for no reason.

- **context.Context for cancellation:** Couple the adapter to the runner's
  context so `ctx.Done()` triggers enforcement. Rejected because it couples
  adapter lifecycle to runner lifecycle. The adapter owns the subprocess and
  should own its cancellation. Context coupling would also make testing
  harder — the adapter's enforcement behavior would depend on external
  cancellation.

- **Runner-side enforcement (in `delegate.go`):** Track budgets in the
  runner by wrapping the adapter call. Rejected because the runner has no
  access to the NDJSON stream (it only sees the final result). Real-time
  token counting and analysis-paralysis detection require NDJSON access,
  which only the adapter has.

## Consequences

### Positive

- **Predictable costs:** Every delegation has an explicit budget ceiling.
  An agent that loops doesn't waste infinite tokens.
- **Automatic stall detection:** Analysis paralysis is caught in real time
  instead of by human observation hours later.
- **Backward compatible:** Zero-value `Budget` means no enforcement. No
  existing behavior changes without explicit configuration.
- **No new interface methods:** Enforcement is internal to the adapter.
  Callers (`delegate.go`) only pass config and check exit codes.

### Negative

- **Increased adapter complexity:** `Wait()` changes from a simple blocking
  `cmd.Wait()` to a goroutine with `select`, pipe reading, and NDJSON
  parsing. This is the core trade-off — enforcement requires real-time
  monitoring, which requires restructuring the subprocess I/O path.
- **Analysis paralysis is heuristic:** Consecutive think-frame counting
  assumes thinks without writes = stall. A legitimate design phase might
  trigger a false positive. The threshold is configurable per target to
  accommodate different task profiles.
- **Token ratio is approximate:** NDJSON frame size is a proxy for token
  count, not an exact measurement. The ratio is directionally accurate but
  not precise.

### Mitigations

- **Configurable thresholds per target:** `max_consecutive_thinks` for
  sr-dev-be can be higher than for qa-docs. Targets with heavy design work
  can set looser thresholds.
- **Fast path for zero budget:** When no budget is configured, `Wait()`
  uses the original blocking `cmd.Wait()` — zero overhead.
- **Negative exit codes:** Unlikely to collide with real process exit codes.
  The harness and agent subprocesses use non-negative exit codes.
- **Descriptive stderr:** On kill, the adapter writes a descriptive message
  to stderr (`"budget exceeded: time (30m)"` or `"budget exceeded: analysis
  paralysis (5 consecutive thinks)"`) so the ledger and `mill status` can
  surface the specific reason.

## References

- Issue #35 — Enforce time and token budgets per task
- Issue #22 — 12 min budget violation in commit message loop
- Issue #32 — 6 min analysis paralysis, zero code
- Issue #1 — Mill: Multi-Agent Delegation Harness
- Issue #25 — Mill CLI: dispatch agent to worktree
- Ticket #49 — Config: Budget struct and mill.yml schema
- Ticket #51 — Adapter: Budget in DispatchOpts
- Ticket #52 — Adapter: Real-time budget enforcement in Wait()
- Ticket #53 — CLI: Wire budget into delegate and classify -1/-2 as BLOCKED
- ADR 0001 — Mill as a Framework on Top of the Harness
