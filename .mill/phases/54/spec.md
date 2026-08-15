# SPEC: Issue #54 — Formal Review Loop

**Phase:** stage:design (Architect SPEC)
**Pipeline:** SPEC → Tech Lead TASKS → Sr Dev IMPLEMENT → Reviewer REVIEW
**PM Spec AC:** (1) auto-review trigger on delegate output, (2) binary APPROVED/CHANGES
verdict, (3) CHANGES triggers rework cycle, (4) max 3 CHANGES → escalation to Staff.

---

## 1. Integration Point: `runDelegate` ↔ Review Loop

### Current State

`delegate.go:runDelegate` (line 30) calls `a.runDispatchLoop(issueNum, taskID, opts, issueBody, labels, cfg)`
at line 204 (sync path) and line 214 (async goroutine). That helper returns `(domain.Classification, error)`.
The caller uses the classification for two purposes:

1. `isIrrecoverable(classification)` — decides whether to `cleanupWorktree` (FATAL/AUTH/NO_CREDIT).
2. Logging the final verdict in the async path is elided (the `_` discard at line 214 is deliberate —
   the goroutine only cares about cleanup).

### Target State

Replace the call with the new `runDispatchLoop54` from `internal/cli/review_loop.go`.
The new loop internalizes all classification logic (it already stores `finalClassification`,
maps it to verdicts, persists state, and prints escalation messages). The caller only needs
to know "do I clean up the worktree or not?"

### Design Decision: Return Signature

`runDispatchLoop54` currently returns only `error`. To support the caller's cleanup decision,
change the signature to return `(domain.Classification, error)`:

```go
func runDispatchLoop54(a *App, issueNum int, taskID string,
    opts adapter.DispatchOpts, issueBody string, labels []string,
    cfg config.Config) (domain.Classification, error)
```

The caller becomes:

```go
// sync path (line 204)
classification, err := a.runDispatchLoop54(issueNum, taskID, opts, issueBody, labels, cfg)
if isIrrecoverable(classification) {
    irrecoverable = true
}
return err

// async path (line 212)
go func() {
    defer a.slots.Release()
    classification, _ := a.runDispatchLoop54(issueNum, taskID, opts, issueBody, labels, cfg)
    if isIrrecoverable(classification) {
        a.cleanupWorktree(issueNum)
    }
}()
```

### Old Code Removal

After integration, delete the old `runDispatchLoop` (delegate.go:227-394), the old
`buildReviewPrompt` (delegate.go:653-671), and the duplicated `recordError` (delegate.go:467-478).
These all have replacements in `review_loop.go`.

The `recordError` in `review_loop.go:308-319` is a method (`func recordError(...)`)
while the one in delegate.go is `func (a *App) recordError(...)`. The integration should
unify on the App method form and move it to a shared location (or keep it in delegate.go
as the canonical copy and remove the package-level duplicate from review_loop.go).

---

## 2. Reviewer Agent Dispatch

### Architecture

The review loop already dispatches two agents sequentially using the same `adapter.Adapter`
interface:

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  runDelegate │────▶│ runDispatchLoop54│────▶│ Adapter     │
│              │     │                  │     │ .Dispatch() │
│  creates     │     │  for round=0..N: │     │             │
│  worktree,   │     │   1. produce     │────▶│  Agent A    │
│  task, slot  │     │   2. review      │────▶│  Agent B    │
└─────────────┘     │   3. classify     │     └─────────────┘
                    │   4. decide       │
                    └──────────────────┘
