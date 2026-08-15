# FRD: Experiment — Does the delegation chain beat a single agent?

**Issue:** #164  
**Roadmap:** Item 4 — Measure whether the chain is worth it

**This is an experiment, not a feature.** It does not build anything. It measures whether Mill's central premise holds.

## User need

The CTO needs evidence before building more.

Mill's thesis: a chain of specialized roles (PM → Architect → Tech Lead → Sr Dev → Reviewer) produces better work than a single agent given the same task. The research in `docs/research/harness-engineering-and-evals.md` finds the field arguing the opposite for coding — fewer parallelizable components, shared context needed, conflicting implicit decisions.

The chain has never been measured against the alternative. Its context fragmentation is real and unquantified. This experiment answers whether the roadmap after item 4 is worth pursuing.

## Functional requirements

1. The experiment runs the same task twice: once through the full chain, once as a single agent with complete brief.
2. The task has real substance — a feature with a design decision, not a rename or mechanical change.
3. Each arm runs three times to capture variance (agents are non-deterministic on identical prompts).
4. Measurement is: quality (passes acceptance criteria as judged by someone who did not run it), cost (tokens and money), time (wall clock), rework (round trips before acceptance).
5. The result is written to `docs/` regardless of outcome.

## Measurement definitions

- **Quality:** binary — did the output pass its acceptance criteria, unmodified, on first submission to a blind judge? Count: passes out of 3 runs.
- **Cost:** sum of input + output tokens across all dispatches in the arm, converted to USD at listed rates.
- **Time:** seconds from first dispatch to landable result (passes quality check).
- **Rework:** count of round trips (dispatch → feedback → re-dispatch) before acceptance.

## What each outcome means

| Outcome | Definition | Action |
|---------|------------|--------|
| Chain clearly better | Chain wins on quality (≥2/3 vs ≤1/3) AND cost within 2× | Proceed with roadmap; premise holds |
| Single agent matches | No significant quality difference (both ≥2/3 or both ≤1/3) | Mill is overhead with good documentation. Decide if org structure is worth it for reasons other than output quality. Write ADR. |
| Single agent better | Single wins on quality (≥2/3 vs ≤1/3) OR matches quality at <50% cost | The org-chart premise does not hold. Write ADR. Roadmap items 5–7 are deprioritized pending redesign. |

## Out of scope

- Running the experiment. This FRD defines what to measure; Architect specs the protocol, execution happens elsewhere.
- Multiple task types. Pick one representative task. Breadth comes from running the experiment again if the result is close.
- Changing Mill based on results. This FRD produces a measurement. Decisions flow from it but are not part of it.

## Priority

**P0 — can end the project.**

A day's work that answers whether the rest is worth building. Roadmap items 5–7 assume the chain is valuable. This validates or invalidates that assumption.

Refs #157 — related to supervision, but this experiment does not require supervision to run.

## Acceptance criteria

1. Task selected and documented — `test -f docs/experiment-164/task.md` returns 0
2. Three runs per arm completed — `ls docs/experiment-164/runs/ | wc -l` = 6
3. Each run records tokens, cost, time, rework — `grep -c 'tokens:' docs/experiment-164/runs/*.md` = 6
4. Quality judged by someone who did not run it — `grep 'judged_by:' docs/experiment-164/results.md` names a person other than executor
5. Results written — `test -f docs/experiment-164/results.md` returns 0
6. Outcome classified per table above — `grep -E 'Outcome: (chain_better|single_matches|single_better)' docs/experiment-164/results.md` matches
7. If outcome ≠ chain_better, ADR written — `test -f docs/adr/00*-experiment-164-*.md || grep 'chain_better' docs/experiment-164/results.md` returns 0
8. PRODUCT.md corrected if premise does not hold — `git diff docs/PRODUCT.md` shows changes if outcome ≠ chain_better
