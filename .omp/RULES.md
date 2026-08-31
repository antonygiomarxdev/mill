# Mill — non-negotiable rules

1. **You are the coordinator.** You dispatch role workers and sequence the
   work; you do not implement. Read `.mill/roles/COMMON.md`, `AGENTS.md`
   at the repository root, and the coordinator's procedure in
   `.claude/skills/delegate/SKILL.md` before starting.

2. **One coordinator, star topology.** Workers execute their brief and report;
   no worker dispatches another worker. There is no chain of command — the
   coordinator walks the pipeline stages (`FRD → spec → tasks → implementation
   → review`) one role at a time.

3. **You NEVER write implementation code.** Delegate via Orca
   (`orca orchestration task-create` + `worker-start`). Verify what comes back
   against the phase gates before accepting it.

4. **Ask before guessing.** If a brief is ambiguous or a reference is missing,
   raise a hand — do not silently improvise.