```

Both produce and review phases use identical dispatch infrastructure: same `DispatchOpts`
struct, same `Session.Wait()` result processing, same `retryDispatch` (for transient errors).
The differences are purely in `Prompt` and `Model`.

### No Adapter Interface Changes Required

The adapter interface is stable. `DispatchOpts` already carries `Prompt`, `Model`,
`Worktree`, `MaxTurns`, and `Budget` — everything the reviewer needs. The reviewer
agent is a conventional session dispatched with a review-specific prompt and
the `laguna-pro` model.

### Model Selection

| Phase    | Model          | Rationale                                         |
|----------|----------------|---------------------------------------------------|
| Produce  | `laguna-free`  | Cheap, fast. Output is work-in-progress.          |
| Review   | `laguna-pro`   | Expensive. Reviews need careful reasoning.        |

Model resolution follows the existing `resolveModel` path with `stage:produce` / `stage:review`
labels. The produce model is resolved from the issue's stage label; the review model is
hardcoded to `"laguna-pro"` (unless a future config key like `review_model` is added).

### Reviewer Prompt Structure

The reviewer prompt (`buildReviewPrompt54`) already follows the right structure.
The SPEC change is to formalize the structured output format (see Section 3).

```
┌──────────────────────────────────────────────────┐
│ buildReviewPrompt54(issueBody, diff, criteria)   │
│                                                   │
│  1. ROLE: "You are a code reviewer..."           │
│  2. ISSUE: markdown body of the GitHub issue      │
│  3. ACCEPTANCE CRITERIA: extracted checklist      │
│  4. CHANGES: produce agent's diff/output          │
│  5. OUTPUT FORMAT: instructions for stderr        │
│  6. QUALITY RULES: no vague feedback, cite        │
│     criteria, APPROVED only if all met            │
└──────────────────────────────────────────────────┘
```

**Change needed:** The output format instructions (lines 274-284) must be updated
to demand specific, numbered, criteria-referencing feedback. See Section 3.

---

## 3. Verdict Model

### Domain Types (Already Exist)

```go
// verdict.go
VerdictApproved  Verdict = "approved"   // All criteria met
VerdictChanges   Verdict = "changes"    // Reviewer requested modifications
VerdictChangesRequested Verdict = "changes_requested"  // Exhausted rework
VerdictRejected  Verdict = "rejected"   // Fatal/auth/no-credit/blocked

// classification.go
ClassificationOK                Classification = "OK"                 // Exit 0 + no signal
ClassificationChangesRequested  Classification = "CHANGES_REQUESTED"  // stderr: "changes_requested:"
ClassificationFatal             Classification = "FATAL"              // Exit 4/9/130/137/143
ClassificationBlocked           Classification = "BLOCKED"            // stderr: "blocked:"
// ... (Auth, NoCredit, RateLimited, Transient, MaxTurns)
```

### Classification Pipeline

```
Reviewer Session
       │
       ▼
session.Wait() → SessionResult{ExitCode, Stderr, Output}
       │
       ▼
classifyResult(exitCode, stderr)
       │
       ├── stderr contains "approved:"          → ClassificationOK
       ├── stderr contains "changes_requested:" → ClassificationChangesRequested
       ├── stderr contains "blocked:"           → ClassificationBlocked
       ├── stderr contains auth signals         → ClassificationAuth
       └── fallback: exit code mapping          → ...
```

The `classifyResult` function (delegate.go:844-889) already handles all the reviewer stderr
signals. No change needed here.

### Structured Reviewer Output Format

The reviewer must output its verdict to stderr in this format:

```
APPROVED:
The implementation meets all acceptance criteria.

--- or ---

CHANGES_REQUESTED:
1. [criterion: "AC title 1"] Specific, actionable feedback referencing this criterion.
2. [criterion: "AC title 2"] Another specific issue.
3. [criterion: "AC title 3"] ...
```

Rules:
- Every CHANGES item MUST reference a specific acceptance criterion by its text.
- "Vague" items (no criterion reference) are invalid — the prompt enforces this.
- APPROVED is the ONLY verdict when all criteria are met. No "APPROVED but note X" hybrids.
- BLOCKED is for external blockers (missing info, dependency unavailable).

The `classifyResult` function matches via substring (`strings.Contains("changes_requested:")`)
so the exact format is flexible — as long as the reviewer emits `changes_requested:` or
`approved:` as the first line of its structured stderr output, classification works.

### Why Not JSON?

The reviewer agent is an LLM. Requiring valid JSON in stderr introduces fragility
(escape errors, trailing commas, truncation). A plain-text header-line protocol
(`APPROVED:` / `CHANGES_REQUESTED:`) is robust to partial output and easy to parse.
The body text (numbered items) is for human/SrDev consumption and not parsed by
the loop — it becomes `reworkFeedback` verbatim.

---

## 4. Rework Cycle

### State Machine

```
                    ┌─────────────────────────────┐
                    │         runDispatchLoop54    │
                    │  round=0, reworkFeedback=""  │
                    └─────────────┬───────────────┘
                                  │
                                  ▼
                    ┌─────────────────────────────┐
                    │      PRODUCE phase           │
                    │  model: cheap (laguna-free)  │
                    │  prompt: original + rework   │
                    │  feedback (if round > 0)     │
                    └─────────────┬───────────────┘
                                  │
                    ┌─────────────▼───────────────┐
                    │  Non-recoverable?            │
                    │  (FATAL/AUTH/NO_CREDIT/      │
                    │   BLOCKED/RATE_LIMITED)       │
                    └──Yes──┬───────────No─────────┐
                            │                      │
                            ▼                      ▼
                      ┌──────────┐    ┌─────────────────────────┐
                      │ ESCALATE │    │      REVIEW phase        │
                      │ (stderr) │    │  model: laguna-pro       │
                      └──────────┘    │  prompt: diff + criteria │
                                      └─────────────┬───────────┘
                                                    │
                                      ┌─────────────▼───────────┐
                                      │  classifyResult()        │
                                      └──┬──────────┬───────────┘
                                         │          │
                               ┌─────────▼──┐  ┌────▼────────────┐
                               │ APPROVED   │  │ CHANGES_REQUESTED│
                               │ (OK)       │  │                  │
                               └─────┬──────┘  └────────┬─────────┘
                                     │                  │
                                     ▼                  ▼
                              ┌────────────┐   ┌──────────────────┐
                              │ DONE       │   │ changesCount++    │
                              │ verdict:   │   │ reworkFeedback =  │
                              │ approved   │   │ reviewResult.Stderr│
                              └────────────┘   └────────┬─────────┘
                                                        │
                                          ┌─────────────▼─────────┐
                                          │ changesCount >= 3?     │
                                          └──Yes──┬───────No──────┘
                                                  │              │
                                                  ▼              ▼
                                          ┌──────────┐   ┌─────────────┐
                                          │ ESCALATE │   │ round++     │
                                          │ to Staff │   │ continue    │
                                          └──────────┘   └─────────────┘
