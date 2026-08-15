# FRD: First run survives without its author

**Issue:** #163  
**Roadmap:** Item 3 — Survive the first run without its author

## User need

A new coordinator dispatches their first worker. They do not have eight hours of context about Mill's failure modes, they have not read `docs/FINDINGS-2026-08.md`, and they are not watching the worker's terminal.

In one session with the author present, four failures occurred silently:
- A brief landed unsubmitted — worker never started, `task-list` showed nothing wrong
- A provider connection error surfaced nowhere — worker died, status still read `dispatched`
- `check --ack` without a delivery id acknowledged nothing — spurious "you have messages" forever
- A manual `task-update` after `worker_done` left the task `blocked`

Three were usage errors. The tool permitted them without feedback. A new user hits all four on day one and leaves.

The need: when something goes wrong, it says so where it happens, and says what to do.

## Functional requirements

1. A dispatch that parks (brief never submitted) is detected within 60 seconds and surfaced to the coordinator.
2. A dispatch that dies (worker exited non-zero, or connection error) is detected and surfaced with the terminal output that explains it.
3. A coordinator action that changes nothing (`--ack` without delivery, `task-update` after settlement) either errors or warns before executing.
4. A preflight check runs before dispatch — verifies Orca reachable, agent registered, worktree clean — and blocks if any fails.
5. Every surfaced failure includes what to do next, not only what went wrong.

## Out of scope

- Fixing Orca upstream issues (e.g., #14505 brief-lands-unsubmitted). Mill can detect and report; it cannot fix the substrate.
- Automatic recovery from failures. Detect and explain; do not auto-retry or auto-fix.
- Failures that occur deep in a task. This FRD covers the first five minutes — dispatch and initial execution.

## Priority

**P0 — blocks usability.**

A product that silently fails is not a product. This is the difference between "it worked for the author" and "it can work for anyone."

Refs #152 — installation (#162) must work before this matters.

## Acceptance criteria

1. A parked dispatch (brief unsubmitted >60s) produces a message to the coordinator — simulate by starting dispatch and not submitting, observe message
2. A dead dispatch (worker exit non-zero) produces a message within 30s — kill worker process, observe message
3. `orca orchestration check --ack` with no pending delivery returns non-zero exit code
4. `orca orchestration task-update --status completed` on a settled task returns non-zero or prints warning
5. `mill preflight` (or equivalent) returns non-zero when Orca is unreachable — stop Orca, run preflight, observe exit code
6. `mill preflight` returns non-zero when agent not registered — use bogus agent id, observe exit code
7. Every failure message includes a "next step" line — `grep -c 'next:' <failure-output>` ≥1
