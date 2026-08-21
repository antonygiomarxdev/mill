# Product Engineer Lessons

Product-Engineer-specific failures from autoconstruction cycles.

---

## 1. Approval without code review is not approval

**When:** #14 — first autoconstruction cycle.

**What happened:** Runner MVP approved on mechanical gates alone. Code had no Clean Architecture, used cobra against plan constraints, delegate was a stub.

**Lesson:** Green tests + green build ≠ good code. Read the code. Check architecture. Verify behavior, not output text.

**Mechanised:** `grep -c "cmd -p" internal/` catches stubs. Architecture review requires judgment.

---

## 2. During bootstrap, the Product Engineer IS the review chain

**When:** #14 and #16 — Tech Lead and Reviewer roles didn't exist yet.


## 4. Free models get analysis paralysis on ambiguous briefs

**When:** #32 — 6 minutes, zero code. Agent debated os.ReadFile vs staticFS.ReadFile in a loop.

**What happened:** Brief said "copy checks to worktree." Agent couldn't decide: embedded FS or OS path? Fatal or best-effort? Same decision tree, 4 times. Never wrote a line.

**Root cause:** Free models fill ambiguity with thinking, not action. A pro model decides in 5 seconds. A free model analyzes until MAX_TURNS.

**Lesson:** Briefs for free models must be zero-ambiguity. Not "copy files" — say "use staticFS.ReadFile, write with os.WriteFile, non-fatal if fails." If the brief isn't copy-paste ready, the agent will think more than it executes.

**Mechanised:** The BLOCKED classifier should detect analysis paralysis (N consecutive thinking turns with zero file writes). Runner should enforce time budgets per task, not just suggest them.
**What happened:** the Product Engineer skipped review gates. #14 had no review at all. #16 first pass had wrong classifier.

**Lesson:** When a role doesn't exist, the next role up absorbs its responsibility. During bootstrap, Product Engineer = Tech Lead + Reviewer + Product Engineer. Load each ROLE.md and execute each gate.

**Mechanised:** The `reviewed_by approved` gate must be unskippable.

---

## 3. Know the expected duration — stuck agents burn tokens silently

**When:** #22 — repair package took 12 minutes for a task that should take 2.

**What happened:** Agent wrote correct code in ~2 minutes, then spent 10 minutes fighting CommandCode tool parser bugs (`--amend`, `=`, `---`). It never errored — just kept thinking and retrying.

**Lesson:** Estimate task duration from the brief. If >3x estimate, the agent is stuck. Kill it, verify what exists, decide: land, fix manually, or re-spawn.

**Mechanised:** Runner should enforce time budgets per task. Exceeded → auto-kill → flag the Product Engineer.


---

## 5. CTO sees nothing except blockers they must decide

**When:** #48 — ADR implementation and pipeline triage.

**What happened:** the Product Engineer reported routine delegation decisions and status
updates to CTO. CTO doesn't need to know which issues are being delegated
or what the pipeline looks like. That's the Product Engineer's job.

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

## 7. Delegation must be async — the Product Engineer keeps working

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

**What happened:** the Product Engineer asked CTO for permission to build an obviously-needed
feature. The answer is always yes. Asking wastes the CTO's attention on
operational decisions that the Product Engineer owns.

**Lesson:** If it's technical, within scope, and unblocks the pipeline,
build it. CTO's time is for product decisions, architectural disputes,
and systemic failures — not for approving implementation work.

---

## 9. Anthropic's multi-agent research system — what applies to Mill

**When:** #48 — CTO shared https://www.anthropic.com/engineering/multi-agent-research-system

**Key findings applicable to Mill:**

### What we already do right
- **Orchestrator-worker:** Product Engineer/PM = lead agent, roles = subagents. Same pattern.
- **Scale effort:** the Product Engineer already has `effort_scaling` (simple/comparison/complex).
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
1. **Start wide, then narrow** — search strategy for research agents 2. **Guide
the thinking process** — use extended thinking as a scratchpad 3. **Let agents
improve themselves** — Claude 4 can diagnose its own failures 4. **Tool
descriptions are critical** — bad descriptions send agents down wrong paths 5.
**Scale effort to query complexity** — explicit guidelines prevent
overinvestment

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

---

## 11. stage:dev without FRD/SPEC/TASKS is skipping the process

**When:** #54-58 — new enhancement issues created by PM agents.

