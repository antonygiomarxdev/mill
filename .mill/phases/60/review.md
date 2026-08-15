# Review: #60 — `mill delegate` issue body context + review loop

## Verdict

**APPROVED**

## Gate Results

- `go build ./...` — PASS
- `go test ./internal/issue/` — PASS (6 tests, including ReadBodyValid, ReadBodyGhostNotFound, ReadBodyFakeGh, ReadBodyFakeGhEmptyBody, ReadBodyFakeGhFailure, TestStageLabel)
- `go test ./internal/cli/` — PASS (all delegate tests: TestDelegateValidIssueDispatchesAndRecords, TestDelegateExitCodeErrorSetsTaskError, TestDelegateCreatesLedgerEntry, TestDelegateDispatchErrorRecordsError, TestDelegateModelFlagOverridesConfig, TestResolveModelStageLabelProduce, TestResolveModelStageLabelReview, TestResolveModelStageLabelImplement, TestBuildRolePromptWithBody, TestBuildReviewPrompt)
- `go test ./internal/domain/` — PASS
- `go test ./internal/state/` — PASS
- `go test ./internal/ledger/` — PASS

## Acceptance Criteria Verification

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | `mill delegate <issue>` reads issue body via `gh` and passes to agent | ✅ | `runDelegate` calls `a.IssueReader(issueNum)` (delegate.go:60), which defaults to `issue.ReadBody` in `NewApp()` (app.go:34). Body text passed to `buildRolePrompt(issueNum, targetRole, issueBody)` (delegate.go:126). |
| 2 | Produce→review loop up to `max-rounds` from config | ✅ | `runDispatchLoop` iterates `maxRounds` (delegate.go:156). Each round: dispatch produce agent + dispatch review agent + classify + persist. |
| 3 | Review phase uses different model than produce | ✅ | Review model is `"laguna-pro"` (delegate.go:153). Produce model from stage label or `opts.Model` (delegate.go:143-149). |
| 4 | `stage:*` labels influence model selection | ✅ | `issue.StageLabel(labels)` extracts label (reader.go:56-62). `resolveModel` maps `stage:produce`→`laguna-free`, `stage:review`→`laguna-pro`, `stage:implement`→`laguna-free` (delegate.go:465-473). |
| 5 | Each round recorded in ledger with round number | ✅ | After each review phase, `ledger.Entry{Round: round}` appended (delegate.go:248-262). |
| 6 | Final verdict persisted in state | ✅ | `finish` label calls `task.UpdateStatus(taskStatus, verdict, finalCommits)`, `s.UpsertTask(task)`, `s.Save(...)` (delegate.go:283-289). Also emits complete ledger entry (delegate.go:291-301). |
| 7 | Async behavior preserved | ✅ | When `!wait`: `go a.runDispatchLoop(...)` + immediate return with "Delegated issue N — task task-N (async)" (delegate.go:136-139). |
| 8 | `go test ./internal/cli/` passes | ✅ | All 56 tests pass (verified by `go test -count=1 -v`). |
| 9 | `go test ./internal/issue/` passes | ✅ | All 6 tests pass including new reader tests. |

## Architecture Compliance

- **ADR 0004 (Review Loop as CLI Concern):** ✅ Review loop lives in `internal/cli/delegate.go`, not in `internal/adapter/`. Adapter interface is unchanged.
- **ADR 0001 (Mill as Framework):** ✅ CLI remains escape hatch.
- **ADR 0002 (Budget Enforcement):** ✅ Budget passed through both produce and review dispatch opts.

## Quality Checks

- No `any`, `unknown`, `Record<string, T>`, or `object` types introduced ✅
- `IssueReader` field on `App` enables test injection without mocking `gh` ✅
- `Task.Round` field added with `json:"round"` tag, zero value tracks first round ✅
- `ledger.Entry.Round` field added with `json:"round"` tag ✅
- Tests use `fakeAdapter` + `defaultIssueReader` for isolation ✅

## Files Reviewed

- `internal/issue/reader.go` (NEW) — `ReadBody`, `StageLabel`
- `internal/issue/reader_test.go` (NEW) — 6 tests
- `internal/cli/delegate.go` (MODIFY) — `runDelegate`, `runDispatchLoop`, `buildRolePrompt`, `buildReviewPrompt`, `resolveModel`
- `internal/cli/delegate_test.go` (MODIFY) — stage routing, body-in-prompt, review prompt tests
- `internal/cli/app.go` (MODIFY) — `IssueReader` field on `App`
- `internal/domain/task.go` (MODIFY) — `Round int` field
- `internal/ledger/ledger.go` (MODIFY) — `Round int` field on `Entry`

## Notes

- `buildReviewPrompt` emits specific signal instructions (APPROVED/CHANGES_REQUESTED/BLOCKED on stderr) matching `classifyResult`'s stderr priority parsing.
- Multiple `stage:*` labels trigger a warning to stderr (delegate.go:73-79).
- State persisted after each round so `mill watch` (#61) can observe progress.
- `MaxRounds` defaults to 4 when config value is ≤ 0 (delegate.go:154-157).
