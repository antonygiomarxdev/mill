# Tasks: Slot/concurrency management to prevent model contention

All tasks modify/update only files listed in the SPEC Components affected table.

## Wave 1 (parallel — independent, no shared deps)

### Task 1: Create `internal/slots/manager.go` — Slot manager core

- role: sr-dev-be
- deps: none
- est: 45m
- file: `internal/slots/manager.go` (NEW)

#### Acceptance Criteria

1. Define `Manager` struct with `maxSlots int`, a channel-based FIFO queue (`chan slotRequest`), and a `mu sync.Mutex`-protected `slots` map of active acquisitions.
2. Define `slotRequest` struct: `issue int`, `role string`, `result chan slotResult`, `priority bool`.
3. Define `slotResult` struct: `err error`, `position int` (queue position at time of enqueue), `acquiredAt time.Time`.
4. Define `SlotStatus` struct with `Occupied []SlotInfo`, `Queue []QueueInfo`, `MaxSlots int`, `HardLimit time.Duration`, `WarningTimeout time.Duration`.
5. Define `SlotInfo` struct: `SlotID int`, `Issue int`, `Role string`, `AcquiredAt time.Time`, `Running time.Duration`.
6. Define `QueueInfo` struct: `Position int`, `Issue int`, `Role string`, `Priority bool`, `EnqueuedAt time.Time`, `Waiting time.Duration`.
7. Implement `NewManager(maxSlots int) *Manager` — initializes the queue channel (buffered to `maxSlots` for signaling, with an internal goroutine-based unbounded queue for waiters), starts the dispatch loop goroutine.
8. Implement `Acquire(ctx context.Context, issue int, role string, priority bool) error`:
   - Sends `slotRequest` to the queue channel
   - Blocks until a slot is available or context is cancelled
   - Priority requests are placed at the front of the queue (preempts next available slot, does NOT evict running tasks)
   - Returns descriptive error on context cancellation/deadline exceeded
9. Implement `Release()` — removes the calling goroutine's slot from the active set and signals the dispatch loop to schedule the next waiter.
10. Implement `Status() SlotStatus` — returns current snapshot: occupied slots with running durations, queued waiters with waiting durations.
11. Implement internal dispatch loop (goroutine spawned in `NewManager`): consumes `slotRequest` from the channel, assigns free slots immediately, enqueues waiters when full.
12. Implement 120-second warning: if a queued waiter has been waiting ≥120s, emit `fmt.Fprintf` to a configurable `io.Writer` (`Manager.Warn`): `"Delegation to <role> waiting 120s (position <N>)"`.
13. Implement 5-minute hard limit on slot ownership (configurable via `Manager.HardLimit`): if a slot has been held > HardLimit, the dispatch loop forcefully reclaims it (cancels the slot context, Release is called) and logs an error: `"Slot <N> held by <role> for <duration> — forced release"`.
14. Queue depth safety valve: if queue length exceeds 50, log an error: `"Slot queue exceeded 50 items — possible deadlock"` — delegation still proceeds (not a hard block).
15. Package follows existing conventions (`internal/adapter/`, `internal/config/`): standard library only, same error patterns, same doc comment style.

### Task 2: Add `Concurrency` to config

- role: sr-dev-be
- deps: none
- est: 10m
- file: `internal/config/config.go` (MODIFY)

#### Acceptance Criteria

1. Add `Concurrency` struct to `config.go`:
   ```go
   type Concurrency struct {
       MaxSlots int `json:"max_slots"`
   }
   ```
2. Add `Concurrency Concurrency` field to `Config` struct with `json:"concurrency,omitempty"` tag.
3. Update `Default()`: set `Concurrency.MaxSlots` to `4`.
4. Existing config tests pass unchanged: `go test ./internal/config/`.
5. `json.Marshal`/`json.Unmarshal` round-trips the new field (omitempty: absent when zero-value, present when set).

### Task 3: Update `mill.yml` template with concurrency section

- role: sr-dev-be
- deps: none
- est: 5m
- file: `internal/cli/static/mill.yml.tmpl` (MODIFY)

#### Acceptance Criteria

1. Add `concurrency:` section to the template after `max-rounds`:
   ```yaml
   # Concurrency control for agent dispatch
   concurrency:
     max-slots: 4   # max simultaneous agent dispatches
   ```
2. The `mill init` command should generate `mill.yml` with a `concurrency:` block containing `max-slots: 4`.
3. Existing init tests pass unchanged: `go test ./internal/cli/ -run Init`.

