# Harness Engineering and Evals: What the Field Knows, and What It Means for Mill

> Research brief — Architect role, 2026-08-15.
> Purpose: check Mill's two foundational decisions (ADR 0005, ADR 0006) against what the field already knows about agent harnesses, evals, and measuring agent work.

---

## 1. Harness Engineering as a Discipline

### What is a harness?

The field has converged on a definition: **Agent = Model + Harness**. The harness is everything that enables a model to act as an agent — system prompts, tool surfaces, rollout protocols, context management, memory, guardrails, verifiers, observability, and sub-agent topology.

Source: Lee (2026) defines the harness as the orchestration layer between model and environment, decomposing it into nine components: system prompt/persona, tool surface, rollout protocol, context manager, memory, sub-agent topology, guardrails and gates, verifiers and judges, and observability.
— https://leehanchung.github.io/blogs/2026/05/08/hidden-technical-debt-agent-harness/

Anthropic's evals post draws the same boundary: "An agent harness (or scaffold) is the system that enables a model to act as an agent: it processes inputs, orchestrates tool calls, and returns results. When we evaluate 'an agent,' we're evaluating the harness *and* the model working together."
— https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents

### Inner vs. outer harness

Böckeler (2026) draws a clean distinction with two concentric circles:
- **Inner harness** — what the model's builder ships (Claude Agent SDK, Codex app server, system prompt, built-in tools).
- **Outer harness** — what the user assembles on top (AGENTS.md, MCP servers, custom skills, project-specific review agents, linters, structural tests).

Both are harness. They evolve on different clocks and accumulate different debt.
— https://martinfowler.com/articles/harness-engineering.html

### Where does the field put the boundary between policy and substrate?

The field uses three nested surfaces with different iteration cadences:

| Surface | Cost to change | Cadence | Owner |
|---|---|---|---|
| Skills / prompts | Cheap — text edits | Hourly to daily | Product builders |
| Harness | Medium — code, ships with binary | Daily to weekly | Research engineers |
| Model | Expensive — post-training compute | Quarterly | Lab |

Source: Lee (2026), "Three Optimization Surfaces."
— https://leehanchung.github.io/blogs/2026/05/08/hidden-technical-debt-agent-harness/

The opinionated direction from multiple sources is **"thin harness, fat skills"**: keep the harness thin with a small set of primitives that mirror what the lab post-trained against; put domain expertise into skills where iteration is fast, the artifact is human-readable, and the cost of being wrong is a text edit rather than a release.
— https://github.com/garrytan/gbrain/blob/master/docs/ethos/THIN_HARNESS_FAT_SKILLS.md

### Does ADR 0005's split match the field?

**Mostly yes, with a caveat.** ADR 0005 splits Mill into: Mill owns policy (roles, phase sequence, gates, brief construction, model tier selection); Orca owns substrate (spawning, supervision, worktrees, message bus, parallelism). This maps cleanly onto the "skills/policy layer" vs. "harness/substrate layer" distinction the field uses.

However, the field draws the boundary *inside* the harness more finely than Mill does. Böckeler distinguishes computational controls (deterministic: linters, type checkers, tests) from inferential controls (semantic: LLM-as-judge, AI review). Mill's phase gates are bash scripts (computational), but the review step is an LLM call (inferential) with no calibration or grading rubric. The field treats these as requiring different engineering discipline.
— https://martinfowler.com/articles/harness-engineering.html

**Where ADR 0005 is unusual:** Mill delegates *all* substrate to an external tool (Orca). Most production systems keep the harness in-process or in-repo. The field's examples — Anthropic's long-running agent harness, OpenAI's Codex harness, Cognition's Devin — all own their harness code. Mill is closer to the "thin harness" extreme than any primary source describes in production. This is a defensible position given the "thin harness, fat skills" argument, but it means Mill's reliability depends entirely on Orca's reliability, and Mill has no fallback if Orca changes its CLI surface.

---

## 2. Hidden Technical Debt

### What does the harness-debt argument identify?

