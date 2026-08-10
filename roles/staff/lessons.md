# Staff Lessons

Staff-specific failures from autoconstruction cycles.

---

## 1. Approval without code review is not approval

**When:** #14 — first autoconstruction cycle.

**What happened:** Runner MVP approved on mechanical gates alone. Code had no Clean Architecture, used cobra against plan constraints, delegate was a stub.

**Lesson:** Green tests + green build ≠ good code. Read the code. Check architecture. Verify behavior, not output text.

**Mechanised:** `grep -c "cmd -p" internal/` catches stubs. Architecture review requires judgment.

---

## 2. During bootstrap, Staff IS the review chain

**When:** #14 and #16 — Tech Lead and Reviewer roles didn't exist yet.


## 4. Free models get analysis paralysis on ambiguous briefs

**When:** #32 — 6 minutes, zero code. Agent debated os.ReadFile vs staticFS.ReadFile in a loop.

**What happened:** Brief said "copy checks to worktree." Agent couldn't decide: embedded FS or OS path? Fatal or best-effort? Same decision tree, 4 times. Never wrote a line.

**Root cause:** Free models fill ambiguity with thinking, not action. A pro model decides in 5 seconds. A free model analyzes until MAX_TURNS.

**Lesson:** Briefs for free models must be zero-ambiguity. Not "copy files" — say "use staticFS.ReadFile, write with os.WriteFile, non-fatal if fails." If the brief isn't copy-paste ready, the agent will think more than it executes.

**Mechanised:** The BLOCKED classifier should detect analysis paralysis (N consecutive thinking turns with zero file writes). Runner should enforce time budgets per task, not just suggest them.
**What happened:** Staff skipped review gates. #14 had no review at all. #16 first pass had wrong classifier.

**Lesson:** When a role doesn't exist, the next role up absorbs its responsibility. During bootstrap, Staff = Tech Lead + Reviewer + Staff. Load each ROLE.md and execute each gate.

**Mechanised:** The `reviewed_by approved` gate must be unskippable.

---

## 3. Know the expected duration — stuck agents burn tokens silently

**When:** #22 — repair package took 12 minutes for a task that should take 2.

**What happened:** Agent wrote correct code in ~2 minutes, then spent 10 minutes fighting CommandCode tool parser bugs (`--amend`, `=`, `---`). It never errored — just kept thinking and retrying.

**Lesson:** Estimate task duration from the brief. If >3x estimate, the agent is stuck. Kill it, verify what exists, decide: land, fix manually, or re-spawn.

**Mechanised:** Runner should enforce time budgets per task. Exceeded → auto-kill → flag Staff.


---

## 5. CTO sees nothing except blockers they must decide

**When:** #48 — ADR implementation and pipeline triage.

**What happened:** Staff reported routine delegation decisions and status
updates to CTO. CTO doesn't need to know which issues are being delegated
or what the pipeline looks like. That's Staff's job.

**Lesson:** Only escalate when CTO decision is truly required (product/scope
decision, ≥2 subagents failed, research contradicts assumptions, dispute
between roles, systemic failure). Everything else: act, document, move on.

**Mechanised:** Escalation gates in `skills/mill.md`. GitHub issues are the
source of truth — keep labels, status, and comments updated so the board
reflects reality without verbal status reports.

---

## 6. Delegation is a handoff, not a fire-and-forget

**When:** #48 — defining the delegation workflow.

**What happened:** Process assumed agent spawn → agent finishes → done.
Real delegation has blockers, ambiguity, and handoffs between roles.

**Lesson:** Each role does its specific work. If something is unclear, the
agent raises its hand via a GitHub issue comment describing the blocker.
The delegator picks up the comment, resolves the ambiguity, and re-delegates
with amplified context. This cycle continues until the work is complete.
The issue is the handoff surface — not DMs, not terminal output.

**Mechanised:** Blocked workflow in `skills/mill.md` section "Blocked
workflow". Ledger records every block/resolve/re-spawn event.

