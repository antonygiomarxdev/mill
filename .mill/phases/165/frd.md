# FRD: Automated evaluation of delegation results

**Issue:** #165  
**Roadmap:** Item 5 — Evals

## User need

A coordinator reviews a delegation's output. Today they read the diff and decide. In the session that produced `docs/FINDINGS-2026-08.md`, that human accepted a report claiming alignment while three sections still contradicted the decision — and separately claimed a file "never existed" when it was merely untracked.

The research finds every source treating evals as foundational and warning they get harder to build the longer you wait.

The need: a delegation's result is judged without a human reading it, for the failure classes that have already occurred. The human reviews what the graders flag, not everything.

## Functional requirements

1. Computational graders exist for failures already documented in `docs/FINDINGS-2026-08.md`:
   - Phase artifact exists with required sections
   - Tree builds; tests pass
   - Declared tier matches dispatched tier (role frontmatter vs dispatch receipt)
   - Diff is non-empty (prevents approval of no-change)
   - Process exited zero
   - Non-leaf role dispatched downstream (chain ran, not just first step)
2. Graders are bash scripts in `.mill/checks/eval-*`, runnable standalone.
3. Graders run in CI on every push, not only on demand.
4. A grader failure names which grader failed and what it found.
5. Graders are deterministic — same input produces same output.

## Out of scope

- Model-as-judge grading (spec answers FRD, review caught what it should). Deferred — requires prompt engineering and calibration.
- Grading subjective quality (code style, design elegance). Computational graders test failure modes, not excellence.
- Grading tasks outside the classes documented in FINDINGS. Build from known failures; expand as new failures occur.

## Priority

**P1 — makes it good.**

Requires items 1, 2, 4 (#152, #162, #164) to hold. If the chain is not worth it (#164), evals for the chain are moot.

Refs #159 (supervision), #110 (learning from defects).

## Acceptance criteria

1. `ls .mill/checks/eval-* | wc -l` ≥ 5 — at least five graders exist
2. Each grader is executable and exits 0 on valid input — `for f in .mill/checks/eval-*; do bash "$f" --help || exit 1; done` passes
3. `grep -r 'eval-' .github/workflows/` shows graders referenced in CI
4. CI run with failing grader shows grader name and finding in output — simulate failure, check CI log
5. `bash .mill/checks/eval-artifact-exists 162` returns 0 when `.mill/phases/162/frd.md` exists
6. `bash .mill/checks/eval-build` returns non-zero on a tree that fails `go build ./...` (or equivalent)
7. `bash .mill/checks/eval-tier-match <dispatch-receipt>` returns non-zero when receipt tier ≠ role frontmatter tier
8. Running same grader twice on same input produces identical output — `diff <(bash .mill/checks/eval-build) <(bash .mill/checks/eval-build)` is empty
