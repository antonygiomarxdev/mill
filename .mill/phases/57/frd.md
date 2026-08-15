# FRD: Auto-compact via `--config compact-mode=fast`

## User need

Long Mill sessions exhaust the AI model's context window. As the conversation grows — issue bodies, code snippets, tool outputs, agent dialogue — the model eventually hits its context limit and loses earlier instructions. The CTO or subagent must manually summarize or restart, losing state and wasting tokens.

Mill must support automatic context compaction so that sessions can run indefinitely without manual intervention. The compaction preserves essential state (current task, active role, recent decisions) while discarding stale tool output and resolved sub-tasks.

## Functional requirements

1. **Configuration flag.** `mill delegate --config compact-mode=fast` enables auto-compaction for that delegation. The flag is also settable in `mill.yml` under a `compact` key so it can be the default for all delegations.

2. **Compaction trigger.** Compaction triggers when the session context reaches 80% of the model's context window limit. Mill tracks estimated token usage and fires compaction before the model hits its hard limit — not after.

3. **What is preserved.** After compaction, the compacted context retains: (a) the original delegation prompt and acceptance criteria, (b) the active role and its capability boundaries, (c) the last 3 agent turns (actions + outputs), (d) any unresolved issues or blocking items, and (e) the current working state (open files, active phase).

4. **What is discarded.** Compaction drops: (a) tool outputs older than the last 3 turns, (b) completed sub-agent dialogue, (c) speculative exploration that produced no changes, and (d) error messages from resolved failures.

5. **Compaction is lossy by design.** The compaction summary replaces discarded context with a structured summary: "Explored X, found Y, decided Z." This is sufficient for continuity, not for reproducing every step. Full session history remains in logs.

6. **Compaction log.** Every compaction event is logged with: timestamp, pre-compaction token count, post-compaction token count, and tokens saved. Written to `.mill/compact.log` as JSONL.

7. **Manual compaction.** `mill compact` triggers compaction immediately regardless of context usage. Useful for cleaning up before a handoff or long wait.

## Out of scope

- Intelligent summarization (semantic compression). Compaction is structural — drop old, keep recent + state. No LLM-powered summarization of discarded content.
- Cross-session compaction. This is within a single session.
- Token-counting precision. Estimates are based on character count / 4 (rough token estimate), not provider-specific tokenizers.
- Compaction of logs or persisted files. Only in-memory session context is compacted.

## Priority

**P2** — quality of life. Context exhaustion is a real problem for long sessions, but Mill sessions today are typically short enough to fit. This becomes P1 when session length grows.
