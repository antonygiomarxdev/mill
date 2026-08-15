# Skill Authoring: What Makes a Skill Work

> Corpus study of 448 SKILL.md files installed on this machine, cross-referenced with public authoring guidance. Produced for Mill ADR 0006's premise that `using-mill.md` was written by inference, not from study of how good skills are built.

**Date:** 2026-08-15
**Corpus:** 448 unique SKILL.md files across `~/.claude/`, `~/.commandcode/`, `~/.agents/`, `~/.nvm/`
**Sources:** 18 independent authors (suppowers, mattpocock, Figma, Atlassian, Supabase, caveman, ponytail, Command Code, plugin-dev, and more)

---

## 1. Corpus Measurements

### 1.1 Length Distribution

| Metric | Lines |
|--------|-------|
| Min | 8 |
| Median | 103 |
| P75 | 170 |
| P90 | 432 |
| P95 | 518 |
| Max | 885 |
| Mean | 155 |

The corpus is heavily right-skewed. Most skills are short (mattpocock's 146 skills have a median of 76 lines). The long tail belongs to reference-heavy packs: Atlassian (median 576), Figma (median 215), and plugin-dev (885 lines for `command-development`).

Anthropic's guidance says keep SKILL.md under 500 lines. Only 6 of 448 files (1.3%) exceed that threshold. The corpus broadly obeys this limit.

### 1.2 Description Field

| Metric | Value |
|--------|-------|
| Files with description | 448/448 (100%) |
| Description length (median) | 223 chars |
| Description length (P90) | 548 chars |
| Description length (max) | 906 chars |
| Describes WHEN to use | 294 (66%) |
| Describes WHAT it does | 19 (4%) |
| Lists trigger phrases | 100 (22%) |
| Third person | 448/448 (100%) |

**Description style varies sharply by author:**

| Author | n | Avg length | "Use when" opener | Lists triggers |
|--------|---|-----------|-------------------|----------------|
| superpowers | 28 | 133 chars | 93% | 7% |
| mattpocock | 146 | 155 chars | 0% | 18% |
| commandcode | 31 | 153 chars | 26% | 23% |
| figma | 56 | 511 chars | 0% | 93% |
| supabase | 4 | 740 chars | 0% | 100% |
| atlassian | 6 | 494 chars | 0% | 83% |
| plugins (plugin-dev) | 29 | 390 chars | 0% | 79% |
| caveman | 11 | 370 chars | 0% | 100% |
| commandcode-bundled | 91 | 329 chars | 0% | 66% |

Two competing camps:
- **Superpowers** uses short "Use when..." descriptions (median 133 chars), no trigger lists, trusting the model to match on semantic meaning.
- **Figma/Supabase/Atlassian/plugin-dev** use long descriptions (400–900 chars) with explicit trigger phrases in quotes, betting that the model undertriggers without them.

The superpowers `writing-skills` codifies this as "Skill Discovery Optimization" (SDO): descriptions should contain ONLY triggering conditions, NEVER summarize the workflow, because agents will shortcut by reading the description instead of the body. This contradicts Anthropic's official guidance (Section 3.3), which says description should include "both what the Skill does and when to use it."

### 1.3 Heading Structure

| Heading | Occurrences | % of files |
|---------|------------|-----------|
| Overview | 54 | 12% |
| Process | 34 | 8% |
| Workflow | 32 | 7% |
| Quick Reference | 28 | 6% |
| Boundaries | 27 | 6% |
| References | 24 | 5% |
| Examples | 18 | 4% |
| Common Rationalizations | 18 | 4% |
| When to Use | 17 | 4% |
| Out of scope | 16 | 4% |
| Notes | 14 | 3% |
| Steps | 13 | 3% |
| Best Practices | 13 | 3% |

Most common first heading: "Process" (10 files), then the skill name itself (5 each for several mattpocock skills).

The heading analysis reveals no single canonical structure. However, the **pattern** that dominates the top-10 largest files is: title → Overview → Workflow/Process (numbered steps) → Common Patterns/Examples → Edge Cases/Troubleshooting → Quick Reference.

### 1.4 Structural Elements

| Element | Files | % |
|---------|-------|---|
| Code blocks | 266 | 59% |
| Tables | 149 | 33% |
| Examples | 272 | 61% |
| Prohibitions (do not/never/must not) | 390 | 87% |
| Checklists (- [ ]) | 126 | 28% |

Prohibition use is nearly universal. 87% of skills contain at least one "do not", "never", or "must not". This is higher than either Anthropic's guidance or the superpowers `writing-skills` would recommend — both argue for reasoned explanations over rigid bans.

### 1.5 Bundling

| Metric | Value |
|--------|-------|
| Skills with extra files in directory | 316 (71%) |
| Median extra files per bundle | 1 |
| Max extra files per bundle | 10 |

Bundling varies by author. mattpocock bundles 100% of skills (each has at least a README). Figma bundles 86%. Command Code bundles 87%. Ponytail and .openclaw bundle 0%.

### 1.6 Source Distribution

| Author | Skills | Median lines | Code blocks | Tables | Bundled |
|--------|--------|-------------|-------------|--------|---------|
| mattpocock | 146 | 76 | 35% | 3% | 100% |
| commandcode-bundled | 91 | 146 | 76% | 45% | 40% |
| figma | 56 | 215 | 71% | 71% | 86% |
| commandcode | 31 | 84 | 39% | 19% | 87% |
| plugins | 29 | 222 | 86% | 38% | 76% |
| superpowers | 28 | 172 | 86% | 75% | 57% |
| skills (external) | 12 | 66 | 50% | 42% | 8% |
| caveman | 11 | 83 | 73% | 45% | 27% |
| agents | 9 | 142 | 100% | 56% | 33% |
| ponytail | 8 | 52 | 25% | 25% | 0% |
| atlassian | 6 | 576 | 100% | 0% | 67% |
| supabase | 4 | 146 | 100% | 50% | 100% |

---

## 2. The Ten Most Substantial Skills: What They Do Differently

The top 10 by line count (deduplicating version copies):

| # | Lines | Name | Author | Notable feature |
|---|-------|------|--------|----------------|
| 1 | 885 | command-development | plugin-dev | Exhaustive frontmatter reference |
| 2 | 701 | triage-issue | Atlassian | Decision thresholds with percentages |
| 3 | 680 | writing-skills | superpowers | TDD methodology, rationalization tables |
| 4 | 638 | skill-development | plugin-dev | Progressive disclosure design principle |
| 5 | 584 | best-practices | gsd-pi | Before/after code comparisons throughout |
| 6 | 529 | figma-generate-design | Figma | Hard gates, parallel workflow pattern |
| 7 | 504 | subagent-driven-development | superpowers | Graphviz decision flowcharts |
| 8 | 486 | skill-creator | skill-creator | Template-based skill scaffolding |
| 9 | 478 | liquid-theme-a11y | liquid-skills | Accessibility audit checklist |
| 10 | 432 | figma-code-connect | Figma | MCP tool integration patterns |

### What these do that the median does not:

**Pattern 1: Decision flowcharts (superpowers, Figma)**
The top skills use Graphviz `dot` diagrams to encode decision trees. `subagent-driven-development` has a 7-node flowchart deciding whether to use it vs `executing-plans`. `figma-generate-design` uses "Skill Boundaries" to direct traffic to sibling skills. The median skill has no decision graph — it assumes the description suffices.

**Pattern 2: Rationalization/anti-pattern tables (superpowers, writing-skills)**
`writing-skills` includes a 9-row table mapping each excuse ("Too simple to test") to its rebuttal ("Simple code breaks. Test takes 30 seconds."). This is the most distinctive structural element in the corpus — only the superpowers pack does it, and it appears in their 5 largest skills. No other author uses it.

**Pattern 3: Concrete decision thresholds (Atlassian)**
`triage-issue` assigns confidence percentages to duplicate detection: ">90% = high confidence duplicate", "70–90% = likely", "40–70% = possibly related", "<40% = likely new." This quantitative precision is rare — only the Atlassian pack does it. It makes the skill actionable in a way that "check if it's similar" does not.

**Pattern 4: Hard gates and forbidden shortcuts (Figma)**
`figma-generate-design` uses explicit "Forbidden:" markers: "Forbidden: `search_design_system` for component keys until 2a-i is complete." This goes beyond ordinary prohibitions — it creates sequential preconditions that the agent must verify before proceeding. The median skill has no gating mechanism.

**Pattern 5: Glossary with precise definitions (mattpocock, codebase-design)**
`codebase-design` defines 8 terms (Module, Interface, Implementation, Depth, Seam, Adapter, Leverage, Locality) with explicit "Avoid:" alternatives for each. It forces consistent vocabulary: "Don't substitute 'component', 'service', 'API', or 'boundary.'" This is the strongest vocabulary enforcement in the corpus.

### Quotable passages:

1. **writing-skills** — on description design: *"Testing revealed that when a description summarizes the skill's workflow, an agent may follow the description instead of reading the full skill content."*

2. **writing-skills** — on form matching failure: *"The form that bulletproofs one failure type measurably backfires on another."*

3. **triage-issue** — on search strategy: *"Use key terms only: 'timeout login mobile' — NOT 'Users are getting a connection timeout error when...'"*

4. **figma-generate-design** — on parallel workflows: *"This combines the best of both: `generate_figma_design` gives pixel-perfect layout accuracy, while `use_figma` gives proper design system component instances."*

5. **codebase-design** — on vocabulary: *"Use these terms exactly — don't substitute 'component,' 'service,' 'API,' or 'boundary.' Consistent language is the whole point."*

---

## 3. Public Guidance

### 3.1 Anthropic's Official Best Practices

Source: https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices

Key points:

- **Description**: "should include both what the Skill does and when to use it." Third person. Max 1024 chars. Specific, includes key terms.
- **Conciseness**: "The context window is a public good." Default assumption: Claude is already very smart. Only add what it doesn't know.
- **Degrees of freedom**: Match specificity to fragility. Narrow bridge → low freedom. Open field → high freedom.
- **Progressive disclosure**: Metadata → SKILL.md → bundled resources. Keep SKILL.md under 500 lines. Split when approaching limit.
- **Evaluation-driven development**: "Create evaluations BEFORE writing extensive documentation." Build 3 scenarios. Establish baseline without skill. Write minimal instructions.
- **Iterative development**: Work with Claude A to create, test with Claude B (fresh instance).
- **Feedback loops**: Run validator → fix → repeat.
- **Anti-patterns**: Windows paths, too many options, time-sensitive info.
- **Checklist**: Description specific, under 500 lines, no time-sensitive info, consistent terminology, concrete examples, one-level-deep references, clear workflow steps.

### 3.2 Superpowers' Writing-Skills Guidance

Source: Installed at `~/.commandcode/skills/writing-skills/SKILL.md`

Key points (divergent from Anthropic):

- **Description = triggers only**: "NEVER summarize the skill's process or workflow." Tested empirically: a workflow-summarizing description caused agents to shortcut.
- **TDD for skills**: "Write test cases (pressure scenarios with subagents), watch them fail (baseline), write the skill, watch tests pass."
- **Match form to failure**: Prohibition-based bulletproofing backfires on shaping problems. Use recipes for output-shaping, prohibitions for discipline enforcement.
- **Token efficiency**: Frequently-loaded skills under 200 words. Move details to tool help.
- **Rationalization tables**: Explicitly catalog agent excuses and rebuttals.

### 3.3 Community Authoring Guide (lipex360x)

Source: https://gist.github.com/lipex360x/3a1a662525e88a3e856b7fda02ab8ce3

Key points:

- **"Pushy" descriptions**: "Claude tends to undertrigger — it won't use a skill unless the match is obvious." Include "even if they don't explicitly say X" pattern.
- **Reasoning over rigid rules**: "Explain the reasoning so the model understands why and can make better judgment calls in edge cases."
- **Craftsmanship repetition**: Repeat quality expectations at multiple points (from canvas-design analysis).

### 3.4 Where the Corpus Agrees with Guidance

| Guidance | Corpus agreement |
|----------|-----------------|
| Third person descriptions | 448/448 (100%) — universal agreement |
| SKILL.md under 500 lines | 442/448 (99%) — effectively universal |
| Progressive disclosure (bundled files) | 316/448 (71%) — majority use it |
| Concrete examples | 272/448 (61%) — majority include them |
| Code blocks in skill body | 266/448 (59%) — majority include them |
| Keep references one level deep | Hard to measure, but chaining is rare |

### 3.5 Where the Corpus Contradicts Guidance

| Issue | Anthropic says | Corpus does | Superpowers says |
|-------|---------------|-------------|-----------------|
| Description content | Include BOTH what + when | 66% describe when only, 4% describe what only | NEVER include what (SDO) |
| Description length | No guidance on length | Median 223 chars; Figma avg 511 chars | Short (133 chars median) |
| Trigger phrases in description | "include key terms" | 22% list explicit trigger phrases | 7% list triggers |
| Prohibitions | Not discussed | 87% use prohibitions | Prohibitions backfire on shaping problems |
| Testing | "Create evaluations" + "test with all models" | No evidence of evaluations in any skill file | Full TDD methodology with pressure scenarios |
| Degrees of freedom | Match specificity to fragility | Not explicitly addressed in any skill | Not discussed |

The sharpest contradiction: Anthropic says descriptions should say both "what" and "when"; superpowers says saying "what" causes agents to shortcut past the skill body. The Figma pack reconciles this by making "what" and "when" both very long (853 chars), which neither camp recommends. The Atlassian pack adds a separate `## Keywords` section (a third approach), though only 7/448 skills (2%) do this.

---

## 4. Critique of `using-mill.md`

### 4.1 What It Does Well

| Strength | Evidence |
|----------|----------|
| Within line budget | 290 lines (well under 500) |
| Tables for structured mappings | Role→stage table, model selection table, phase gate table |
| CLI command examples | Every step has runnable bash blocks |
| Practical lifecycle guidance | Worktree cleanup, worker-release, force-without-looking warning |
| Verification culture | "Recalculate every quantitative claim — do not trust the worker's numbers" |
| Measured honesty | Notes from practice: `--agent` must match, prohibitions underperform (#160) |

### 4.2 What It Is Missing

| Missing element | Why it matters | Precedent in corpus |
|----------------|----------------|---------------------|
| **Decision flowchart for role selection** | The role table maps stages to roles, but real dispatch decisions are more nuanced ("Is this a UX task or a UI task?"). A graphviz flowchart would reduce misrouting. | `subagent-driven-development` has a 7-node decision graph |
| **"Common Mistakes" or "Troubleshooting" section** | No catalog of what goes wrong in practice. The lifecycle warning about 13 worktrees and 63 MB is buried at the end. | `triage-issue` has a 4-section "Edge Cases & Troubleshooting" with concrete fixes |
| **Glossary** | FRD, ADR, spec, brief, stage — all used without definition. A new coordinator won't know the vocabulary. | `codebase-design` has an 8-term glossary with "Avoid:" alternatives |
| **Examples of complete dispatch cycles** | The skill describes the cycle abstractly but never shows a concrete example of "here's an issue, here's the brief, here's the dispatch, here's the verification." | `triage-issue` has 3 full end-to-end examples |
| **"When NOT to use Mill" section** | No boundary definition. When should the coordinator do the work itself instead of delegating? | `figma-generate-design` has "Skill Boundaries" directing to sibling skills |
| **Failure mode catalog** | "Never ignore a failure. Never re-dispatch without changing something" is good but abstract. Specific failure→fix mappings are missing. | `writing-skills` has rationalization tables mapping excuse→rebuttal |
| **Decision thresholds** | When is a result "good enough"? When does a free model failure warrant escalation to pro? No criteria. | `triage-issue` uses percentage confidence bands |

### 4.3 What It Should Drop

| Element | Problem |
|---------|---------|
| Empty code block at lines 175–176 | ` ```bash\n``` ` — a zero-content code block that looks like a formatting error |
| Duplicated topology and reporting rules | These appear identically in `COMMON.md` and `ROLE.md`. The skill should reference those files instead of duplicating 40+ lines |
| The parenthetical "(ADR 0005)" about `orca serve` | Dates the skill; Anthropic guidance says avoid time-sensitive info |

### 4.4 What It Should Add

| Addition | Rationale |
|----------|-----------|
| Glossary of 6 terms (FRD, ADR, spec, brief, stage, gate) | New coordinators need vocabulary; prevents miscommunication with workers |
| Decision flowchart for role selection | Reduces misrouting; the stage→role table is necessary but not sufficient |
| 1 end-to-end dispatch example | "Here's an issue → here's the brief → here's the dispatch → here's the result" in 20 lines |
| "Common Mistakes" section with 5 entries | e.g., "Dispatching next role before verifying current result" |
| "When to do it yourself" boundary | Some tasks are too small or too context-dependent for delegation |

### 4.5 Rewritten `description`

The current description is a single 609-character sentence listing trigger phrases. It works as a trigger list but reads as one long run-on, and it doesn't describe what the skill actually enables.

Proposed:

```yaml
description: >-
  Coordinate multi-role work through Mill's delegation framework.
  Use when dispatching a worker role, building a brief from a ROLE.md,
  answering a raised hand, verifying a worker's result against phase
  gates, deciding which role comes next, or when the user says
  "delegate this", "dispatch a worker", "who should do this", "hand
  this to the architect", or asks to build a feature, fix a bug, write
  a spec, or review work where the work should go to a role rather
  than be done in this session.
```

Changes:
- Opens with a one-line "what" (Anthropic guidance: include both what + when)
- Follows with trigger list (superpowers/Figma pattern)
- Removes "Triggers on" phrasing that is a label, not a trigger itself
- 370 characters — within the corpus median-to-P90 range

---

## 5. Does Quality Correlate with Popularity?

### 5.1 The Hypothesis

The CTO's premise: "popular skills are probably well made." Popularity here means installed-by-many-authors, not download counts (unavailable).

### 5.2 What the Data Shows

The largest packs by count (mattpocock: 146, commandcode-bundled: 91, figma: 56) are installed because their authors ship them as bundles, not because individual skills were selected by users. Installation is a packaging decision, not a quality signal.

However, within packs, some structural differences are visible:

| Author | Median lines | Code blocks | Tables | Rationalization tables | Decision graphs | Testing methodology |
|--------|-------------|-------------|--------|----------------------|-----------------|---------------------|
| mattpocock | 76 | 35% | 3% | 0 | 0 | 0 |
| superpowers | 172 | 86% | 75% | 5 skills | 8 skills | Full TDD |
| Figma | 215 | 71% | 71% | 0 | 0 | 0 |
| Atlassian | 576 | 100% | 0% | 0 | 0 | 0 |
| plugin-dev | 222 | 86% | 38% | 0 | 0 | 0 |

Superpowers is the only author that tests its skills with pressure scenarios, uses rationalization tables, and includes decision flowcharts. It is also the only pack where skills were clearly written as a deliberate methodology (TDD for documentation) rather than as reference material.

### 5.3 Assessment

**Quality is not correlated with popularity in this corpus.** Installation count reflects packaging decisions (which author ships a bundle), not per-skill quality evaluation. The mattpocock pack has 146 skills, most under 80 lines with minimal structure — these are short, functional, and widely installed, but they are not the best-designed skills in the corpus by any structural measure.

The superpowers pack is the most deliberately engineered (28 skills), with the richest structural patterns (flowcharts, rationalization tables, TDD methodology), the most opinionated description strategy (short, trigger-only), and the only explicit testing methodology. It is not the largest pack.

**What would settle it:** An A/B test where the same task is dispatched using (a) a mattpocock-style short skill, (b) a Figma-style long-description skill, (c) a superpowers-style TDD-tested skill, measuring task completion rate and compliance. No such test exists in the corpus or in published literature.

**What exists instead:** The superpowers `writing-skills` cites internal "wording tests" showing that prohibition-based guidance can backfire vs. recipe-based guidance (5+ reps per variant, manually verified). This is the closest thing to empirical evidence in the corpus, but it measures one design choice, not overall quality.

### 5.4 Caveats

- The corpus represents what is installed on one machine, not what is most popular globally.
- "Quality" here is measured structurally (presence of examples, decision graphs, testing), not by outcome (task success rate).
- Some very short skills (mattpocock's `research` at 13 lines, `implement` at 16 lines) may be perfectly effective for their scope — brevity is not a defect if the task is narrow.

---

## Sources

| Source | URL |
|--------|-----|
| Anthropic: Skill authoring best practices | https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices |
| Superpowers writing-skills (installed) | `~/.commandcode/skills/writing-skills/SKILL.md` |
| Superpowers anthropic-best-practices (installed) | `~/.commandcode/skills/writing-skills/anthropic-best-practices.md` |
| Community authoring guide (lipex360x) | https://gist.github.com/lipex360x/3a1a662525e88a3e856b7fda02ab8ce3 |
| Anthropic skills overview | https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview |
| Anthropic skills quickstart | https://platform.claude.com/docs/en/agents-and-tools/agent-skills/quickstart |
| Plugin-dev skill-development | `~/.claude/plugins/marketplaces/claude-plugins-official/plugins/plugin-dev/skills/skill-development/SKILL.md` |
| Figma generate-design | `~/.claude/plugins/cache/claude-plugins-official/figma/2.2.95/skills/figma-generate-design/SKILL.md` |
| Atlassian triage-issue | `~/.claude/plugins/cache/claude-plugins-official/atlassian/94a30436435f/skills/triage-issue/SKILL.md` |
| mattpocock codebase-design | `~/.claude/plugins/cache/claude-plugins-official/mattpocock-skills/1.2.2/skills/engineering/codebase-design/SKILL.md` |
| using-mill.md (subject of critique) | `.mill/skills/using-mill.md` |
