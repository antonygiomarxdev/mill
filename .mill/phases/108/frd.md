# FRD: Role Learning Feedback Loop
## User need
When a subagent produces work that is later found defective — either in pre-merge review (a reviewer flags a defect in the diff) or in production (a bug ships and is discovered later) — the fix is applied but the producing role never learns from it. The same class of mistake recurs on subsequent delegations because there is no mechanism to capture the incident, attribute it to the role that produced it, and feed a distilled lesson back into that role's context on its next invocation. Staff end up repeatedly explaining the same things, and institutional memory is lost between sessions.

## Functional requirements
1. **Detection point — Diff (review stage):** When a reviewer classifies a diff as `CHANGES_REQUESTED` or `NOT_APPROVED` before merge, the producing role (e.g., senior-dev, designer) is automatically flagged as the subject of a new lesson candidate.
2. **Detection point — Production bug:** When a bug is filed/observed in production, it is attributed to the role responsible for the work — implementation bug → sr-dev; design/definition bug → pm/designer; documentation bug → qa-docs/docs role.
3. **Implicit signal capture:** Lessons are derived automatically from implicit signals — reviewer verdict (APPROVED/CHANGES), pre-push/coverage gate failure, and rework (a task re-assigned after rejection) — rather than relying on manual lesson authoring as the primary mechanism.
4. **Automatic distillation:** Each confirmed defect is distilled into a single one-line lesson of the form "In `<context>`, do X not Y, because Z" and appended to the responsible role's `lessons.md` file (e.g., `.mill/roles/<role>/lessons.md`).
5. **Confirmation gate:** A lesson is only recorded once a defect is confirmed as actually merged and fixed (for production) or once the diff is merged after rework (for review-stage findings), to avoid recording spurious findings from aborted changes.
6. **Context injection:** On the next delegation to that role, the role's `lessons.md` is automatically loaded into the agent's context (via `ROLE.md` or a learned-lessons hook) so the subagent is aware of prior pitfalls.
7. **Staff curation:** Lessons are curated by Staff (not blindly auto-applied) — staff may review, merge duplicates, reword, or reject one-off/overfit lessons before they persist into a role's `lessons.md`.
8. **Per-role accumulation:** Each role maintains its own append-only `lessons.md` accumulating distilled learnings, serving as institutional memory scoped to that role's responsibilities (composing taste-1 "user style" with role process memory).
9. **Real case coverage (Rumai #406):** The loop must capture the pattern class where "a primitive consumes an interaction token from `useTheme()` but the hook does not return it; tests mock it, production crashes" — demonstrating that the loop generalizes from a concrete incident to a transferable lesson.
## Out of scope
- Manual, free-form lesson writing as the primary input mechanism (manual notes may supplement but do not drive the loop).
- Auto-applying lessons as hard enforcement gates (e.g., blocking delegation until a lesson is "satisfied") — lessons are advisory context, not hard validators.
- Cross-role lesson propagation (a bug in sr-dev's work is learned by sr-dev only, not automatically pushed to every other role; Staff mediates broader sharing).
- Per-session taste learning of user coding style (taste-1) — that remains the separate, existing taste mechanism; this FRD concerns role-level process memory.
- Reverting or auto-deleting lessons once recorded — curation is additive (reword/reject before persistence), not post-hoc removal.
## Priority
**P1** — high value and directly addresses the repeated-mistake feedback gap that staff currently close manually; foundational for role reliability but does not block current launch milestones.
**P0** — *if* a production regression of the Rumai #406 class recurs within this quarter, this loop should be fast-tracked to P0 to prevent repeat incidents.
