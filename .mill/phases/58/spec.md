# Spec: Slot/concurrency management to prevent model contention

## Architecture

**Problem:** When Mill fans out parallel delegations, all subagents compete for the same AI provider API. This causes rate limiting, increased latency, and failed delegations. The current `runDelegate` launches goroutines with no concurrency control — 5 parallel `mill delegate` calls spawn 5 simultaneous agent processes, all hitting the same provider.

**Solution:** A slot-based concurrency limiter that gates subagent dispatch.

### Slot manager (`internal/slots/`)

New package that manages a fixed-size pool of slots:

```go
type Manager struct {
    maxSlots int
    queue    chan slotRequest
}

type slotRequest struct {
    issue   int
    role    string
    result  chan slotResult
}

func (m *Manager) Acquire(ctx context.Context, issue int, role string) error
func (m *Manager) Release()
func (m *Manager) Status() SlotStatus
```

### Configuration

`mill.yml`:
```yaml
concurrency:
  max-slots: 4   # default: 4
```

Per-provider limits are deferred (future work).

### Slot lifecycle

1. **Acquire:** `runDelegate` calls `slotManager.Acquire()` before spawning the goroutine. If slots are free, returns immediately. If full, blocks until a slot opens (or context deadline).
2. **Dispatch:** Goroutine runs with the acquired slot.
3. **Release:** When the goroutine's `runDispatchLoop` returns (terminal state), it calls `slotManager.Release()`. The slot is released even if the dispatch failed (hanging agents time out via budget).
4. **Priority:** `mill delegate --priority` acquires the next available slot ahead of waiters. The `--priority` flag is Staff-only (validated against active role).

### Queue behavior

When all slots are occupied:
- New delegations wait in a FIFO queue (Go channel, buffered to unlimited via goroutine-based queue)
- The delegating parent is notified: "Delegation queued — 3/4 slots occupied, position 2"
- After 120 seconds of waiting, a warning is emitted: "Delegation to Sr.Dev waiting 120s (position 1)"
- If a priority delegation arrives, it preempts the next slot

### Slot status command

`mill slots` prints a live table:
```
SLOTS: 2/4 occupied
  1: Sr.Dev FE (issue #55) — running 45s
  2: Tech Lead (issue #60) — running 12s

QUEUE: 2 waiting
  #1: Sr.Dev BE (issue #61) — waiting 30s
  #2: QA/Docs (issue #62) — waiting 5s
```

### Integration with `runDelegate`

The `runDelegate` function gains slot acquisition:
```go
func (a *App) runDelegate(args []string) error {
    // ... existing setup (flags, issue parsing, role validation) ...
    
    // Acquire slot before dispatching
    if err := a.slots.Acquire(ctx, issueNum, targetRole); err != nil {
        return fmt.Errorf("slot acquisition failed: %w", err)
    }
    
    // Dispatch in goroutine — Release() called when done
    go func() {
        defer a.slots.Release()
        a.runDispatchLoop(issueNum, taskID, opts)
    }()
    
    return nil
}
```

### Global singleton

The slot manager is a singleton per Mill instance. It lives on the `App` struct (`a.slots`). All `runDelegate` calls share the same manager. This works because `mill delegate` commands within a single session share the same `App` instance.

## Components affected

| File | Change |
|---|---|
| `internal/slots/manager.go` | NEW: Slot manager — acquire, release, queue, status |
| `internal/slots/manager_test.go` | NEW: Tests for acquire/release, FIFO ordering, priority preemption, timeout |
| `internal/cli/delegate.go` | MODIFY: `runDelegate` acquires/releases slots; add `--priority` flag |
| `internal/cli/app.go` | MODIFY: Add `"slots"` case to Run switch; add `slots` field to App struct |
| `internal/cli/slots.go` | NEW: `runSlots` status command |
| `internal/config/config.go` | MODIFY: Add `Concurrency` struct to Config |
| `mill.yml` template | MODIFY: Add `concurrency:` section |

### Files NOT affected
- `internal/adapter/` — slot management is above the adapter layer
- `internal/state/` — no schema changes
- `internal/domain/` — no new types

## Risks

### Risk 1: Deadlock if slot never released
**Severity:** Medium. **Mitigation:** `defer a.slots.Release()` in the goroutine ensures release on any exit path (success, error, panic). The budget system (#60's review loop timeout) kills hanging sessions — when `session.Wait()` times out, the goroutine returns and releases the slot. A 5-minute hard limit on slot ownership (configurable) ensures no slot is held indefinitely.

### Risk 2: Slot manager is process-local, not distributed
**Severity:** Low. **Mitigation:** The FRD explicitly scopes slots to "per-Mill instance" — not distributed. Multiple `mill` processes on the same machine each have their own slot manager. This is a known limitation and future work. For now, the CTO runs one Mill instance at a time.

### Risk 3: Priority preemption starves normal delegations
**Severity:** Low. **Mitigation:** Priority is a Staff-only privilege. Staff rarely uses it (emergency escalations). Priority does not evict running tasks — it preempts the *next* available slot. A running low-priority task is never killed.

### Risk 4: Queue grows unbounded if slots never free
**Severity:** Low. **Mitigation:** The 5-minute hard limit on slot ownership (Risk 1) ensures slots free eventually. The 120-second warning gives the user visibility into queue depth. If the queue exceeds 50 items, an error is logged — this is a pathological case indicating a system problem.

## ADR

**NEW ADR: ADR 0009 — Process-local slot manager for concurrency control.** Rationale:
- Solves the immediate problem (provider contention without rate limiting)
- Process-local scope matches Mill's single-instance design
- Channel-based queue is zero-dependency, standard Go
- Priority flag for emergency Staff overrides
- `mill slots` command provides observability
- Future: per-provider limits, distributed coordination

## Acceptance criteria

1. `mill.yml` `concurrency.max-slots` limits concurrent dispatches (default 4)
2. Delegation queued when all slots occupied (FIFO order)
3. Delegating parent notified of queue position
4. 120-second warning for queued delegations
5. `mill delegate --priority` preempts next available slot
6. Slot released on dispatch completion, error, or timeout
7. `mill slots` prints live slot/queue status
8. 5-minute hard limit on slot ownership (configurable)
9. `go test ./internal/slots/` passes
10. `go test ./internal/cli/` passes (delegate + slots commands)