## Wave 2 (sequential — depends on Wave 1)

### Task 4: Create `internal/slots/manager_test.go` — Slot manager tests

- role: sr-dev-be
- deps: Task 1
- est: 40m
- file: `internal/slots/manager_test.go` (NEW)

#### Acceptance Criteria

1. `TestAcquireReleaseBasic`: acquire 1 slot → Status shows 1 occupied → release → Status shows 0 occupied.
2. `TestAcquireBlocksWhenFull`: create manager with MaxSlots=2, acquire 2 slots (both succeed immediately), acquire 3rd in goroutine with short timeout → verify it blocks until a slot is released.
3. `TestFIFOOrdering`: MaxSlots=1, acquire slot 1, enqueue slots 2 and 3, release slot 1 → verify slot 2 acquires before slot 3.
4. `TestPriorityPreemption`: MaxSlots=1, acquire slot 1 (normal), enqueue slot 2 (normal), enqueue slot 3 (priority), release slot 1 → verify slot 3 (priority) acquires before slot 2.
5. `TestPriorityDoesNotEvictRunning`: MaxSlots=1, acquire slot 1, enqueue priority slot 2 → verify slot 1 is NOT released (priority only preempts next available, not running).
6. `TestContextCancellation`: MaxSlots=1, acquire slot 1 (fills it), start Acquire with a cancelled context → returns error immediately (context.Canceled).
7. `TestContextDeadline`: MaxSlots=1, acquire slot 1, start Acquire with 50ms deadline → returns context.DeadlineExceeded after timeout.
8. `TestHardLimitReclaim`: create manager with HardLimit=100ms, acquire slot, sleep 150ms → verify Status shows 0 occupied (slot reclaimed), verify error logged.
9. `TestWarningTimeout`: create manager with WarningTimeout=50ms, MaxSlots=1, acquire slot 1, enqueue waiter, sleep 100ms → verify warning message written to Warn writer.
10. `TestQueueDepthWarning`: MaxSlots=1, acquire slot 1, enqueue 51 waiters → verify error logged about queue depth >50.
11. `TestStatusSnapshot`: acquire 2 of 4 slots, enqueue 2 waiters → Status returns correct Occupied count (2), Queue count (2), MaxSlots (4), and durations are non-zero.
12. `TestReleaseUnknownSlot`: Release when no slot is held → does not panic (no-op or logged warning).
13. `TestConcurrentAcquireRelease`: spawn N goroutines acquiring and releasing slots concurrently with MaxSlots=3 → no deadlocks, no data races (`go test -race`).
14. Follow existing test conventions: table-driven tests, `*testing.T`, standard library only.

### Task 5: Integrate slot Acquire/Release into `runDelegate`

- role: sr-dev-be
- deps: Task 1, Task 2
- est: 30m
- file: `internal/cli/delegate.go` (MODIFY)

#### Acceptance Criteria