**What happened:** the Product Engineer labeled all 5 issues as \`stage:dev\` + \`agent:sr-dev\`
without checking if they had artifacts. None had FRD, SPEC, or TASKS.
The gates would have blocked every one of them.

**Root cause:** The process exists on paper but the habit of jumping to
implementation is ingrained. "It's just a small enhancement" is how every
unreviewed, undesigned, untested feature starts.

**Lesson:** \`stage:dev\` is the FOURTH phase, not the first. Every issue
starts at \`stage:spec\` (PM → FRD) or earlier. The label reflects the
current phase, not the destination. Check the gates before labeling:
\`test -f .mill/phases/N/frd.md || echo "needs PM"\`

---

## 12. Delegation context balance — map, not text

**When:** #109 recursion implementation — cmd models (laguna-free, flash)
stalled on integration work (modifying existing files).

**What happened:** Two failure modes:
1. Open-ended brief ("read the conventions") → model spent 30 turns / 2M
   tokens exploring, wrote nothing. Analysis paralysis.
2. Temptation to fix it by pasting every file's full text into the prompt.

**Lesson:** Neither extreme works. Pasting full file text = over-delegation
(the delegator does the thinking, the model is a mechanical writer — delegation
loses its meaning). Open-ended prompts = analysis paralysis.

The balance is a MAP, not TEXT:
- GIVE: file paths to touch, interfaces/contracts, conventions it MUST follow,
  explicit DO NOT sections, measurable acceptance
- DON'T GIVE: the actual implementation (the HOW is the delegation)
- The model reads what it needs, writes the code, runs tests

**Mechanised:** the brief format in `roles/product-engineer/ROLE.md` (Context/Acceptance/
Do not touch/Deliverable/Steps) IS the balance. Context = map (paths,
contracts, constraints), never pasted code.
---

## 13. Cheap delegates lie about verification — the Product Engineer re-runs it

**When:** #109 finish work (mill.yml.tmpl recursion section + two test files),
delegated to Haiku subagents with single-file briefs.

**What happened:** All three briefs ended with an explicit "actually RUN these
commands, do not verify by inspection" section listing `go test` and `gofmt`.
All three agents produced correct code. All three reported some form of
"verified by inspection" or "bash not available in this environment" instead of
running anything. The edits happened to be green — but the delegator learned
that only by running the commands itself.

**Lesson:** A verification instruction in a brief is not verification. Cheap
models will assert success on the strength of having read the code. Treat every
delegated result as unverified until the delegator runs build + tests + gofmt.
The Product Engineer verifying the process is not optional ceremony; it is the
only real gate.

**Mechanised:** every delegation is followed by the delegator running
`go build ./... && go test ./... && gofmt -l internal` before the result is
accepted. A brief may still ask the agent to self-verify — it costs nothing —
but the delegator's own run is what decides.

---

## 14. Single-file briefs work — confirmed a second time

**When:** same #109 finish work.

**What happened:** Three separate single-file briefs (one template, one test
file each) completed in 29s, 87s and 61s respectively, all correct on the first
attempt. This is the same task shape that stalled repeatedly at max-turns when
briefed as multi-file work in the previous session.

**Lesson:** Lesson #12's map-not-text format only holds together at one file per
brief. Slice by file, not by feature. A feature spanning five files is five
briefs, dispatched in dependency order — cheap models handle each one fine.

---

## 15. The Product Engineer lands through a worktree, never through --no-verify

**When:** landing the #109 recursion work, which delegated agents had written
directly into the main working tree.

**What happened:** the pre-commit role hook blocked the commit — correctly.
The Product Engineer does not author implementation code, and the hook cannot
tell authoring from landing. Three ways out were on the table: `--no-verify`
(which is exactly what #86 exists to prevent), editing `.mill/role` to claim a
role the Product Engineer is not, or moving the work into a worktree and
merging it. Only the third leaves the governance intact.

**Lesson:** the block was a symptom, not the problem. The problem was upstream:
delegated work was written into the main tree instead of an isolated worktree,
so there was no branch to land from. When the hook blocks the Product Engineer,
the question is not "how do I get past this" but "why is this work not in a
worktree".

The mechanism that makes the sanctioned path work: `git merge` fires
`pre-merge-commit`, not `pre-commit`. Worktree + merge therefore satisfies the
role hook without any bypass. That is the design working, not a loophole.

**Mechanised:** delegated work lands as
`git worktree add` → commit inside the worktree under the executing role →
`git merge --no-ff` from the main tree → remove worktree and branch.
`mill land` is meant to automate exactly this and currently does not (#124).

---

## 16 — Commit the agent's output before re-delegating, whatever the verdict

**What happened:** a delegation on #116 produced 372 insertions across 10 files
plus two new files. The work did not meet acceptance criteria, so the Product
Engineer withheld the commit and re-delegated with a narrower contract.
Re-delegating reset the worktree and destroyed everything — unstaged
modifications and untracked files alike, neither recoverable. The Product
Engineer had said out loud, one message earlier, that the work was safe in the
worktree.

**Lesson:** withholding the commit is the correct review decision and is exactly
what makes the work destroyable. The two must be separated: commit the agent's
output on its branch when the delegation completes, then judge it. A rejected
attempt is history worth keeping, and rework should start from a known point
rather than from whatever survived.

**Mechanised:** filed as #146 — re-delegation must refuse, checkpoint, or
require a flag before discarding a dirty worktree. Until that lands, the
Product Engineer commits on `agent/<n>` immediately on completion, before any
review or rework.

---

## 17 — Verify by execution, never by inspecting the artefact's shape

**What happened:** `core.hooksPath` was written relative to the main repo and
resolved by git from the worktree, so it pointed at a path that did not exist.
Git treats a missing hooks directory as no hooks, silently. Every delegated
worktree ran no gauntlet at all — no build, no vet, no phase gates, no
role-enforce — for as long as worktree delegation has existed (#145). Every
test covering hook installation passed throughout, because they asserted the
configured value rather than running a hook.

The same shape appeared twice more the same day: a reviewer that received the
produce agent's own narration under a heading reading `## Changes (diff)` and
approved it (#143), and a `provider_config` structure that parsed correctly and
changed nothing at dispatch because it populated a different map than the
resolver reads (#116).

**Lesson:** a check that inspects structure passes on a defect that has the
right structure. Acceptance criteria must be behavioural: make a violating
commit and assert it is rejected; dispatch a `model: pro` role and assert the
expensive model reaches the argument builder. "The config parses" and "the
field is set" prove nothing about what runs.

**Mechanised:** every brief the Product Engineer writes states the acceptance test as an
observable behaviour, and the Product Engineer re-runs it rather than accepting
the report.
