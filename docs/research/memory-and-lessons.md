# How Should Mill Remember What It Learns?

> Research — Architect role, 2026-08-15.
> Issue: #137 — lessons are written to a file nothing reads. The product
> definition demands learning arrive in a role's context when that role next
> executes (docs/PRODUCT.md, "Learning from failure").
> Purpose: decide how Mill stores, curates, and delivers lessons — or whether
> the current lessons mechanism should be deleted.

---

## 1. The problem, measured

Mill writes lessons to `.mill/roles/<role>/lessons.md` for three roles (staff,
tech-lead, reviewer). **No code reads them and no brief includes them.** A grep
of `checks/` — the only mechanisms that run during a commit — finds zero
references to `lessons` or `memory`. COMMON.md describes lessons.md as
"reference material, not required reading" — prose with no mechanism, exactly
the failure shape FINDINGS-2026-08 names: "A rule written in Markdown with no
mechanism behind it."

Seventeen lessons sit in `staff/lessons.md`. Not one reached a worker.

This is not a storage problem. Every tool examined stores lessons in a plain
text file in the working tree. The failure is that nothing reads it at the
moment of work — and an unread file that looks like learning is the failure
this project keeps finding (PRODUCT.md: "Writing them to a file nothing reads
is journaling, and is worse than nothing, because it looks like the problem
was handled").

---

## 2. What the memory-file pattern actually is

Six mechanisms were examined. All are plain text in a known location. They
differ on **who writes, when it is read, and what happens when it grows.**

### 2.1 Command Code — `AGENTS.md` memory (project)

Command Code carries memory into **every turn** as part of the system prompt,
not the conversation. It is re-read every request, survives compaction (it is
never summarized away), and costs tokens on every turn — `/context` shows the
exact cost. Three tiers (user, project, subdirectory) are assembled in order,
each headed by its source path so the model knows which file a rule came from.
A memory file that grows into a wiki is a real, recurring cost; the guidance
is to keep it to standing rules and pull long-form material in with `@path`
imports so it is only present when relevant.

Sources: https://commandcode.ai/docs/memory, local
`command-code-knowledge/reference/memory.md`.

### 2.2 Command Code — `Taste` (learned, separate mechanism)

Taste is deliberately distinct from written memory: "learned style, as opposed
to written rules." It is learned automatically from every accept, reject, and
edit; stored in `.commandcode/taste/` (project) or `~/.commandcode/taste/`
(global) with one markdown file per category, each entry carrying a
**confidence score**. This is the one examined mechanism that is not curated
by a human — it is a continuous learning loop with no curation step, and it is
the only one that tracks confidence.

Sources: https://commandcode.ai/docs/taste, local
`command-code-knowledge/reference/taste.md`.

### 2.3 Claude Code — per-project `MEMORY.md` (curated, on-demand)

Claude Code keeps a per-project memory directory
(`~/.claude/projects/<project>/memory/`) containing a small `MEMORY.md` index
plus one detail file per entry. On this machine the real entries are hand-kept,
each with a `type: feedback` origin and a `How to apply` section; the index is
3–5 lines of pointers with one-line "why it matters" summaries. Nothing appends
automatically. Curation is the human reading the index, and the index is kept
short because it is the read surface. This is a **curated index + detail-file**
pattern, read on demand, not injected into every prompt.

Local evidence: `~/.claude/projects/-home-ksante-dev-rumai-labs-rumai/memory/`,
`~/.claude/projects/-home-ksante-dev-ATOM/memory/`.

### 2.4 Anthropic — `claude-progress.txt` + feature list (long-running agent harness)

Anthropic's long-running agent harness solves the "each new session begins with
no memory" problem with two plain-text files. An **initializer agent** writes a
`claude-progress.txt` log and a `feature_list.json` of end-to-end features all
marked "failing"; every subsequent **coding agent** reads the progress file and
git log at session start, works one feature, and ends the session by committing
and appending a progress update. This is the cleanest example of a memory file
that is read at the moment of work: the read is a **mandatory startup step**
(pwd, read progress file, read feature list, read git log), and the write is a
**mandatory shutdown step** (commit + progress update). The file is a bridge
between context windows, and it is deliberately kept as a short log of what was
done, not a wiki of everything known.

Source: https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents

### 2.5 OpenAI Codex — `AGENTS.md` as table of contents + structured `docs/`

OpenAI's harness-engineering writeup is the most directly on-point source. They
**tried the "one big AGENTS.md" approach and it failed in four measured ways**:
context is scarce and a giant file crowds out the task; too much guidance
becomes non-guidance; it rots instantly into a graveyard of stale rules; and a
single blob is hard to verify mechanically. Their fix: a short AGENTS.md
(~100 lines) injected into context as **the table of contents**, with the real
knowledge living in a structured `docs/` directory treated as the system of
record. Plans, quality grades, and technical-debt tracking are all versioned
artifacts co-located in the repo. They enforce freshness mechanically with
linters, CI jobs, and a recurring "doc-gardening" agent that scans for stale
documentation and opens fix-up PRs — "garbage collection" run continuously
rather than every Friday. When documentation falls short, "we promote the rule
into code."

Source: https://openai.com/index/harness-engineering/

### 2.6 Superpowers — the plan ledger (survives compaction)

Superpowers' `subagent-driven-development` skill uses a per-plan **ledger file**
(`.superpowers/sdd/<plan>/progress.md`) as its recovery map. It is checked at
session start (its first line names the plan; tasks with a "complete" line are
done), appended at every milestone ("fix round 1/5", "complete (commits…)").
The skill's stated reason is blunt: "Conversation memory does not survive
compaction. Controllers that lost their place have re-dispatched entire
completed task sequences — the single most expensive failure observed." The
ledger is the one artifact that is trusted over the agent's own recollection
after compaction.

Local: `~/.commandcode/skills/subagent-driven-development/SKILL.md`.

### Synthesis

| Mechanism | Written by | Read when | Curated | Kept from becoming noise by |
|---|---|---|---|---|
| Command Code `AGENTS.md` | human | every turn (system prompt) | human edits | small file + `@path` imports; `/context` shows token cost |
| Command Code Taste | automatic | every turn | automatic | one md per category, confidence score per entry |
| Claude Code `MEMORY.md` | human | on demand | human | short index → detail files |
| Anthropic progress file | agent | mandatory at session start | append-only log | short log, work-scoped |
| OpenAI docs/ | agent + human | on demand via TOC | recurring doc-gardening agent | ~100-line AGENTS.md map + linters + garbage-collection cadence |
| Superpowers ledger | agent | mandatory at session start | append-only per plan | per-plan file, deleted when plan completes |

The two mechanisms that actually **deliver** learning (Anthropic, superpowers)
share a shape: the read is a mandatory, scripted first step of the session, and
the write is a mandatory last step. The two mechanisms that are **curated**
(Claude Code index, OpenAI doc-gardening) both treat the memory as something
that must be actively kept small or it becomes noise. No examined mechanism
relies on an agent voluntarily reading a memory file mid-task.

---

## 3. Curation: what makes an entry worth keeping

No tool examined curates lessons by compaction or expiry of individual entries.
Curation in practice is one of three shapes:

1. **A short read surface + detail behind it** (Claude Code): the index stays
   3–5 lines because the index is what gets read; the detail lives in files
   that are only opened when relevant.
2. **A table-of-contents map + active garbage collection** (OpenAI): the
   injected file stays ~100 lines; a recurring agent scans for stale rules and
   opens fix-up PRs; linters mechanically enforce freshness. "Pay down debt
   continuously in small increments rather than letting it compound."
3. **Confidence scoring** (Taste): the only automatic mechanism, and it is the
   only one that tracks how sure the system is that a lesson still holds.

None of them solve the Mill problem, because Mill's lessons are **not read at
all** — a lesson cannot be curated into relevance if nothing reads it. The
OpenAI finding is the operative one: "a monolithic manual turns into a
graveyard of stale rules. Agents can't tell what's still true, humans stop
maintaining it, and the file quietly becomes an attractive nuisance."

**What makes an entry worth keeping:** a lesson that changes what a role does
next time. Anything a gate or check already enforces is not a lesson — it is a
mechanised rule, and it belongs in a script, not a prose file (staff lesson #10:
"`exit 1` is enforcement. Markdown is documentation"). A lesson that merely
records what happened, without a "do this instead next time", is journaling.

---

## 4. Delivery: the hard part is not storage

The three delivery options, with their costs:

**Option A — inject every lesson into every brief.** The coordinator reads
`.mill/roles/<role>/lessons.md` and pastes it into each dispatch brief.
Cost: near zero tokens per brief (Mill's lessons are short); the coordinator
already constructs every brief. Risk: it makes lessons read, but it does not
make them *used* — a brief full of stale lessons is exactly the "too much
guidance becomes non-guidance" failure OpenAI measured. It also pushes the
coordination burden onto Staff, who must remember to include it, which is the
same "prose, not mechanism" failure.

**Option B — reference by path, read on demand.** The brief tells the worker
"read `.mill/roles/<role>/lessons.md` before starting." This is what COMMON.md
already claims lessons are ("reference material"), and it has already failed
for seventeen lessons. A path reference is not delivery — it is a suggestion,
and an agent that is optimising for completion over compliance will skip it
exactly as it skips every other suggestion (staff lesson #10).

**Option C — promote into the ROLE.md itself once proven.** The lesson is
written into the role definition (or COMMON.md) so it is part of the role's
context whenever that role executes, then removed from the lessons file.
Cost: a promotion step in the coordinator's post-dispatch review; the read is
free because ROLE.md is already loaded into every worker's context. This is
the only option where the read cannot be skipped, because ROLE.md is not
optional context — it is the role's contract. It matches how Command Code
delivers memory (into the prompt, every turn) and how OpenAI ends the loop
("when documentation falls short, we promote the rule into code").

**Recommendation: Option C, with a mandatory startup read as the interim
mechanism.** The product requirement is that learning "arrive in the role's
context when that role next executes." The only context every role is
guaranteed to receive is its ROLE.md. So:

- **New lessons** are recorded by the coordinator after a dispatch, in
  `.mill/roles/<role>/lessons.md` (append-only, short, one paragraph each).
- **The startup read is mechanised**: the dispatch brief opens with a fixed
  line — `Read .mill/roles/<role>/lessons.md first; its entries bind this
  task.` — mirroring Anthropic's scripted "get your bearings" step and the
  superpowers ledger check. This is the delivery mechanism until promotion
  makes it redundant.
- **Promotion is the curation step**: when a lesson survives one or two
  dispatches without being contradicted, the coordinator moves it into
  COMMON.md or ROLE.md and removes it from lessons.md. A lesson that stays in
  lessons.md across promotions is either not yet proven or not worth keeping.

This is three mechanisms with one delivery path: the coordinator writes new
lessons to lessons.md, the brief forces the read, and proven lessons graduate
into the role definition where the read is structural.

---

## 5. Scope: one mechanism or three?

Plainly: **one mechanism, three stores.** The delivery mechanism is the same
whether the knowledge is a role lesson, a project finding, or the coordinator's
session memory — it is a markdown file in the repo, and it reaches a worker
only when a brief (or a role definition) points at it. What differs is the
read surface:

- **Per-role lessons** (`.mill/roles/<role>/lessons.md`) — the highest-frequency
  store: they must arrive at every dispatch of that role. The startup-read +
  promotion loop above is built for this.
- **Project findings** (`docs/FINDINGS`, `docs/research/`) — read on demand,
  referenced by briefs that need them. OpenAI's `docs/` pattern applies: a map
  (AGENTS.md / COMMON.md) pointing at the research docs, with a doc-gardening
  pass for staleness.
- **Coordinator session memory** — the coordinator's own context across
  sessions. This is the one store that is *not* a role lesson and not a project
  finding; it is the memory the staff role carries about how delegation goes,
  and it belongs in `staff/lessons.md` — which is exactly where Mill already
  writes it (lessons 1–17 are all coordinator memory).

The trap is to build three delivery mechanisms for three stores. The stores
already exist and are in the right places; what is missing is the single
delivery mechanism (startup read + promotion) plus a decision about who
gardens the files. The `docs/FINDINGS` file is already the project-level
gardening artifact — it exists precisely because individual issues scatter and
"the *patterns* are what get forgotten."

---

## 6. What Mill should do, concretely

### Recommendation

**Keep lessons.md, but make it read — mechanically, at dispatch — and make
promotion into ROLE.md/COMMON.md the curation step. Do not delete it.**

The alternative — delete lessons.md because an unread file looks like learning
— would be throwing away the only record of what Mill's own failures taught,
to fix a delivery gap. The seventeen staff lessons contain at least three that
should already be enforced mechanically (lesson #10: gates are scripts, not
prose — already true in `checks/`; lessons #13/#17: verify by execution, not
report — already the coordinator's rule). The failure is not that the lessons
are wrong; it is that nothing reads them. Deleting the file would turn a
delivery problem into a data-loss event.

### What to implement first, and what it costs

**First step: the mandatory startup read, as a gate.** Add a fixed line to the
brief template in `roles/staff/ROLE.md` (the coordinator builds every brief
from it): "Read `.mill/roles/<role>/lessons.md` first; its entries bind this
task." This is a one-line text edit — near-zero cost, no new mechanism, and it
directly answers the product requirement that learning arrive in the role's
context. The brief already carries the role's context; this makes lessons part
of it.

**Second step: promotion as curation.** When a lesson survives one or two
dispatches without being contradicted, the coordinator moves it from
lessons.md into the role's ROLE.md (or COMMON.md, for lessons that bind all
roles) and deletes it from lessons.md. Cost: part of the coordinator's existing
post-dispatch review, which already happens for every result. This is the
OpenAI loop — "promote the rule into code" — applied to Mill's policy layer,
where ROLE.md is the code.

**Third step (optional): a doc-gardening cadence for lessons and FINDINGS.**
A recurring check that scans `.mill/roles/*/lessons.md` for lessons that have
outlived their promotion window (e.g. older than N dispatches, still unproven)
and flags them for deletion. This is the OpenAI garbage-collection pattern, and
it is what keeps lessons.md from becoming the "graveyard of stale rules" that a
monolithic manual becomes. It can be a bash check in `checks/` — Mill already
has the gate pattern.

### Costs

| Step | Cost | Mechanism needed |
|---|---|---|
| Startup-read line in brief template | near zero (text edit) | none new — coordinator already builds briefs |
| Promotion loop | small, ongoing coordinator time | none new — review already happens |
| Doc-gardening check | small (one bash gate) | new check in `checks/` |

The expensive part of a full "learning system" — vector stores, summarization
passes, a memory server — is precisely what the field says is dissolving in
favor of plain text files plus git history (harness-debt research,
`docs/research/harness-engineering-and-evals.md`, §2). Mill already has the
plain text files. It needs the read, and a decision about what stays.

---

## Sources

| Source | URL / path |
|---|---|
| Command Code memory docs | https://commandcode.ai/docs/memory |
| Command Code memory reference | `~/.nvm/.../command-code-knowledge/reference/memory.md` |
| Command Code Taste docs | https://commandcode.ai/docs/taste |
| Command Code Taste reference | `~/.nvm/.../command-code-knowledge/reference/taste.md` |
| Anthropic long-running agent harness | https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents |
| OpenAI harness engineering | https://openai.com/index/harness-engineering/ |
| Claude Code MEMORY (local, per-project) | `~/.claude/projects/-home-ksante-dev-rumai-labs-rumai/memory/` |
| Superpowers subagent-driven-development | `~/.commandcode/skills/subagent-driven-development/SKILL.md` |
| Mill's own FINDINGS | `docs/FINDINGS-2026-08.md` |
| Mill's product definition | `docs/PRODUCT.md` |
| Prior research: harness debt | `docs/research/harness-engineering-and-evals.md` |