```

### Round Tracking

The `domain.Task` struct already has a `Round int` field. Each iteration of the loop
sets `task.Round = round` (0-indexed). After each round (produce + review), state is
persisted via `s.UpsertTask(task)` and `s.Save(a.statePath())`. This means `mill watch`
and other observers always see the current round number.

### Changes Count vs MaxRounds

**Critical distinction:** The PM spec says "Max 3 rework cycles." This is NOT the same
as `config.MaxRounds`.

- `MaxRounds` (default 4, configurable): total produce+review cycles allowed.
- `EscalationThreshold` = 3: maximum consecutive CHANGES_REQUESTED verdicts before escalation.

Semantics:
- If round 0 gets APPROVED → done.
- If rounds 0, 1, 2 all get CHANGES_REQUESTED → after the 3rd CHANGES, escalate (even if MaxRounds allows more).
- If round 0 gets CHANGES, round 1 gets APPROVED → done (only 1 rework, under threshold).
- If a non-recoverable error occurs mid-cycle → escalate immediately (FATAL/AUTH etc.).

Implementation approach: track `changesCount` (int, local variable, 0 ≤ changesCount ≤ 3).
Increment on each `ClassificationChangesRequested`. When `changesCount >= 3`, break the loop
and escalate — regardless of `round < maxRounds`.

### Acceptance Criteria Extraction

`extractAcceptanceCriteria` parses `- [ ]` markdown checklist items. This is correct
for GitHub issue bodies that use the Task List format. For issues without checklist
items, `buildReviewPrompt54` already falls back to "(no acceptance criteria provided —
evaluate against best practices)". This is acceptable for v1.

---

## 5. Escalation Rule

### Trigger

Escalation fires when:
1. `changesCount >= 3` (three consecutive CHANGES_REQUESTED verdicts, even if MaxRounds allows more cycles), OR
2. A non-recoverable classification occurs (FATAL, AUTH, NO_CREDIT, BLOCKED, RATE_LIMITED).

### Escalation Behavior

When escalation fires, the loop:

1. **Prints** a summary to stderr (`a.Err`) with:
   - Issue number
   - Escalation reason (e.g., "3 changes requested — rework exhausted")
   - Full issue body
   - All review feedback from every round (keyed by round number)

2. **Persists** the task with:
   - `Status: TaskError`
   - `Verdict: VerdictChangesRequested`
   - `Round` set to the last round attempted
   - `UpdatedAt` refreshed

3. **Appends** a ledger entry with:
   - `Event: "complete"`
   - `Status: string(TaskError)`
   - `Verdict: "changes_requested"`
   - `Classification: "CHANGES_REQUESTED"`
   - `Round: task.Round`

4. **Returns** `(ClassificationChangesRequested, nil)` to the caller.

### Staff Notification

The current implementation prints to stderr. This is the Staff terminal (the user who
ran `mill delegate --wait`). For async delegates, the stderr output is discarded by
the goroutine. The primary notification mechanism is **state persistence**: Staff can
run `mill watch` or inspect `.mill/state.json` to see tasks in TaskError status with
verdict changes_requested.

Future enhancement (out of scope for this SPEC): GitHub comment, Slack webhook,
or email notification for escalated issues. The state model already carries all
necessary data for such a feature.

### Worktree Cleanup

On escalation due to changes exhaustion, the worktree is NOT cleaned up — the
implementation output is preserved for Staff to inspect. Cleanup only happens
for irrecoverable classifications (FATAL/AUTH/NO_CREDIT).

---

## 6. Existing Code Assessment

### `review_loop.go:runDispatchLoop54` (lines 33-231)

| Aspect              | Assessment | Disposition |
|---------------------|-----------|-------------|
| Loop structure      | Correct. Produce → review → classify → decide. | **Reuse.** |
| State persistence   | `state.Load`, `UpsertTask`, `state.Save` after each phase. | **Reuse.** |
| Ledger logging      | Entries for produce, review, and complete. | **Reuse.** |
| Classification switch| Handles OK, ChangesRequested, Blocked/Auth/etc. | **Reuse** with minor additions. |
| Escalation output    | `goto escalate` label prints review feedback to stderr. | **Reuse**, format tightened. |
| Feedback propagation | `reworkFeedback = reviewResult.Stderr` on CHANGES. | **Reuse.** |
| `recordError`        | Package-level func, duplicated from delegate.go. | **Delete** this copy; use the App method from delegate.go. |
| Return type          | Returns only `error`. | **Change** to `(domain.Classification, error)` — see Section 1. |
| Changes count        | Uses `round+1 >= maxRounds` to detect exhaustion. | **Replace** with `changesCount >= 3` per PM spec AC #4. |
| Produce retry        | Calls `adapter.Dispatch` directly (no `retryDispatch`). | **Bug.** The old `runDispatchLoop` uses `retryDispatch` which adds exponential backoff on transient failures. The new loop must use `retryDispatch` for both produce and review phases. |

### `review_loop.go:buildReviewPrompt54` (lines 237-287)

| Aspect              | Assessment | Disposition |
|---------------------|-----------|-------------|
| Issue body          | Included verbatim. | **Reuse.** |
| Acceptance criteria | Checklist items or fallback message. | **Reuse.** |
| Diff output         | Produce agent's output included. | **Reuse.** |
| Output format       | Templates for APPROVED/CHANGES_REQUESTED/BLOCKED. | **Update** to enforce numbered, criteria-referencing items. |
| Quality rules       | No vague feedback, cite criteria, APPROVED when all met. | **Reuse.** Enhance: add explicit citation format `[criterion: "..."]`. |

### `review_loop.go:extractAcceptanceCriteria` (lines 292-304)

Simple, functional, no changes needed. **Reuse as-is.**

### `delegate.go:runDispatchLoop` (old, lines 227-394)

**Delete entirely.** All functionality is superseded by `runDispatchLoop54` with
the changes described above. The old loop's produce-output-to-file and enforcement-log
features (lines 267-273, 382-391) should be ported into the new loop if still desired.

### `delegate.go:buildReviewPrompt` (old, lines 653-671)

**Delete.** Superseded by `buildReviewPrompt54`.

### `delegate.go:recordError` (App method, lines 467-478)

**Keep.** This is the canonical copy. Remove the duplicated package-level `recordError`
from `review_loop.go`.

### `delegate.go:classifyResult` (lines 844-889)

No changes needed. Already handles `"approved:"`, `"changes_requested:"`, and all
fault signals. **Reuse as-is.**

### `adapter/adapter.go` (full file)

No changes needed. `DispatchOpts`, `SessionResult`, `Session`, and `Adapter` are all
sufficient for the review loop. **No changes.**

### `domain/verdict.go` and `domain/classification.go`

Complete. No new values needed. **No changes.**

### `domain/task.go`

Task struct already has `Round int`. No changes needed. **No changes.**

### `state/state.go`

Sufficient for persisting round-level state. **No changes.**

### `ledger/ledger.go`

`Entry` already has `Round int`. **No changes.**

### `config/config.go`

`MaxRounds` (default 4, configurable) is unchanged. The escalation threshold (3)
is a hardcoded constant, not configurable in v1. **No changes.**

---

## Summary of Required Changes

| File                    | Change                                                         |
|-------------------------|----------------------------------------------------------------|
| `review_loop.go`        | Add `changesCount` tracking; change return type; use `retryDispatch`; remove `recordError`; port enforcement-log feature; update `buildReviewPrompt54` output format. |
| `delegate.go`           | Replace `runDispatchLoop` call with `runDispatchLoop54`; delete old `runDispatchLoop`, old `buildReviewPrompt`. |
| No other files modified |                                                                 |

## Constants

```go
// EscalationThreshold is the max number of consecutive CHANGES_REQUESTED
// verdicts before the review loop escalates to Staff.
const escalationThreshold = 3
```

When `config.MaxRounds` < escalationThreshold, the loop exits at MaxRounds first
and the verdict is `changes_requested` (same escalation path, different exit cause).
