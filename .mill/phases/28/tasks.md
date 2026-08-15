# Tasks: PM Assessment — Mill adoption gaps

Research deliverable (no code changes). Outputs: `docs/research/mill-adoption-gaps.md`, new GitHub issues, updated `.mill/map.json`.

## Wave 1 (sequential — gap discovery)

- [ ] **Systematic gap discovery** — role: pm, deps: none, est: 120m
  1. Scan all open GitHub issues for referenced gaps, blockers, and frustrations; extract every concrete gap with issue links
  2. Review `docs/sessions/` directory for session notes mentioning failures, workarounds, or missing features
  3. Walk every Mill CLI subcommand (`init`, `land`, `delegate`, `role`, `sync-skills`, `clean`) against source in `internal/cli/` — note stubs, panics, missing validation
  4. Audit `internal/domain/`, `internal/state/`, `internal/adapters/` for error recovery gaps, nil-pointer risks, missing fallback paths
  5. Check `.mill/roles/` for completeness: every role with a ROLE.md has `model:` and `skills:` populated; note stubs
  6. Review `ADR/` directory: note any deleted adapter stubs (per ADR 0001 — OpenCode adapter) that leave the system incomplete
  7. Compile raw gap list: each gap has a one-line description, category (robustness / docs / feature gap / config / CI/CD), source evidence (issue #, session date, code path), and current workaround if any
  8. Output: `local://28-gaps-raw.md` with the full raw list for ranking in next task

## Wave 2 (depends on Wave 1)

- [ ] **Gap ranking by impact and cost** — role: pm, deps: systematic gap discovery, est: 90m
  1. Classify each gap as P0 (blocker — cannot ship without), P1 (fragility — breaks in production), or P2 (polish — nice to have)
  2. Assign impact rationale: "why it matters" in ≤3 sentences per gap with concrete consequences (e.g., "init fails silently → user thinks Mill is broken and abandons adoption")
  3. Assign agent-minutes cost using ≤3 binned values (15m, 60m, 120m). Estimates reflect developer + reviewer time, not model inference cost. Binned precision avoids false accuracy
  4. Identify dependencies between gaps: which P0 gaps must be resolved before which P1 gaps? Which gaps share code paths?
  5. Produce implementation order: P0 → P1 → P2, with dependency edges within each tier. Include a dependency diagram (Mermaid graph or ASCII) showing critical path
  6. Output: `local://28-gaps-ranked.md` with ranked list, costs, dependencies, and implementation order

## Wave 3 (depends on Wave 2)

- [ ] **Write `docs/research/mill-adoption-gaps.md`** — role: qa-docs, deps: gap discovery + gap ranking, est: 60m
  1. **Methodology section**: document how gaps were discovered (issue analysis, session notes, code review, CLI walkthrough). Define P0/P1/P2 criteria. List sources consulted
  2. **P0 Gaps section**: minimum coverage — `mill init` robustness (missing binary checks, `go.mod`/`.git` validation, partial init recovery), `mill land` lock handling (stale lock files, concurrent land, lock-on-non-repo), missing OpenCode adapter (ADR 0001 deleted stub but no replacement). Each gap: description, why it blocks, current workaround, estimated agent-minutes, dependent issues/PRs
  3. **P1 Gaps section**: minimum coverage — `mill.yml` validation (panics on invalid YAML, no schema enforcement), missing binary checks (`git`, `gh` CLI not verified at startup), no README/setup docs, no error recovery for partial state (corrupt `state.json`, orphaned worktrees). Each gap: what breaks, how often, blast radius, estimated agent-minutes
  4. **P2 Gaps section**: minimum coverage — remaining ROLE.md stubs, `mill sync-skills` stub, `mill clean` stub, no CI/CD pipeline. Each gap: what's missing, user impact, estimated agent-minutes
  5. **Implementation order section**: ranked list with dependency graph (Mermaid). P0 first, then P1, then P2. Within each tier, dependencies determine order. Include critical path callout
  6. Every gap has an estimated agent-minutes cost (≤3 distinct values) and a "why it matters" rationale
  7. Document is designed to be read in ≤5 minutes by CTO; summary-first, detail-inline, links to source evidence
  8. Write to `docs/research/mill-adoption-gaps.md`

## Wave 4 (depends on Wave 3)

- [ ] **Create backlog issues from P0/P1 gaps** — role: pm, deps: assessment document, est: 45m
  1. For each P0 gap, create a GitHub issue with title `"Gap: <gap description>"` and body linking to the assessment doc section
  2. Label each P0 issue: `stage:backlog`, `agent:architect`, `priority:p0`
  3. For each P1 gap, create a GitHub issue with title `"Gap: <gap description>"` and body linking to the assessment doc section
  4. Label each P1 issue: `stage:backlog`, `agent:architect`, `priority:p1`
  5. P2 gaps are documented in the assessment but NOT filed as issues (CTO decides later)
  6. Record all created issue numbers in `local://28-gap-issues.json` for map.json update

- [ ] **Update `.mill/map.json`** — role: pm, deps: backlog issues created, est: 15m
  1. Add each P0 and P1 gap issue to `mill-framework.issues` with `phase: "backlog"`, `status: "pending"`, `next: "Architect → SPEC"`
  2. Mark issue 28 phase as `"complete"` with `next: "Architect → REVIEW"`
  3. Ensure `next` field on `mill-framework` reflects current state: remove 28 from pending TASKS, add new gap issues
