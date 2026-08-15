## Architecture

The Role Learning Feedback Loop is a four-layer pipeline: **Signal Detection → Distillation Engine → Curation & Persistence → Context Injection**, gated by a **Confirmation Gate** state machine that prevents spurious lessons from aborted or unresolved changes.

**Signal Detection Layer** monitors two implicit-signal sources:
1. **Review-stage classifier**: Hooks into the existing review-loop (`/review_loop.go`). When a reviewer verdict resolves to `CHANGES_REQUESTED` or `NOT_APPROVED`, it emits a *defect candidate* event tagged with the subagent's role (from delegation metadata) and the diff classification.
2. **Production-bug tracker**: Integrates with the bug-issue store. When a bug is filed and attributed (by staff or by the role-attribution mapper), it emits a defect candidate tagged to the responsible role (implementation → sr-dev, design/definition → pm/designer, documentation → qa-docs/docs).
3. **Rework detector**: Tracks task re-assignment after rejection — a second delegation of the same task to (possibly the same) role following a failed review signals rework.

**Distillation Engine** consumes confirmed defects. A distiller module (LLM-caller over Claude) transforms each confirmed incident into a one-line lesson: *"In `<context>`, do X not Y, because Z."* A **role-attribution mapper** resolves which role's `lessons.md` receives the lesson — derived from the work type of the rejected/flagged change. The Rumai #406 class is captured as: *"In hooks that consume an interaction token, always return the result so callers receive the value — tests that mock `useTheme()` return silently in production and crash the component."*

**Confirmation Gate** is a two-state state machine per defect candidate (Pending→Confirmed) with type-specific criteria:
- Review-stage: transitions to Confirmed only when the diff is **merged after rework** (not on initial rejection, avoiding false positives from aborted PRs).
- Production: transitions to Confirmed only when the bug is **marked fixed and the fix commit is merged**.

Only Confirmed candidates enter the Curation & Persistence Layer.

**Curation & Persistence Layer** maintains an append-only per-role **curation queue** (`.mill/roles/<role>/queue.md`) of confirmed lesson candidates. Staff review is the gate: they may approve, reword, merge duplicates (fuzzy-match on lesson similarity), or reject one-off/overfit lessons. Upon approval, the distilled lesson is appended to `.mill/roles/<role>/lessons.md`.

**Context Injection Layer** modifies the existing delegation path (`delegate.go`). Before invoking a subagent for a role, the loader reads that role's `lessons.md` and prepends it into the agent's system context (via `ROLE.md` include or a learned-lessons hook), making prior pitfalls advisory visible. Lessons are loaded as plain text — not as hard enforcement gates.

## Components affected

| Component | Change | Description |
|---|---|---|
| `internal/cli/review_loop.go` | modify | Add verdict hooks emitting defect-candidate events on `CHANGES_REQUESTED`/`NOT_APPROVED` with role attribution from delegation metadata |
| `internal/cli/delegate.go` | modify | Add rework detection (re-delegation after failed review) and lesson-context injection (read `<role>/lessons.md`, prepend to agent context) before role invocation |
| `internal/cli/routing_56.go` | modify | Route defect-candidate events through a confirmation-gate state machine with Pending→Confirmed transitions keyed to merge-after-rework (review) and fix-merged (production) |
| New: `internal/learning/distiller.go` | create | Distillation engine — transforms confirmed defects into one-line lessons via LLM call; includes role-attribution mapper |
| New: `internal/learning/confirmation_gate.go` | create | State machine and persistence for defect-candidate lifecycle; enforces type-specific merge gates |
| New: `internal/learning/curator.go` | create | Staff curation workflow — reads `.mill/roles/<role>/queue.md`, applies approve/reword/merge/reject; appends to `lessons.md` |
| New: `internal/learning/injector.go` | create | Context-injection loader — reads role `lessons.md`, formats for `ROLE.md` include or learned-lessons hook |
| New: `cli` subcommand | create | Staff-facing `mill curate-lessons <role>` command to review/merge/reword/reject pending candidates in the curation queue |
| `.mill/roles/<role>/lessons.md` | create | Append-only per-role institutional memory file (all current roles) |
| `.mill/roles/<role>/queue.md` | create | Per-role curation queue for confirmed-but-unreviewed lesson candidates |

## Risks

- **Overfitting from one-off incidents**: A single unusual bug could generate a narrow lesson that reduces signal quality. Mitigated by Staff curation (merge/reject) and the append-only model — rejected lessons never persist.
- **False role attribution**: A bug's root cause may span roles (e.g., a design+implementation gap). The mapper attributes to the *type* of work, which may misassign credit. Staff curator can re-target before persistence.
- **Spurious lessons from aborted changes**: A diff rejected then abandoned would pollute the loop. Mitigated by the Confirmation Gate — only lessons from diffs merged *after rework* persist.
- **Distillation quality degradation**: LLM-based distillation may produce vague or duplicated lessons ("be careful with tokens"). Mitigated by Staff rewording and fuzzy-duplicate merging in the curation queue.
- **Context bloat on injection**: Accumulating lessons.md grows unbounded, inflating every role-delegation prompt. Mitigated by keeping lessons to single lines and allowing Staff to curate down; future work may add recency-weighted trimming.
- **Staff curation bottleneck**: If Staff don't regularly review the curation queue, lesson latency grows and the loop stalls. The `mill curate-lessons` subcommand and queue-md format make batch review feasible, but human throughput is the limiting factor.
- **Production-bug attribution lag**: Bugs discovered in production and attributed to a role weeks later create a delayed feedback loop. The confirmation gate (fix-merged) further delays lesson recording, but this is intentional to avoid false attribution on reverted fixes.
- **Lesson staleness**: A role's `lessons.md` reflects historical pitfalls that may no longer apply as the codebase evolves (e.g., a legacy hook pattern). Mitigated by Staff curation, but no automated expiry exists — lessons are never auto-deleted per spec.