Lee (2026) argues that agent harnesses are **temporary artifacts** whose structure compensates for current model limitations. As models improve, harness structure becomes overhead. The specific debts he names:

1. **Tool wrappers** that will dissolve when models can read raw API specs (already happening).
2. **Planner-executor scaffolds** that dissolve when models interleave planning and action natively (already happening with agentic-thinking models).
3. **Memory abstractions** (vector stores, summarization passes) dissolving in favor of plain text files plus git history.
4. **Multi-agent topologies** dissolving as context windows grow and tool use improves.
5. **No-code workflow builders** dissolving because a single long-horizon agent does what canvas tools assembled from dozens of nodes.
6. **Training/production harness mismatch** — the production harness should not be a deployed training harness, and vice versa.

Source: "The harness is the structure we need for the level of model capability we have today. A well-engineered 2026 harness is a 2026 artifact."
— https://leehanchung.github.io/blogs/2026/05/08/hidden-technical-debt-agent-harness/

### Which debts has Mill shed, carried, or moved to Orca?

| Debt category | Mill's position | Assessment |
|---|---|---|
| **Process spawning / worktree isolation** | Shed — moved to Orca (ADR 0005) | Correct. This is substrate plumbing, not policy. |
| **Supervision / dead-worker detection** | Shed — Orca's `worker-abandon` fences uncertain workers (#157) | Correct. Field consensus: supervision is harness infrastructure. |
| **Message bus / raise-a-hand cycle** | Shed — Orca's `send`/`ask`/`check`/`reply` | Correct. |
| **Planner-executor scaffold** | **Carried.** Mill's coordinator IS a planner-executor: it plans the sequence (FRD → spec → tasks → implementation → review), dispatches workers, and synthesizes results. | Debt risk. The field says this pattern is dissolving into the model. However, Mill's version is organizational decomposition (PM, Architect, Tech Lead), not step-by-step tool planning, so it may survive longer. |
| **Multi-agent topology** | **Carried.** Mill is explicitly a multi-agent system with 8+ worker roles. | Debt risk. Cognition's Walden Yan argues: "Running multiple agents in collaboration only results in fragile systems. The decision-making ends up being too dispersed and context isn't able to be shared thoroughly enough between the agents." Mill mitigates this with the star topology (coordinator as single hub), which matches Anthropic's approach, but the cost of context fragmentation across workers remains. — https://cognition.ai/blog/dont-build-multi-agents |
| **Tool wrappers** | Mostly shed. Mill uses `orca orchestration` CLI directly. | Correct, though ADR 0005 notes coupling risk to Orca's CLI surface. |
| **Memory abstraction** | Shed in principle. Progress files and git log are the memory (ADR 0006: "Plain text in the working directory is something the model already knows how to read"). | Correct. Matches Anthropic's long-running agent pattern: `claude-progress.txt` + `feature_list.json` + `git log`. — https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents |
| **Evaluation harness** | **Missing entirely.** Mill has no eval of any kind. | Critical gap. See Section 3. |
| **Observability** | Partially present. FINDINGS-2026-08 shows `/proc` inspection and ledger timestamps were needed to discover defects. | Needs work. The field treats observability as a core harness component, not an afterthought. |

### OpenAI's harness debt: the "garbage collection" problem

OpenAI's Codex team found that fully agent-generated codebases accumulate drift — agents replicate patterns that already exist, including suboptimal ones. They initially spent every Friday (20% of the week) cleaning up "AI slop," then encoded "golden principles" into the repository and built recurring cleanup tasks.