---

## 7. Delegation must be async — Staff keeps working

**When:** #48 follow-up — CTO expects to continue working while agents run.

**What happened:** `mill delegate` was synchronous — it blocked until the
agent finished. The CTO couldn't spawn multiple agents and check results
later. The whole point of delegation is parallelism.

**Lesson:** Delegate async by default. `mill delegate <issue> --role X`
returns immediately with a task ID. The agent runs in background.
`mill status` shows progress. `mill delegate --wait` for sync when needed.

**Mechanised:** `runDelegate` in `internal/cli/delegate.go` — goroutine
for async path, `--wait` flag for sync.

---

## 8. Never ask "should I build this?" — just build it

**When:** #48 — implementing the task bridge for native delegation.

**What happened:** Staff asked CTO for permission to build an obviously-needed
feature. The answer is always yes. Asking wastes the CTO's attention on
operational decisions that Staff owns.

**Lesson:** If it's technical, within scope, and unblocks the pipeline,
build it. CTO's time is for product decisions, architectural disputes,
and systemic failures — not for approving implementation work.

---

## 9. Anthropic's multi-agent research system — what applies to Mill

**When:** #48 — CTO shared https://www.anthropic.com/engineering/multi-agent-research-system

**Key findings applicable to Mill:**

### What we already do right
- **Orchestrator-worker:** Staff/PM = lead agent, roles = subagents. Same pattern.
- **Scale effort:** Staff already has `effort_scaling` (simple/comparison/complex).
- **Brief format:** Detailed task descriptions prevent duplication. Our "Do not touch"
  and measurable acceptance criteria match their "clear task boundaries."
- **End-state evaluation:** Our criteria are countable, not process-based. Same approach.
- **Async delegation:** We already do async. Anthropic's system is still synchronous
  and they acknowledge it's a bottleneck.

### Gaps to address
- **No context window management.** Anthropic uses memory + compression for sessions
  exceeding 200K tokens. Mill agents can hit this with multi-role chains.
- **No artifact system.** Anthropic's subagents write to filesystem directly, bypass
  the coordinator for large outputs. Mill's worktrees partially solve this but the
  flow is still: agent → output → coordinator reads → next agent.
- **Token cost tracking.** Anthropic: 15× more tokens than chats. Multi-agent is
  expensive. Mill doesn't track or budget token spend.
- **Lead agent can't steer subagents mid-flight.** Neither can Anthropic's. Both
  systems are fire-and-forget per wave.

### Prompt engineering principles (from Anthropic)
1. **Start wide, then narrow** — search strategy for research agents
2. **Guide the thinking process** — use extended thinking as a scratchpad
3. **Let agents improve themselves** — Claude 4 can diagnose its own failures
4. **Tool descriptions are critical** — bad descriptions send agents down wrong paths
5. **Scale effort to query complexity** — explicit guidelines prevent overinvestment

---

## 10. Prompt instructions don't enforce process — failing scripts do

**When:** #41 — coverage target missed, no role raised their hand.

**What happened:** The skill said "you MUST spawn reviewer before continuing"
and "coverage must be ≥90%." The agents read it. Nobody enforced it.
Sr Devs wrote tests, coverage hit 85.5%, but nobody blocked progress.
The Reviewer was never spawned. The gate was skipped.

**Root cause:** Prompt instructions are suggestions. Agents will follow them
most of the time, but under pressure or ambiguity they optimize for
completion over compliance. "MUST" in a markdown file is not enforcement.

**Lesson:** Every gate must be a script that exits non-zero on failure.
`bash checks/gate-coverage` fails the build. The agent literally cannot
continue. `exit 1` is enforcement. Markdown is documentation.

**Mechanised:** `checks/gate-{frd,spec,tasks,coverage,review}` scripts.
Phase transitions in `skills/mill.md` reference them. Pre-push hook runs
them. The gate is the law — no exceptions, no bypasses.