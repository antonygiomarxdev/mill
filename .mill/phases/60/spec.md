# Spec: `mill delegate` must invoke AI with issue-body context

## Architecture

**Problem:** `mill delegate` currently creates a worktree and dispatches a single session with a generic prompt (`buildRolePrompt`). It does not read the issue body from GitHub, does not incorporate acceptance criteria into the prompt, and does not run the produce→review→rework loop that makes delegation useful. The single dispatch classifies the result and stops — no iterative refinement.

**Solution:** Three architectural additions to the delegate flow:

### 1. Issue body reader (`internal/issue/reader.go`)
A new function reads the GitHub issue body via `gh issue view <N> --json body`. This is called early in `runDelegate` and the body text is folded into the prompt. The issue module currently only parses issue numbers (`Parse`/`MustParse`); reading the body is a natural extension in the same package.

### 2. Review loop integration (`internal/cli/delegate.go`)
The `runDispatchLoop` function is extended to support a produce→review cycle:
- **Produce phase:** Dispatch agent A (cheap model) with the issue body as context. Agent A reads acceptance criteria and produces changes.
- **Review phase:** Dispatch agent B (expensive model) with the diff output. Agent B classifies the result (approved / changes-requested). If changes are requested, the cycle repeats up to `max-rounds` (from config).
- **Exit conditions:** Approved → persist, close. Max rounds exhausted → persist with MAX_TURNS verdict. Fatal/Auth/Blocked → persist with error verdict.
- Each round appends a ledger entry with round number, model used, and classification.

### 3. Stage-aware model routing
The `stage:*` label on the issue determines which models to use. The `resolveModel` function already reads role frontmatter; it is extended to consult issue labels:
- `stage:produce` → free model (laguna-free)
- `stage:review` → paid model (laguna-pro)
- `stage:implement` → free → escalate on complexity
- No `stage:*` label → use `--role` default or config.Model

The `gh issue view --json labels` call reads labels alongside the body.

**Data flow:**
```
gh issue view N --json body,labels
        ↓
  buildPrompt(issueBody, targetRole, stageLabel)
        ↓
  produce phase → adapter.Dispatch(freePrompt)
        ↓
  review phase → adapter.Dispatch(paidPrompt)
        ↓
  verdict → persist → done
```

**Constraint:** The review loop logic MUST reuse the existing adapter interface. No new adapter methods. The loop is a CLI-level concern — it calls `a.Adapter.Dispatch()` twice per round with different prompts and models.

## Components affected

| File | Change |
|---|---|
| `internal/issue/reader.go` | NEW: `ReadBody(int) (body string, labels []string, err error)` — shells out to `gh issue view` |
| `internal/issue/reader_test.go` | NEW: Tests for body parsing, label extraction, gh-not-found errors |
| `internal/cli/delegate.go` | MODIFY: `runDelegate` calls `issue.ReadBody`, folds body into prompt, passes labels to `runDispatchLoop`. `runDispatchLoop` gains review-loop logic. `buildRolePrompt` accepts body text. |
| `internal/cli/delegate_test.go` | MODIFY: New tests for review-loop cycle, body-in-prompt, label-aware routing |
| `internal/domain/task.go` | MODIFY: Add `Round int` field to Task for tracking review-loop progress |
| `internal/config/config.go` | No change needed (MaxRounds already exists in mill.yml template) |

### Files NOT affected
- `internal/adapter/` — no changes; the adapter is used as-is for both produce and review dispatches
- `internal/state/` — no schema changes beyond Task.Round field
- `CHECK` scripts — pre-existing gates continue to apply

## Risks

### Risk 1: `gh` CLI dependency for issue body reading
**Severity:** Medium. **Mitigation:** `gh` is already a hard dependency (GitHub issues are the backbone of mill's workflow). If `gh` is absent, `ReadBody` returns a clear error: "gh CLI not found — install github.com/cli/cli". The error propagates to `runDelegate` and surfaces to the user. No fallback — this is a deliberate design choice: mill's delegation model requires GitHub issues.

### Risk 2: Review loop doubles API costs
**Severity:** Medium. **Mitigation:** The `max-rounds` config (default 4) caps iterations. Each iteration uses one cheap model call (produce) and one expensive call (review). At max 4 rounds, that's 8 API calls per task. This matches the bash harness (`loop.sh`) budget. The ledger records every call for cost auditing.

### Risk 3: Review phase may produce contradictory classifications
**Severity:** Low. **Mitigation:** `classifyResult` already handles stderr signals with priority over exit codes. The review prompt instructs the reviewer to emit a classification signal on stderr (`APPROVED:`, `CHANGES_REQUESTED:`, `BLOCKED:`). The classifier maps these to domain verdicts. If the reviewer emits no signal, exit code 0 → APPROVED by default.

### Risk 4: Async goroutine lifecycle (`go runDispatchLoop` + review loop)
**Severity:** Medium. **Mitigation:** The goroutine already tracks state via `state.Save()`. With the review loop, the goroutine persists state after each round (not just at the end), so `mill watch` (#61) can observe progress. The goroutine handles panics internally — any panic is caught, logged as FATAL, and the task is marked errored. The main process (`mill delegate`) returns immediately after spawning the goroutine, consistent with current async behavior.

### Risk 5: Label parsing fragility
**Severity:** Low. **Mitigation:** `gh issue view --json labels` returns a JSON array of label objects with `name` fields. Only `stage:*` labels are parsed; unrecognized labels are ignored. If multiple `stage:*` labels exist, the first one wins with a warning to stderr.

## ADR

**NEW ADR: ADR 0004 — Review Loop as CLI Concern.** The review loop is implemented in `internal/cli/`, not in `internal/adapter/`. Rationale:
- The adapter interface remains provider-agnostic: `Dispatch(opts) → Session`. Adding review-loop logic to the adapter would couple it to mill's workflow model.
- The CLI layer orchestrates multiple dispatches (produce + review) using the same adapter interface. This keeps the adapter thin and testable.
- If a provider later offers native review-loop support (e.g., a "review mode" flag), a new adapter method can be added. Until then, composing two dispatches at the CLI level is simpler and more portable.

**Existing ADRs apply:**
- **ADR 0001** (Mill as Framework): The CLI remains the escape hatch. This spec makes the escape hatch functional.
- **ADR 0002** (Budget Enforcement): The review loop respects `Budget` — each dispatch in the loop checks budget independently. If the budget is exhausted mid-loop, the task is BLOCKED.

## Acceptance criteria

1. `mill delegate <issue>` reads the issue body via `gh` and passes it to the agent
2. `mill delegate <issue>` runs produce→review loop (up to `max-rounds` from config)
3. Review phase uses a different model than produce phase (free→paid escalation)
4. `stage:*` labels on the issue influence model selection
5. Each round is recorded in the ledger with round number and classification
6. Final verdict (APPROVED/CHANGES_REQUESTED/BLOCKED) is persisted in state
7. Async behavior preserved: `mill delegate` returns immediately, goroutine runs to completion
8. `go test ./internal/cli/` passes (existing tests + new loop tests)
9. `go test ./internal/issue/` passes (new reader tests)
