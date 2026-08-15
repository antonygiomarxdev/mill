# FRD: Slot/concurrency management to prevent model contention

## User need

When Mill spawns many parallel subagents (e.g., `mill delegate` fans out to 5 Sr. Devs), all of them compete for the same AI provider's API. This causes rate limiting, increased latency, and in extreme cases, failed delegations because the provider rejects concurrent requests above a quota.

Mill must limit concurrent subagents to a configurable maximum so that the provider's rate limits are respected and no delegation fails due to contention.

## Functional requirements

1. **Configurable slot limit.** `mill.yml` supports a `concurrency` key with a `max-slots` integer. When set, Mill never runs more than `max-slots` subagents concurrently. Default: 4.

2. **Slot acquisition.** Before spawning a subagent, Mill acquires a slot. If all slots are occupied, the delegation is queued — not rejected. The delegating parent is notified: "Delegation to Sr.Dev queued (3/4 slots occupied, position 2)."

3. **FIFO queue.** Queued delegations are processed in first-in-first-out order as slots free up. A queued delegation that waits longer than 2 minutes is escalated with a warning to the parent: "Delegation to Sr.Dev waiting 120s (position 1)."

4. **Slot release.** A slot is released when the subagent yields its final result — not when it finishes processing internally. A subagent that hangs or times out must release its slot so the queue can proceed.

5. **Priority bypass.** `mill delegate --priority` allows a delegation to jump the queue. This is a CTO/Staff privilege. A priority delegation preempts the next available slot.

6. **Slot status visibility.** `mill slots` prints the current state: total slots, occupied slots, queued delegations with wait times, and which roles are in which slots. Useful for debugging contention.

7. **Global vs. per-provider limits.** The `max-slots` limit applies globally across all providers. Per-provider limits (e.g., "max 2 Anthropic, max 2 OpenAI") are deferred to a follow-up.

## Out of scope

- Per-provider slot limits. Global only in this phase.
- Dynamic slot adjustment based on provider load. Slots are static, configured once.
- Slot reservation for specific roles. Any role can occupy any slot.
- Cross-machine slot coordination. Slots are per-Mill instance, not distributed.

## Priority

**P1** — quality of life. Model contention causes real failures at moderate fan-out, but single-agent delegations are unaffected. This becomes P0 when fan-out is the default workflow pattern.
