# Tasks: Review loop — produce, review, rework, approve/reject

## Wave 1 (parallel — independent, no shared deps)

- [ ] **Add `VerdictChangesRequested` to domain** — role: sr-dev-be, deps: none, est: 10m
  1. `internal/domain/verdict.go`: add `VerdictChangesRequested Verdict = "changes_requested"` constant
  2. Existing domain tests pass unchanged: `go test ./internal/domain/`

- [ ] **Add review verdict stderr signal patterns to `classifyResult`** — role: sr-dev-be, deps: none, est: 20m
  1. `internal/domain/classification.go`: add `ClassificationChangesRequested Classification = "CHANGES_REQUESTED"` constant
  2. `internal/cli/delegate.go` `classifyResult`: add stderr signal check for `"changes_requested:"` → `ClassificationChangesRequested` (checked AFTER `"blocked:"` and BEFORE auth signals, so review verdicts take priority over incidental error signals)
  3. `internal/cli/delegate.go` `classifyResult`: add stderr signal checks for `"approved:"` → `ClassificationOK` and `"blocked:"` → `ClassificationBlocked` (review-specific signals; `approved:` maps to OK to trigger done path, `blocked:` already handled but ensure it's the first check for priority)
  4. Existing `TestClassifyResultExitCodes` in `delegate_test.go` updated to cover `CHANGES_REQUESTED:`, `APPROVED:`, and `BLOCKED:` stderr signals
  5. All tests pass: `go test ./internal/cli/`

## Wave 2 (sequential — depends on Wave 1)

- [ ] **Extend `runDispatchLoop` with produce→review→rework cycle** — role: sr-dev-be, deps: VerdictChangesRequested + classification signals, est: 60m
  1. `internal/cli/delegate.go` `runDispatchLoop`: restructure from single retry-loop into produce→review→rework cycle:
     - **Produce phase:** one dispatch with cheap model; captures output + diff for review context
     - **Review phase:** second dispatch with expensive model; review prompt includes issue body, diff, acceptance criteria (see Task 3)
     - **Verdict routing:** parse stderr for `APPROVED:`, `CHANGES_REQUESTED:`, `BLOCKED:` signals
     - **APPROVED:** persist state → `TaskDone` + `VerdictApproved` → exit loop
     - **CHANGES_REQUESTED:** if `round < maxRounds` → rework (new produce dispatch with feedback) → re-review; if `round >= maxRounds` → exit with `ClassificationMaxTurns` + `VerdictChangesRequested` + escalate message to `a.Err`
     - **BLOCKED / AUTH / FATAL:** immediate exit → `VerdictRejected` + escalate to Staff via `fmt.Fprintf(a.Err, …)`
  2. Max cycles = `cfg.MaxRounds` (default 4 from `config.Default()`). Enforce: after 3 `CHANGES_REQUESTED` verdicts (= cycle count exhausted), escalate to Staff with summary of all review feedback
  3. Persist state (`state.UpsertTask` + `state.Save`) after EACH phase (produce and review), not just at loop end, so `mill watch` (#61) can observe progress
  4. Ledger entries per round: produce entry with `event: "produce"`, round number, model; review entry with `event: "review"`, round number, verdict, classification, model. Use `ledger.Entry` existing fields (no struct changes needed per spec)
  5. `runDispatchLoop` signature unchanged: `func (a *App) runDispatchLoop(issueNum int, taskID string, opts adapter.DispatchOpts) error`
  6. Existing delegate tests updated to pass with new loop structure; all pass: `go test ./internal/cli/`

- [ ] **Build review prompt function** — role: sr-dev-be, deps: none (pure function, independent of loop), est: 25m
  1. `internal/cli/delegate.go`: new function `buildReviewPrompt(issueBody string, diffOutput string, acceptanceCriteria []string) string`
  2. Prompt structure:
     - **Role instruction:** "You are a code reviewer. Review the following code change against the acceptance criteria."
     - **Issue body** block: `## Issue\n<issueBody>`
     - **Acceptance criteria** block: `## Acceptance Criteria\n- <criterion>` per item
     - **Diff** block: `## Changes (diff)\n<diffOutput>`
     - **Output format template:** strict required format:
       ```
       Output your verdict on stderr as one of:
       - APPROVED: (work meets all acceptance criteria)
       - CHANGES_REQUESTED: (numbered, specific, criteria-referencing feedback items)
       - BLOCKED: (cannot proceed — missing info or external dependency)
       ```
     - **Quality rules:** (a) every CHANGES_REQUESTED item MUST reference a specific acceptance criterion; (b) no vague feedback like "this doesn't look right"; (c) if all criteria met, MUST output APPROVED
  3. Handle empty issue body and empty criteria lists gracefully (emit warnings, still dispatch review)
  4. Unit-testable pure function — no receiver on `*App`, no I/O side effects
  5. Create `internal/cli/review_prompt_test.go`: table-driven tests for nil/empty criteria, long diff, all three verdict templates present in output

## Wave 3 (parallel — depends on Wave 2)

- [ ] **Add review timeout and max-cycles escalation** — role: sr-dev-be, deps: runDispatchLoop cycle, est: 30m
  1. `internal/cli/delegate.go`: wrap review-phase `session.Wait()` with 5-minute timeout via `context.WithTimeout`:
     ```go
     ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ReviewTimeoutSeconds)*time.Second)
     defer cancel()
     ```
  2. `internal/config/config.go`: add `ReviewTimeoutSeconds int` field to `Config` struct with `json:"review_timeout_seconds"` tag; default `300` (5 minutes) in `Default()`
  3. On review timeout: log event `"review_timeout"` to ledger, persist state as `TaskError` + `VerdictRejected`, escalate to Staff via `fmt.Fprintf(a.Err, "Review timed out for issue %d after %ds — escalating to Staff", issueNum, cfg.ReviewTimeoutSeconds)`
  4. Max-cycles escalation: when `round >= maxRounds` after `CHANGES_REQUESTED`, build escalation summary containing original issue body, all review feedback (round number + stderr output), and current git diff. Write summary to `a.Err` with header: `"ESCALATION: Review cycle exhausted for issue %d\n<summary>"`
  5. Existing config tests pass: `go test ./internal/config/`
  6. New config test: `TestConfigReviewTimeoutDefault` verifies default is 300

- [ ] **Tests for review loop paths** — role: sr-dev-be, deps: runDispatchLoop cycle + review prompt + timeout, est: 45m
  1. `internal/cli/delegate_test.go`: extend `fakeAdapter` to support a sequence of results (`results []adapter.SessionResult`) to simulate produce→review→produce→review cycles without re-mocking
  2. `TestReviewLoopApprovedFirstRound`: produce → OK, review → stderr `"APPROVED:"` → verify `task.Status == TaskDone`, `task.Verdict == VerdictApproved`, 2 ledger entries (produce + review events)
  3. `TestReviewLoopChangesRequestedThenApproved`: cycle 1: produce OK, review `"CHANGES_REQUESTED: 1. Missing error handling"` → cycle 2: produce OK (with feedback), review `"APPROVED:"` → verify `task.Status == TaskDone`, `task.Verdict == VerdictApproved`, round counter incremented correctly
  4. `TestReviewLoopMaxCyclesExhausted`: 3 cycles of `CHANGES_REQUESTED` → verify exit with `ClassificationMaxTurns`, `VerdictChangesRequested`, escalation message on `a.Err`
  5. `TestReviewLoopBlockedImmediate`: produce OK, review stderr `"BLOCKED: missing API credentials"` → verify immediate exit, `VerdictRejected`, escalation message
  6. `TestReviewTimeoutEscalation`: fakeAdapter returns a session whose `Wait()` blocks → context timeout triggers escalation path; verify `TaskError` + `VerdictRejected` + error output
  7. `TestBuildReviewPromptOutput`: verify prompt contains issue body, diff, criteria, and all three verdict labels
  8. All tests pass: `go test -count=1 ./internal/cli/`
