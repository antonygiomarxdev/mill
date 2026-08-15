# FRD: Review loop — produce, review, rework, approve/reject

## User need

When a role (Sr. Dev, Tech Lead, etc.) produces a deliverable — code, config, spec, documentation — there must be a formal review step before the work is accepted. Currently, Mill delegates work but has no built-in review loop. The Reviewer role exists on paper but has no integration with the delegation pipeline.

Without a review loop, quality gates are the only defense, and they catch mechanical problems (lint, coverage) but not substantive ones (wrong approach, missed acceptance criteria, design drift). The review loop closes this gap.

## Functional requirements

1. **Review phase follows IMPLEMENT.** When a role completes its IMPLEMENT phase and produces a deliverable, the pipeline MUST route the work to a Reviewer before marking it complete. The Reviewer role is always the reviewer — no role reviews its own work.

2. **Reviewer receives full context.** The Reviewer receives: the original issue body with acceptance criteria, the FRD or SPEC (whatever spec document exists), and the complete diff of changes produced. The Reviewer does not guess what was asked for.

3. **Decision: approved or changes-requested.** The Reviewer produces exactly one of two outcomes: (a) APPROVED — every acceptance criterion is met, the implementation matches the spec, and there are no blocking issues; or (b) CHANGES_REQUESTED — the Reviewer enumerates specific, actionable changes needed, each tied to an acceptance criterion.

4. **Changes-requested includes actionable feedback.** Each requested change must reference: which acceptance criterion is not met, what the observed behavior/output is, and what the expected behavior/output should be. Vague feedback ("this doesn't look right") is rejected by the loop mechanism.

5. **Rework loop.** After CHANGES_REQUESTED, the original producer role is re-invoked with the Reviewer's feedback as additional context. The role produces a revised deliverable, which is re-submitted to review. This loop repeats until APPROVED or until 3 cycles — after 3 CHANGES_REQUESTED rounds, the issue escalates to Staff for human intervention.

6. **Approved work advances.** When the Reviewer APPROVES, the pipeline advances the issue to the next stage (e.g., `stage:dev` → `stage:review` → `stage:done`). The approval is recorded as an issue comment with the Reviewer's sign-off.

7. **Review timeout.** If the Reviewer does not respond within 5 minutes, the pipeline escalates to Staff. The review loop never blocks indefinitely.

## Out of scope

- Pair review (multiple reviewers). One Reviewer per deliverable in this phase.
- Automated review beyond existing quality gates. The review loop evaluates substance, not syntax.
- Reviewer workload balancing. The Reviewer role handles one review at a time; queuing is future work.
- Review of specs/FRDs. This loop is for implementation deliverables (code, config, docs). Spec review is a separate PM/Architect workflow.

## Priority

**P0** — blocks quality assurance. Without a review loop, there is no human-in-the-loop verification that implementers actually built what the spec asked for. The Reviewer role is defined but dead code.