1. Set `maxSlots` from config: after `cfg, err := a.loadConfig()`, read `maxSlots := cfg.Concurrency.MaxSlots`. Default to 4 if `maxSlots <= 0`.
2. Initialize `a.slots` if nil: `if a.slots == nil { a.slots = slots.NewManager(maxSlots) }`.
3. Add `--priority` flag to the flag set: `fs.BoolVar(&priority, "priority", false, "preempt next available slot (staff only)")`.
4. Validate priority flag: if `--priority` is set, verify `activeRole == "staff"`. If non-staff uses `--priority`, return error: `"--priority is restricted to staff role"`.
5. If `--priority` is used and valid, set `priorityFlag = true`.
6. Call `a.slots.Acquire(ctx, issueNum, targetRole, priorityFlag)` before the goroutine dispatch. Use `context.Background()` with no deadline (the slot manager's HardLimit handles indefinite holds).
7. On `Acquire` error: `return fmt.Errorf("slot acquisition failed: %w", err)` — do not proceed to dispatch.
8. Notify parent of queue position: if queue position > 0 at acquisition, emit: `fmt.Fprintf(a.Err, "Delegation queued — %d/%d slots occupied, position %d\n", occupied, maxSlots, position)`.
9. Move the goroutine dispatch (`go a.runDispatchLoop(...)`) into a wrapper that calls `defer a.slots.Release()`:
   ```go
   go func() {
       defer a.slots.Release()
       a.runDispatchLoop(issueNum, taskID, opts)
   }()
   ```
10. The `wait` path also acquires/releases: `Acquire` before dispatch, `defer Release` in the synchronous call.
11. The slot is released on ALL exit paths (success, error, panic) because of `defer`.
12. Existing delegate tests pass with a nil `a.slots` (Manager gracefully handles nil — `Acquire` on nil manager is a no-op). If a test needs slot behavior, it sets `a.slots`.
13. The `App` struct gains a `slots *slots.Manager` field (not exported). Add to `app.go`.

### Task 6: Create `mill slots` CLI command

- role: sr-dev-be
- deps: Task 1
- est: 20m
- file: `internal/cli/slots.go` (NEW) + `internal/cli/app.go` (MODIFY)

#### Acceptance Criteria

1. Create `internal/cli/slots.go` with `func (a *App) runSlots(args []string) error`.
2. If `a.slots` is nil, print `"No active slot manager. Run 'mill delegate' first."` and return nil.
3. Call `status := a.slots.Status()`.
4. Print formatted table to `a.Out`:
   ```
   SLOTS: <occupied>/<max> occupied
     <N>: <Role> (issue #<issue>) — running <duration>
     ...
   QUEUE: <count> waiting
     #<position>: <Role> (issue #<issue>) — waiting <duration>
     ...
   ```
5. If no slots occupied: print `"SLOTS: 0/<max> — idle"`.
6. If no queue: omit the QUEUE section entirely.
7. Add `"slots"` case to the `Run` switch in `app.go:83`:
   ```go
   case "slots":
       return a.runSlots(args[1:])
   ```
8. Add `"slots"` to the `usage` help text: `"  slots              Show slot/concurrency status"`.
9. `go build ./...` compiles cleanly.

## Wave 3 (depends on Wave 2)

### Task 7: Integration tests for delegate + slots commands

- role: sr-dev-be
- deps: Task 4, Task 5, Task 6
- est: 35m
- file: `internal/cli/delegate_test.go` (MODIFY) + `internal/cli/slots_test.go` (NEW)

#### Acceptance Criteria

1. **Slot acquisition in delegate path:**
   - `TestDelegateAcquiresSlot`: set `a.slots` to new Manager with MaxSlots=1. Assert after `runDelegate` returns, `a.slots.Status().Occupied` has 1 entry with correct issue and role.
   - `TestDelegateReleasesSlotOnCompletion`: fakeAdapter returns OK immediately. `runDelegate` with `--wait`. After completion, `a.slots.Status().Occupied` is 0.
   - `TestDelegateReleasesSlotOnError`: fakeAdapter returns exit code 1. After dispatch loop finishes, slot is released.

2. **Slot blocking behavior:**
   - `TestDelegateQueuedWhenFull`: MaxSlots=1, acquire a slot externally (fill it), then call `runDelegate`. Verify error output contains "Delegation queued".
   - `TestDelegateBlocksUntilSlotFree`: MaxSlots=1, fill slot, call runDelegate in goroutine, release slot after 200ms → verify delegation proceeds.

3. **Priority flag:**
   - `TestDelegatePriorityStaff`: activeRole="staff", `--priority` → no error, priority flag passed to Acquire.
   - `TestDelegatePriorityNonStaffRejected`: activeRole="sr-dev-be", `--priority` → error "restricted to staff role".
   - `TestDelegatePriorityPreemptsQueue`: MaxSlots=1, acquire slot (fill it), enqueue normal delegation, enqueue priority delegation → verify priority acquires first when slot frees.

4. **Slots command:**
   - `TestSlotsIdle`: `a.slots` initialized with MaxSlots=4, no acquisitions → output contains "0/4" and "idle".
   - `TestSlotsWithOccupied`: acquire 2 slots → output contains "2/4 occupied", shows issue numbers and roles.
   - `TestSlotsWithQueue`: MaxSlots=1, acquire 1 slot, enqueue 2 → output contains "QUEUE: 2 waiting", shows positions.
   - `TestSlotsNilManager`: `a.slots` is nil → prints "No active slot manager".

5. **Config integration:**
   - `TestConfigConcurrencyDefault`: default config has `Concurrency.MaxSlots == 4`.
   - `TestConfigConcurrencyRoundTrip`: marshal/unmarshal config with `Concurrency{MaxSlots: 8}` → survives round-trip.

6. All tests pass: `go test ./internal/slots/`, `go test ./internal/config/`, `go test ./internal/cli/`.
7. Race detector clean: `go test -race ./internal/slots/`.

---

## Acceptance criteria (cross-cutting)

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
