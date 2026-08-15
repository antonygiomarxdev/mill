# Review: #61 — `mill watch` block until all tasks settle

## Verdict

**APPROVED**

## Gate Results

- `go build ./...` — PASS
- `go test ./internal/cli/` — PASS (9 watch tests: TestRunWatchNoTasks, TestRunWatchAllDone, TestRunWatchOneError, TestRunWatchProgressOutput, TestRunWatchTimeout, TestRunWatchInterval, TestRunWatchHelpFlag, TestRunWatchFilterOnlyDelegateTasks, TestRunWatchRouting)
- `go test ./internal/domain/` — PASS
- `go test ./internal/state/` — PASS

## Acceptance Criteria Verification

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | `mill watch` blocks until all tasks terminal | ✅ | Poll loop in `runWatch` calls `allTerminal(delegateTasks)` (watch.go:64-70). When true, prints final summary and returns exit code. Terminal = `TaskDone || TaskError` (watch.go:32-34). |
| 2 | Progress printed at intervals showing status and elapsed | ✅ | `printProgress` writes `\rWatching N tasks... id: status (Xs) | ...` (watch.go:97-103). Elapsed = `int(now.Sub(t.StartedAt).Seconds())`. |
| 3 | Exit code 0 on success, non-zero on error | ✅ | `computeExitCode` returns `CommandError{Code: 1+errorCount, Msg: "..."}`, capped at 125 (watch.go:133-142). Timeout returns code 124 (watch.go:76). |
| 4 | `--interval` flag controls polling frequency | ✅ | `flag.Int("interval", 2, ...)` (watch.go:40). Used as `time.Duration(*interval) * time.Second` for ticker. |
| 5 | `--timeout` flag sets max wait | ✅ | `flag.Int("timeout", 0, ...)` (watch.go:41). When > 0, deadline timer fires → exit code 124 with timeout summary. |
| 6 | Help text for `mill watch --help` | ✅ | `printWatchUsage` outputs usage text matching spec format (watch.go:78-91). Pre-scanned before flag parsing so `-h`/`--help` always works. |
| 7 | `go test ./internal/cli/` passes | ✅ | All 9 watch-specific tests pass. |
| 8 | "No tasks to watch" when empty | ✅ | `filterDelegateTasks` returns empty slice → `fmt.Fprintln(a.Out, "No tasks to watch")` (watch.go:55-57). |

## Architecture Compliance

- **ADR 0001 (Mill as Framework):** ✅ Watch is a CLI-only concern — reads state, prints progress, exits. No adapter dependency.
- **Watch does not start/manage goroutines** ✅ — that's `mill delegate`'s job.
- **Watch does not modify state or ledger** ✅ — read-only.
- **`filterDelegateTasks` matches `task-` prefix** ✅ — only delegate-created tasks appear.

## Quality Checks

- No `any`, `unknown`, `Record<string, T>`, or `object` types introduced ✅
- `CommandError` type carries exit codes for proper os.Exit usage ✅
- State reloaded on each poll tick (not cached) so goroutine updates are observed ✅
- `clearProgress` overwrites progress line before printing final summary ✅
- `tabwriter` used for summary table, matching `runStatus` convention ✅
- Tests use `t.TempDir()` for isolated state files ✅
- Watch routing added to `app.go` Run switch (line 97): `case "watch": return a.runWatch(args[1:])` ✅

## Files Reviewed

- `internal/cli/watch.go` (NEW) — `runWatch`, `printWatchUsage`, `isTerminal`, `allTerminal`, `filterDelegateTasks`, `printProgress`, `clearProgress`, `printFinalSummary`, `printTimeoutSummary`, `computeExitCode`
- `internal/cli/watch_test.go` (NEW) — 9 tests
- `internal/cli/app.go` (MODIFY) — watch routing case

## Notes

- `TestRunWatchTimeout` correctly uses `--timeout 1` to verify exit code 124.
- `TestRunWatchFilterOnlyDelegateTasks` verifies that non-delegate tasks (e.g., `manual-insert`) don't appear in output.
- `TestRunWatchRouting` verifies the `app.Run("watch")` routing doesn't produce "unknown command" error.
