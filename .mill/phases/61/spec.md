# Spec: `mill watch` must block until all tasks settle

## Architecture

**Problem:** `mill watch` is not implemented. The `Run` dispatch in `app.go` has no `watch` case — the command falls through to the default "unknown command" error, which shows help text. Staff has no way to observe async delegated tasks completing; they manually check `loop-*.log` files from the bash harness.

**Solution:** Add a `watch` subcommand that polls `.mill/state.json` at a configurable interval until all tasks reach a terminal state, then prints a summary table and exits.

### State polling model

The watch loop reads state on each tick:
1. Load `.mill/state.json`
2. Filter tasks — only tasks created by `mill delegate` (not manually inserted)
3. Print a progress line for each non-terminal task (status + elapsed time)
4. If all tasks are terminal (TaskDone or TaskError), print final summary and exit with code 0
5. If any task errored, exit code is non-zero (1 + number of errored tasks, capped at 125)

### Terminal states

A task is terminal when `Status ∈ {TaskDone, TaskError}`. The `TaskStatus` constants are already defined in `internal/domain/status.go`:
- `TaskRunning` → non-terminal
- `TaskDone` → terminal (success)
- `TaskError` → terminal (failure)

### Watch behavior

```
Usage: mill watch [--interval <seconds>] [--timeout <seconds>]

--interval: polling interval in seconds (default: 2)
--timeout: maximum wait time in seconds (default: 0 = no timeout)

Exit codes:
  0 — all tasks completed successfully
  1 — one task errored
  N — N tasks errored (capped at 125)
  124 — timeout reached with tasks still running
```

Progress output format (overwrites same line with `\r`):
```
Watching 3 tasks... task-392: running (12s) | task-393: running (8s) | task-394: running (3s)
```

Final summary:
```
ID         ISSUE  STATUS  COMMITS  VERDICT
task-392   392    done    5        APPROVED
task-393   393    done    3        APPROVED
task-394   394    error   0        FATAL

2/3 tasks succeeded, 1 failed
```

### Architecture decision

The watch command is a **CLI-only concern** — it reads state, prints progress, and exits. It does NOT:
- Start or manage goroutines (that's `mill delegate`'s job)
- Modify state or ledger
- Depend on the adapter layer
- Require a running daemon or service

This keeps the implementation minimal: one new function in `internal/cli/watch.go` plus a `"watch"` case in `app.go`.

## Components affected

| File | Change |
|---|---|
| `internal/cli/watch.go` | NEW: `runWatch` function implementing the poll loop |
| `internal/cli/watch_test.go` | NEW: Tests for terminal detection, progress output, exit codes |
| `internal/cli/app.go` | MODIFY: Add `case "watch": return a.runWatch(args[1:])` to Run switch |
| `internal/cli/app_test.go` | MODIFY: Add test for `mill watch` routing |

### Files NOT affected
- `internal/state/` — no schema changes; watch is read-only
- `internal/domain/` — no new types needed; TaskStatus constants are sufficient
- `internal/adapter/` — watch does not dispatch agents
- `internal/ledger/` — watch reads state, not ledger

## Risks

### Risk 1: Race between state save and poll
**Severity:** Low. **Mitigation:** The goroutine in `runDispatchLoop` calls `s.Save()` atomically — the file is written entirely via `os.WriteFile`. On the watch side, `state.Load()` reads the file atomically. There is no partial-write window. Worst case: watch reads stale data for one poll cycle (≤2 seconds), then re-reads the updated file on the next tick.

### Risk 2: Watch blocks indefinitely if a task hangs
**Severity:** Medium. **Mitigation:** The `--timeout` flag provides an escape hatch. Without `--timeout`, watch blocks until all tasks settle. Staff can always Ctrl+C. If timeout fires, exit code 124 signals "timeout reached" so scripts can distinguish timeout from failure. The watch command also prints which tasks are still running when timeout fires.

### Risk 3: State file grows with completed tasks
**Severity:** Low. **Mitigation:** `mill watch` reads all tasks from state. With 100 completed tasks, the state file is ~10KB — negligible. If state grows to problematic sizes, a future `mill cleanup` (#59) can archive completed tasks. For now, no action needed.

### Risk 4: Watch output is not machine-parseable
**Severity:** Low. **Mitigation:** The progress lines use `\r` overwrite (terminal-only). The final summary table uses tabwriter (same as `mill status`), which output to pipes is tab-separated and parseable. A `--json` flag can be added later if needed; not in scope for this spec.

## ADR

No new ADR. The watch command is a read-only observer — it introduces no new architectural boundaries or cross-cutting decisions. Existing ADRs apply:
- **ADR 0001** (Mill as Framework): Watch is a CLI concern within the framework's escape-hatch layer.

## Acceptance criteria

1. `mill watch` blocks until all tasks in `.mill/state.json` reach terminal state
2. Progress is printed at regular intervals showing task status and elapsed time
3. Exit code 0 when all tasks succeed, non-zero when any task errors
4. `--interval` flag controls polling frequency (default 2s)
5. `--timeout` flag sets a maximum wait (default: no timeout)
6. Help text shown for `mill watch --help`
7. `go test ./internal/cli/` passes
8. `mill watch` with no tasks exits immediately with "No tasks to watch"
