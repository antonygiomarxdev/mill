# Spec: PM Assessment — Mill adoption gaps

## Architecture

**Problem:** Mill lacks a systematic, ranked gap analysis that feeds the backlog. The CTO cannot prioritize because the gaps are scattered across issues, Slack messages, and session notes. PM needs to produce an evidence-based assessment that ranks every gap by impact and provides implementation cost estimates.

**Solution:** This is a **research deliverable**, not a code change. The architect's role is to define the structure and quality standards for the assessment, then delegate to PM for execution. The assessment follows the existing research pattern (`docs/research/`) and produces a ranked gap list that directly feeds `.mill/map.json` + new issues.

### Assessment structure

The deliverable is `docs/research/mill-adoption-gaps.md` with these sections:

1. **Methodology** — how gaps were discovered (issue analysis, session notes, code review, UX walkthrough). What constitutes P0/P1/P2.
2. **P0 Gaps** (blockers) — each with: gap description, why it blocks, current workaround, estimated agent-minutes to fix, dependent issues/PRs. Minimum coverage: `mill init` robustness, `mill land` lock handling, missing OpenCode adapter (ADR 0001 deleted the stub), gauntlet hook auto-install in worktrees.
3. **P1 Gaps** (fragility) — each with: what breaks, how often, blast radius, estimated agent-minutes. Minimum coverage: missing `mill.yml` validation (panics on invalid YAML), missing binary checks (`git`, `cmd`), no README/setup docs, no error recovery for partial state.
4. **P2 Gaps** (polish) — each with: what's missing, user impact, estimated agent-minutes. Minimum coverage: remaining ROLE.md files, `mill sync-skills` stub, `mill clean` stub, no CI/CD pipeline.
5. **Implementation order** — ranked list with dependency graph. P0 first, then P1, then P2. Within each tier, dependencies determine order.

### Integration with backlog

The assessment directly feeds new issues:
- Each P0 gap → new GitHub issue with `stage:backlog` + `agent:architect` labels
- Each P1 gap → new GitHub issue with `stage:backlog` + `agent:architect` labels
- P2 gaps are documented but not auto-filed as issues (CTO decides)
- `.mill/map.json` updated with new issue references

## Components affected

| File | Change |
|---|---|
| `docs/research/mill-adoption-gaps.md` | NEW: Full assessment document |
| `.mill/map.json` | MODIFY: Add new gap issues |

No code files changed. This is a research deliverable.

## Risks

### Risk 1: Assessment is biased by PM's recent session experience
**Severity:** Medium. **Mitigation:** The FRD requires systematic coverage across all Mill components, not anecdotal reports. The assessment must cite specific issues, code paths, and session notes — not "I felt this was slow." The Architect reviews the assessment against the source codebase before accepting.

### Risk 2: Agent-minute estimates are wrong
**Severity:** Low. **Mitigation:** The FRD explicitly requires estimates to be ≤3 distinct values (e.g., 15m, 60m, 120m). This binned precision is "roughly right" — it avoids false precision. Actual costs will vary by model and complexity; the estimates are for prioritization, not budgeting.

### Risk 3: Assessment duplicates existing issue content
**Severity:** Low. **Mitigation:** The assessment is a *summary + ranking*, not a copy. It links to existing issues for detail and adds only: impact assessment, cost estimate, and dependency analysis. The summary format is designed to be read in ≤5 minutes by the CTO.

## ADR

No new ADR. This is a research deliverable — not a system design decision. The assessment itself may trigger ADRs for specific gap solutions.

## Acceptance criteria

1. `docs/research/mill-adoption-gaps.md` exists with all 5 required sections
2. Minimum P0 gaps covered: init robustness, land lock handling, OpenCode adapter stub
3. Minimum P1 gaps covered: config validation, binary checks, README docs
4. Minimum P2 gaps covered: ROLE.md stubs, sync-skills, clean, CI/CD
5. Every gap has an estimated agent-minutes cost (≤3 distinct values)
6. Every gap has a "why it matters" rationale
7. Implementation order section exists with dependency graph
8. `bash checks/gate-spec 28` passes
