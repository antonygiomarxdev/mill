# FRD: Model routing — cheap models write, expensive models review

## User need

Mill currently uses the same AI model for every role regardless of the task's cognitive demands. Writing boilerplate code or config uses the same expensive model as reviewing architecture decisions or debugging complex failures. This wastes money on simple tasks and underserves complex ones.

Role-model mapping must be configurable so that cheap models handle production work (writing code, generating config, formatting output) and expensive models handle judgment work (reviewing, debugging, designing).

## Functional requirements

1. **Model mapping by role.** Each role's `ROLE.md` frontmatter declares a `model:` field (`free`, `paid`, or `pro`). Mill reads this at delegation time and routes the subagent to the declared model tier. Example: PM uses `pro`, Sr. Dev uses `paid`, QA/Docs uses `free`.

2. **Three model tiers.** The system supports three tiers: (a) `free` — the cheapest available model, suitable for mechanical tasks (formatting, docs, simple code generation); (b) `paid` — mid-tier, for production code with moderate complexity; (c) `pro` — the most capable model, for architecture, debugging, review, and design.

3. **Provider mapping.** Mill maps model tiers to actual provider models via configuration (`mill.yml`). When a tier is unavailable (provider down, quota exceeded), Mill falls back to the next tier up — never down. A `free` model that fails falls back to `paid`; `paid` fails to `pro`; `pro` has no fallback and reports the error.

4. **CLI override.** `mill delegate --model pro` forces the target model tier for that delegation, overriding the role's default. This is a CTO/Staff privilege, not available to subagents delegating further.

5. **Cost tracking.** Every delegation logs: role, model tier used, estimated token count, and estimated cost. This data is written to `.mill/costs.jsonl` for audit. The format is one JSON object per line, append-only.

6. **Model availability check.** Before spawning a subagent, Mill verifies the requested model tier is reachable. If not, it falls back per rule #3 and logs a warning: "Model tier 'paid' unavailable for Sr.Dev — falling back to pro."

## Out of scope

- Dynamic model selection based on task content (e.g., "this looks hard, upgrade to pro"). Selection is role-based only.
- Per-provider cost optimization. The mapping is tier → model, not tier → cheapest-available-across-providers.
- Streaming or chunked response billing. Cost estimates are approximate based on reported token counts.
- Model fine-tuning or custom models.

## Priority

**P1** — quality of life. Cost optimization matters at scale but does not block current functionality. The default single-model behavior still works; this makes it smarter.