Mill has the same problem but no cleanup mechanism. The roles directory accumulates lessons files that nothing reads (FINDINGS #137). There is no recurring scan for stale or contradictory policy.
— https://openai.com/index/harness-engineering/

---

## 3. Evals

### How are agent systems evaluated?

Anthropic's evals post (2026-01) defines the structure: a **task** (input + success criteria), a **trial** (one attempt), a **grader** (logic that scores), a **transcript** (complete record), and an **outcome** (final state). Three grader types:

1. **Code-based** — deterministic, fast, cheap: string matching, binary tests (fail-to-pass), static analysis, outcome verification.
2. **Model-based** — flexible, scalable, non-deterministic: rubric scoring, natural language assertions, pairwise comparison.
3. **Human** — gold standard but expensive: SME review, spot-check sampling, A/B testing.

Source: "When we evaluate 'an agent,' we're evaluating the harness *and* the model working together."
— https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents

For coding agents specifically, the field relies on **deterministic graders** as the primary signal: does the code compile, do tests pass, does it break existing tests? SWE-bench Verified follows this approach — a solution passes only if it fixes failing tests without breaking existing ones.
— https://openai.com/es-419/index/introducing-swe-bench-verified/

### Non-determinism matters

Agent behavior varies between runs. Two metrics capture this:
- **pass@k**: likelihood of at least one success in k attempts (measures capability).
- **pass^k**: probability that ALL k trials succeed (measures reliability).

For Mill delegations, pass^k is more relevant than pass@k: a delegation that succeeds once but fails three times is unreliable, and the coordinator cannot afford to re-run every delegation multiple times.
— https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents

### What would an eval for Mill measure?

Mill currently verifies work by a human reading a diff. Based on the field's framework, here is what could be measured, ordered by cost:

**Tier 1 — Computational graders (cheap, deterministic, automatable):**
- Does the produced artifact (spec, code, ADR) pass the phase gates?
- Does the code compile and pass existing tests? (fail-to-pass, pass-to-pass)
- Does `role-enforce` correctly block disallowed file types?
- Are artifacts present in the expected locations after delegation?
- Does the cost model actually dispatch different models to different tiers?

**Tier 2 — Structural graders (cheap, partially deterministic):**
- Is the artifact structurally complete? (ADR has all four sections; spec has boundaries defined; tasks are granular enough)
- Token count / cost per delegation
- Time to completion per role

**Tier 3 — Inferential graders (expensive, non-deterministic):**
- LLM-as-judge reviewing spec quality against a rubric
- LLM-as-judge comparing reviewer output against the actual diff (catching #143/#158)
- Pairwise comparison of delegation quality across model tiers

**Tier 4 — Human graders (most expensive):**
- SME review of architectural decisions
- Spot-check sampling of delegation quality

**Cost estimate:** Tier 1 can be implemented as bash scripts in the existing gates directory — near zero marginal cost. Tier 2 requires modest instrumentation. Tier 3 requires an LLM judge and calibration against human samples — medium cost. Tier 4 is ongoing human time.

The field's recommendation is to start with 20-50 tasks drawn from real failures. Mill's FINDINGS document already identifies 15+ real failures. Converting these into eval tasks would be the fastest path to a non-zero eval.

Source: "20-50 simple tasks drawn from real failures is a great start... Evals get harder to build the longer you wait."
— https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents

---

## 4. SWE-bench Verified

### What its methodology corrects

SWE-bench Verified is a 500-sample subset of the original SWE-bench test set, filtered by human annotators to remove tasks that are:

1. **Underspecified** — the problem description doesn't convey what the solution should look like (38.3% of original samples flagged).
2. **Unfairly tested** — the FAIL_TO_PASS tests reject valid solutions, often by requiring exact match on strings that only appear in the PR discussion, not the issue description (61.1% flagged).
3. **Environment-broken** — the dev environment setup causes test failures regardless of solution quality.

In total, 68.3% of original SWE-bench samples were filtered out. GPT-4o's score doubled from 16% to 33.2% on the verified set — not because it got smarter, but because the benchmark stopped penalizing it for things that weren't its fault.

Source: OpenAI (2024), "Introducing SWE-bench Verified."
— https://openai.com/es-419/index/introducing-swe-bench-verified/

The key insight: **performance increased *within* difficulty categories**, not just overall. This means the filtering removed genuinely impossible tasks across all difficulty levels, not just easy ones.

### What its methodology corrections say about measuring agent work honestly

Three principles from SWE-bench Verified that apply to judging a Mill delegation:

1. **Verify that the evaluation criteria are fair.** SWE-bench Verified's core contribution was annotating whether tests were too specific or unrelated to the problem. Mill's phase gates check structure (file exists, coverage threshold) but not whether the criteria themselves are fair. FINDINGS #151 shows `gate-coverage` sampled one arbitrary package — the gate was unfair in exactly the way SWE-bench corrected for.

2. **Separate "the task was impossible" from "the agent failed."** When a delegation fails, was it because the brief was underspecified, or because the agent was incapable? Mill has no mechanism to distinguish these. The field's answer: annotate your tasks, and filter out the ones that are impossible before blaming the agent.

3. **Use multiple annotators and take the worst label.** SWE-bench Verified used three annotators per sample and took the most severe flag. Mill's review is a single LLM call against an empty diff (#143) — one judge, no calibration, no redundancy.

### Additional insight: scaffold performance varies wildly

SWE-bench Verified showed that GPT-4's performance on SWE-bench Lite varies from 2.7% to 28.3% depending on the scaffold used. This means the harness matters as much as the model. Mill's claim that "quality comes from the review step, not from the writer" (PRODUCT.md) is consistent with this finding — but only if the review step is actually using a capable model, which FINDINGS #116 shows it was not.

---

## 5. Where Mill Is Wrong or Unusual

### 5.1 Mill has no eval at all — this contradicts the entire field

Every primary source treats evals as foundational. Anthropic: "Teams without evals get bogged down in reactive loops." OpenAI: "We needed to understand what changes when a software engineering team's primary job is no longer to write code." Böckeler: computational and inferential sensors are the harness.

Mill's verification is a human reading a diff. This is the pre-eval state the field describes as the breaking point: "The breaking point often comes when users report the agent feels worse after changes, and the team is 'flying blind' with no way to verify except to guess and check."
— https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents

**Verdict: Mill needs an eval layer. This is not optional.**

### 5.2 The multi-agent topology carries real risk

Cognition's Walden Yan argues against multi-agent architectures on the grounds that (1) sub-agents don't share context well, (2) actions carry implicit decisions that conflict when agents work in parallel, and (3) "running multiple agents in collaboration only results in fragile systems."
— https://cognition.ai/blog/dont-build-multi-agents

Mill's star topology partially mitigates this — the coordinator shares context to each worker, and workers don't communicate directly. But Mill's workers DO make conflicting implicit decisions: the Architect chooses a pattern, the Tech Lead decomposes it, and the Sr Dev implements it — each in a separate context window with no shared trace. Yan's Principle 2 applies: "Actions carry implicit decisions, and conflicting decisions carry bad results."

Anthropic's long-running agent harness uses a **single coding agent** across multiple context windows, not multiple specialized agents. They explicitly note: "It's still unclear whether a single, general-purpose coding agent performs best across contexts, or if better performance can be achieved through a multi-agent architecture."
— https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents

**Verdict: Mill's multi-agent architecture is a bet against the field's current direction. It may pay off for organizational decomposition (PM, Architect, Tech Lead require genuinely different expertise), but the context fragmentation cost is real and unmeasured.**

### 5.3 The cost model has never run — and the field says the harness IS the cost model

FINDINGS #116 shows `model: pro` / `model: free` in role frontmatter affects nothing. Every role dispatched on the cheapest model. Lee (2026) notes: "First-party harnesses outperform third-party ones on the same model" because "the harness is part of the contract the model was trained under." Empirical work (arxiv 2603.08640v1) showed measurable benchmark gaps between first-party and third-party harnesses.

Mill is a third-party harness that cannot control which model it dispatches. This means Mill is running every model out-of-distribution relative to its training harness, AND cannot implement the cost differentiation that is supposed to be its core economic premise.
— https://leehanchung.github.io/blogs/2026/05/08/hidden-technical-debt-agent-harness/

**Verdict: The cost model is not a nice-to-have; it is the economic foundation. If Mill cannot dispatch different model tiers, its economic argument collapses. ADR 0005 acknowledges this ("the one substrate capability Orca does not provide") but treats it as a future problem. The field says it is a present problem.**

### 5.4 "Policy stays in Markdown" is unusual but defensible

ADR 0006 retires the Go binary because everything Mill keeps is Markdown or bash. The field's "thin harness, fat skills" direction supports this: skills (text) iterate hourly; harnesses (code) iterate daily; models iterate quarterly.

However, OpenAI's Codex team found that documentation alone doesn't maintain coherence: "By enforcing invariants, not micromanaging implementations, we let agents ship fast without undermining the foundation." They enforce with custom linters and structural tests — computational, not prose.
— https://openai.com/index/harness-engineering/

Mill's `role-enforce` and phase gates ARE computational (bash), which matches this. But the rest of the policy — the phase sequence, brief construction, who delegates to whom — lives in prose that an agent must read and follow. FINDINGS #153 showed this failed: roles were never told to hand off because the prose rule had no mechanism.

**Verdict: The Markdown-first approach is directionally correct per "thin harness, fat skills," but Mill needs to mechanize the rules that matter (the sequence, the dispatch) rather than relying on agents to follow prose. This is the lesson of FINDINGS #153, and the field confirms it.**

### 5.5 ADR 0006's claim that "a binary was not enforcing it either" is a rationalization

ADR 0006 argues: "Today's evidence cuts both ways: the roles were never told to hand off (#153), so the binary was not enforcing it either." This is true but misleading. The binary *could have been fixed* to enforce the sequence; a skill *cannot enforce* it — it can only instruct. The field's distinction between computational (enforced) and inferential (suggested) controls is relevant here. Mill moved enforcement from computational (code that could dispatch) to inferential (prose that an agent might follow).

This is not necessarily wrong — the field agrees models are getting better at following instructions — but it is a real loss of enforcement capability that ADR 0006 understates.

---

## What Mill Should Change

Based on the evidence above, ordered by urgency:

### 1. Build a minimal eval suite (highest priority)

**Evidence:** Every source treats evals as foundational. Anthropic: "Evals get harder to build the longer you wait." Mill has none.

**Action:** Convert the 15+ failures in FINDINGS-2026-08 into eval tasks. Start with Tier 1 computational graders (bash): does the artifact exist, does it compile, does it pass gates, does the cost model dispatch the right model? This is near-zero cost and directly tests the defects that already occurred.

**Source:** https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents (Sections: "Collect tasks for the initial eval dataset", "Step 1: Start with what you already test manually")

### 2. Mechanize the dispatch sequence

**Evidence:** FINDINGS #153 (roles never hand off), confirmed by the field's distinction between computational and inferential controls. Böckeler: "Computational controls are cheap, fast, and deterministic. Inferential controls are expensive and non-deterministic."

**Action:** Either (a) add a computational check that verifies the coordinator actually dispatched the expected next role, or (b) encode the sequence in the coordinator's prompt as a structured checklist (like Anthropic's `feature_list.json`), not as prose in COMMON.md.

**Source:** https://martinfowler.com/articles/harness-engineering.html (Computational vs Inferential section)
**Source:** https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents (Feature list section)

### 3. Fix the cost model or abandon the economic claim

**Evidence:** FINDINGS #116, Lee (2026) on harness-model coupling. The field says model tier selection is part of the harness contract.

**Action:** Either implement model tier selection through Command Code's configuration (as ADR 0005 suggests), or explicitly state in PRODUCT.md that the cost model is not yet operational and the economic premise is aspirational.

**Source:** https://leehanchung.github.io/blogs/2026/05/08/hidden-technical-debt-agent-harness/ (First-party vs third-party harness section)

### 4. Add observability beyond log-reading

**Evidence:** FINDINGS documents that every major defect was discovered through `/proc`, ledger timestamps, or manual inspection. OpenAI's team made their app "legible" to agents via observability stacks (LogQL, PromQL, traces). Anthropic's team reads transcripts regularly as a core practice.

**Action:** At minimum, log which model was actually dispatched per role, which phase each worker is in, and what the reviewer's verdict rests on (not just the verdict).

**Source:** https://openai.com/index/harness-engineering/ ("Increasing application legibility" section)
**Source:** https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents (Step 6: "Check the transcripts")

### 5. Evaluate whether the multi-agent topology is paying for itself

**Evidence:** Cognition's argument against multi-agents; Anthropic's uncertainty about whether specialized agents outperform a single general-purpose agent. Mill's context fragmentation across workers is unmeasured.

**Action:** Run an A/B comparison: give the same task to (a) the full Mill delegation chain and (b) a single agent with the complete brief. Measure quality, cost, and time. If the single agent matches or exceeds the chain, the multi-agent overhead is not paying for itself.

**Source:** https://cognition.ai/blog/dont-build-multi-agents
**Source:** https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents ("Future work" section)

### 6. Clean up stale policy regularly

**Evidence:** OpenAI's "garbage collection" pattern — recurring scans for stale documentation, with agents opening fix-up PRs. Mill's lessons files are written and never read (#137).

**Action:** Add a recurring task (weekly or per-delegation) that scans `.mill/` for stale lessons, contradictory rules, and untested gates. This is the policy equivalent of OpenAI's "doc-gardening agent."

**Source:** https://openai.com/index/harness-engineering/ ("Entropy and garbage collection" section)

---

## Sources

### Primary sources (supplied by CTO)
1. https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents — Anthropic (2026-01-09). Eval structure, grader types, non-determinism metrics, roadmap from zero evals.
2. https://leehanchung.github.io/blogs/2026/05/08/hidden-technical-debt-agent-harness/ — Lee, H. (2026-05-08). Harness decomposition, training vs production asymmetry, thin harness/fat skills, bitter lesson.
3. https://walkinglabs.github.io/learn-harness-engineering/es/ — Walking Labs (2026). Harness engineering curriculum: environment, state, verification, control systems.
4. https://openai.com/es-419/index/introducing-swe-bench-verified/ — OpenAI (2024-08-13). SWE-bench Verified methodology, human annotation, benchmark fairness.

### Additional primary sources
5. https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents — Anthropic (2025-11-26). Two-agent harness pattern (initializer + coding agent), progress files, feature lists, incremental progress.
6. https://cognition.ai/blog/dont-build-multi-agents — Yan, W. / Cognition (2025-06-12). Context engineering principles, argument against multi-agent architectures, share context principle.
7. https://openai.com/index/harness-engineering/ — Lopopolo, R. / OpenAI (2026-02-11). Zero-manually-written-code experiment, agent legibility, garbage collection, progressive disclosure.
8. https://martinfowler.com/articles/harness-engineering.html — Böckeler, B. (2026-04-02). Inner/outer harness, computational vs inferential controls, feedforward/feedback, harness templates.
9. https://www.anthropic.com/engineering/building-effective-agents — Anthropic (2024-12-19). Agentic system patterns (workflows vs agents), augmented LLM, orchestrator-workers.
10. https://arxiv.org/abs/2603.08640v1 — Post-training harness mismatch paper, cited by Lee (2026). Measurable benchmark gap between first-party and third-party harnesses.
11. https://github.com/garrytan/gbrain/blob/master/docs/ethos/THIN_HARNESS_FAT_SKILLS.md — Garry Tan / gbrain. "Thin harness, fat skills" ethos.

### Mill's own evidence
12. docs/FINDINGS-2026-08.md — Two days of measurement: 15+ defects, economics never ran, verification by shape not behavior.
13. docs/adr/0005-orca-as-execution-substrate.md — Decision to use Orca as substrate.
14. docs/adr/0006-mill-is-a-skill-not-a-binary.md — Decision to retire Go CLI, Mill becomes skill + policy directory.
15. docs/PRODUCT.md — Product definition: org chart that executes, economics, learning from failure.
