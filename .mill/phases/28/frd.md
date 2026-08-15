# FRD: PM Assessment — Mill adoption gaps

## User need

Mill is a multi-agent delegation framework in early development. Before investing further in features, the CTO needs a clear, evidence-based assessment of what blocks real adoption today. The assessment must prioritize gaps by impact — what prevents someone from starting, what makes usage fragile, and what is polish.

This is a research-and-recommend deliverable, not a feature spec. The output is a ranked gap list that feeds the backlog.

## Functional requirements

1. **P0 gaps identified.** The assessment enumerates every gap that blocks a new user from starting or finishing a Mill workflow. Minimum: `mill init` (project setup), `mill land` (merge to target), and provider adapter diversity (currently single-provider: CommandCode only, no OpenCode adapter per ADR 0001 deletion). Each P0 gap includes: what is missing, why it blocks, and the minimum viable implementation scope.

2. **P1 gaps identified.** The assessment enumerates fragility gaps — things that work but break easily. Minimum: gauntlet hooks not auto-installed in worktrees, no config validation (panics on invalid `mill.yml`), no error handling for missing binaries (`git`, `cmd`), and no documentation (no README explaining setup, usage, roles, workflow).

3. **P2 gaps identified.** The assessment enumerates polish gaps. Minimum: remaining ROLE.md files for PM, Architect, Tech Lead, UX, UI, Reviewer, QA/Docs; `mill sync-skills` stub; `mill clean` stub; no CI/CD for Mill itself.

4. **Each gap has estimated agent-minutes.** Every gap includes a rough implementation cost in agent-minutes (not person-hours — these are AI agent tasks). Estimates must be ≤ 3 distinct from each other (no false precision).

5. **Recommendation ordered by impact.** The assessment concludes with a recommended implementation order: P0 first, then P1, then P2. Within each priority tier, gaps are ordered by dependency — what must be built first to unblock the next item.

6. **Gaps are independently delegable.** Every gap is scoped so that a single subagent can implement it without coordinating with another gap's implementation. Gaps that have cross-dependencies are split or called out.

## Out of scope

- Implementation of any gap. This is assessment only.
- Architecture or design decisions for how to fix gaps. That is the Architect's work on individual gap tickets.
- Cost analysis of provider usage. This is about adoption blockers, not runtime costs.
- Competitive analysis of other delegation frameworks. This is about Mill's own gaps, not market positioning.

## Priority

**P1** — feeds the backlog. The CTO cannot prioritize what they cannot see. This assessment turns "we have problems" into "here are the 12 gaps, ranked, with costs."
