# Tasks: `mill delegate` — issue body context + review loop

## Wave 1 (parallel — independent, no shared deps)

- [ ] **Add `Task.Round` field to domain** — role: sr-dev-be, deps: none, est: 10m
  1. `Task` struct in `internal/domain/task.go` gains `Round int` field with `json:"round"` tag — zero value tracks first dispatch round
  2. `NewTask` initializes `Round` to `0`
  3. Existing domain tests compile and pass unchanged: `go test ./internal/domain/`
  4. `state` serialization round-trips `Round` — `state_test.go` tests pass unchanged: `go test ./internal/state/`

- [ ] **Issue body reader** — role: sr-dev-be, deps: none, est: 30m
  1. Create `internal/issue/reader.go`: `ReadBody(issueNum int) (body string, labels []string, err error)` shells out to `gh issue view N --json body,labels --repo "$(git remote get-url origin)"`
  2. Parse JSON output: extract `body` as string, extract `labels[].name` into `[]string` (only `name` field — stage filtering done in CLI layer)
  3. Return descriptive error when `gh` binary not found in `$PATH`: `"gh CLI not found — install github.com/cli/cli"`
  4. Return wrapped error when `gh issue view` fails (issue not found, auth failure, network error)
  5. Create `internal/issue/reader_test.go`: table-driven tests for valid body+labels, gh-not-found exit code, empty body, empty labels, and mixed label set including non-stage labels
  6. All tests pass: `go test ./internal/issue/`

## Wave 2 (sequential — depends on Wave 1)

- [ ] **Review loop + stage routing + body integration in `delegate.go`** — role: sr-dev-be, deps: Task.Round + ReadBody, est: 60m
  1. `runDelegate`: call `issue.ReadBody(issueNum)` after parsing issue number; propagate read errors cleanly to user
  2. Extend `buildRolePrompt(issueNum int, targetRole string, issueBody string)` — append body text as `**Issue Body:**\n<issueBody>` block when non-empty; maintain backward-compatible call signature (empty body = no block)
  3. Extend `resolveModel` with `stageLabel string` parameter: `stage:produce`→`laguna-free`, `stage:review`→`laguna-pro`, `stage:implement`→`laguna-free`, empty/missing→existing role-default/config fallback behavior; multiple `stage:*` labels → first wins with `fmt.Fprintf(a.Err, …)` warning
  4. Refactor `runDispatchLoop` into produce→review cycle:
     - **Produce phase:** dispatch with cheap model (from stage routing), issue body context in prompt
     - **Review phase:** dispatch with expensive model, diff output as context, classify result via `classifyResult`
     - **Loop:** repeat produce→review up to `cfg.MaxRounds` (default 4); exit on APPROVED; exit with MAX_TURNS when rounds exhausted; exit immediately on BLOCKED/FATAL/AUTH/NO_CREDIT/RATE_LIMITED
  5. Each review phase classification appends ledger entry with new `Round int` field on `ledger.Entry` (zero value = first round); increment `task.Round` after each round
  6. Persist state via `state.UpsertTask` + `state.Save` after each round (not just at loop end) so `mill watch` can observe progress
  7. Map final verdict: APPROVED → `VerdictApproved`, CHANGES_REQUESTED after max rounds → `VerdictChanges` with `ClassificationMaxTurns`, BLOCKED/FATAL/AUTH → `VerdictRejected`
  8. Async behavior preserved: `runDelegate` spawns goroutine for non-`--wait` path, returns immediately with "Delegated issue N" message
  9. Existing `delegate_test.go` tests updated for new signatures; all pass: `go test ./internal/cli/`

## Wave 3 (depends on Wave 2)

- [ ] **Delegate tests for review loop, body context, and stage routing** — role: sr-dev-be, deps: Review loop integration, est: 45m
  1. `TestDelegateBodyInPrompt`: create `gh` mock script in `$PATH`, verify `runDelegate` reads issue body and body text appears in `DispatchOpts.Prompt`
  2. `TestReviewLoopSingleRound`: fakeAdapter returns OK on first produce→review cycle, verify final task has `VerdictApproved` and `Round == 1`
  3. `TestReviewLoopMaxRounds`: fakeAdapter returns CHANGES_REQUESTED every review, verify loop exhausts `MaxRounds` and task gets `ClassificationMaxTurns` verdict
  4. `TestReviewLoopBlocked`: fakeAdapter returns BLOCKED on first review, verify loop exits immediately, verdict is `VerdictRejected`
  5. `TestStageLabelProduce`: issue labels `["stage:produce"]`, verify produce dispatch uses `"laguna-free"` model
  6. `TestStageLabelReview`: issue labels `["stage:review"]`, verify review dispatch uses `"laguna-pro"` model
  7. `TestReviewLoopLedgerRoundNumbers`: verify ledger entries contain incrementing round numbers across a 2-round cycle
  8. `TestDelegateAsyncReturn`: `--wait=false` path returns immediately with non-nil `Out` message, task state in `state.json` is `TaskRunning`
  9. All tests pass: `go test -count=1 ./internal/cli/`
