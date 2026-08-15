# Tasks: `mill watch` — block until all tasks settle

## Wave 1 (parallel — independent files)

- [ ] **Create `internal/cli/watch.go` — `runWatch` poll loop and output** — role: sr-dev-be, deps: none, est: 60m
  1. Implement `runWatch(args []string) error` on `*App`
  2. Parse flags: `--interval` (default 2s, type int/float64), `--timeout` (default 0 = no timeout, seconds)
  3. Load state from `a.statePath()` via `state.Load`
  4. Filter tasks to only those created by `mill delegate` — match task ID prefix `"task-"` (delegate uses `fmt.Sprintf("task-%d", issueNum)`)
  5. If no tasks remain after filtering, print `"No tasks to watch"` to stdout and return nil
  6. Poll loop: on each tick, load state fresh, compute progress line with `\r` overwrite
  7. Progress format: `"Watching N tasks... <id>: <status> (<elapsed>s) | ..."` with elapsed = `time.Since(task.StartedAt).Truncate(time.Second)`
  8. Terminal check: task is terminal when `Status == domain.TaskDone || Status == domain.TaskError`
  9. Exit codes: all succeeded → 0; N errors → min(1+N, 125); timeout → 124
  10. Summary table on completion (reuse `tabwriter` pattern from `runStatus`): ID, ISSUE, STATUS, COMMITS, VERDICT columns, followed by `"N/M tasks succeeded, K failed"` line
  11. Timeout: if `--timeout > 0`, start a deadline timer; when it fires, print which tasks are still running and return exit code 124
  12. Help text: show usage when `-h` or `--help` in args, matching format in spec
  13. Use `a.Out` for progress/summary, `a.Err` for errors only

- [ ] **Modify `internal/cli/app.go` — add watch routing** — role: sr-dev-be, deps: none, est: 5m
  1. Add `case "watch": return a.runWatch(args[1:])` to the `Run` switch block (before `default`)
  2. No new imports needed if `runWatch` lives in `watch.go` (same package)

## Wave 2 (parallel — depends on Wave 1)

- [ ] **Create `internal/cli/watch_test.go` — watch behavior tests** — role: sr-dev-be, deps: Task 1 (watch.go), est: 45m
  1. `TestRunWatchNoTasks`: state with zero tasks → output contains `"No tasks to watch"`, exit code 0
  2. `TestRunWatchAllDone`: state with one `TaskDone` task → output contains `"tasks succeeded"`, exit code 0
  3. `TestRunWatchOneError`: state with one `TaskDone` + one `TaskError` → output contains `"1/2 tasks succeeded, 1 failed"`, exit code 1+1=2 (capped at 125)
  4. `TestRunWatchProgressOutput`: state with one `TaskRunning` task → verify progress line contains `"running"` and elapsed time
  5. `TestRunWatchTimeout`: state with a `TaskRunning` task, `--timeout 1` → exit code 124, output mentions still-running tasks
  6. `TestRunWatchInterval`: pass `--interval 1`, verify the flag is parsed (no panic)
  7. `TestRunWatchHelpFlag`: `runWatch([]string{"-h"})` → prints usage, nil error (per Go convention, `flag.ErrHelp` → nil)
  8. `TestRunWatchFilterOnlyDelegateTasks`: state contains one `task-42` (delegate) + one `manual-insert` (non-delegate task with ID not matching `task-` prefix) → only `task-42` appears in output
  9. All tests use `t.TempDir()` for isolated `.mill/state.json` files; write state via `state.State{}.Save(path)`

- [ ] **Modify `internal/cli/app_test.go` — watch routing test** — role: sr-dev-be, deps: Task 2 (app.go), est: 10m
  1. Add `TestRunWatchRouting`: calls `app.Run("watch")` and asserts no `"unknown command"` error (verify it dispatches to watch, not default)
