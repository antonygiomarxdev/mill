# ADR 0001: Mill as a Framework on Top of the Harness

**Status:** accepted
**Deciders:** Staff, CTO

## Context

Mill currently operates as a standalone Go binary (`cmd/mill`) that spawns AI
agent sessions via raw CLI (`cmd -p`, `opencode run`) into git worktrees. This
model has three structural problems:

1. **Fights the harness instead of working with it.** The harness (omp, claude
   code, opencode) already provides context-file discovery, subagent spawning,
   tool validation, and session management. Mill reimplements all of these in
   Go — bypassing native mechanisms.

2. **No autonomous behavior.** The CTO must manually invoke `mill delegate
   <issue> --role <target>` for every delegation step. The agent in the CTO
   session doesn't know it IS Mill — it has no loaded skill, no role context,
   no delegation autonomy.

3. **Cascading delegation requires the adapter.** Subagents spawned via `task`
   cannot themselves spawn sub-subagents (the `task` tool is not available
   inside subagent sessions — verified 2026-08-10). The CLI adapter is the
   only way to achieve multi-level delegation chains. However, the adapter
   layer is overbuilt: two implementations (CommandCode + OpenCode), a generic
   interface, process management, JSON parsing, and exit-code scraping — for
   what is fundamentally "spawn an agent in a worktree."

The band-aid (commit 6cc1729, `copyScaffold`) fixed the immediate symptom
(agents in empty worktrees with no AGENTS.md), but introduced context
divergence: scaffold files are embedded at build time and diverge from the
live repo copies. Agents receive two conflicting context channels (CLI prompt
from live files, filesystem from stale embed).

## Decision

Mill becomes a **skill loaded into the CTO session**. The skill provides:

1. Tool detection (omp / claude code / opencode / copilot)
2. Autonomous role classification (Staff vs PM)
3. Delegation orchestration — decides native `task()` vs CLI fallback
4. Context delivery to the correct tool-specific directories
5. Agent type selection per role

### Architecture

```
CTO session (omp / claude / opencode)
  │
  └─ [Mill · Staff]  ← skill loaded at session start
       │
       ├─ Classifies user message → Staff or PM
       ├─ Loads roles/COMMON.md + roles/<role>/ROLE.md
       │
       ├─ Simple work (single role, no cascade)?
       │     └─ task(agent=X, model=Y, prompt=...)  ← harness-native
       │
       ├─ Cascade work (multi-role chain)?
       │     └─ mill delegate <issue> --role <target>  ← CLI fallback
       │           └─ Spawns agent process in worktree with full context
       │
       ├─ Blocked? (agent raises hand)
       │     └─ Staff documents blocker → resolves → re-spawns agent
       │
       └─ Ledger + state → .mill/ledger/<issue>.jsonl + .mill/state.json
```

### Adapter: survives as escape hatch, not core

The adapter layer is **reduced, not deleted**. One implementation replaces two:

- `internal/adapter/` keeps one adapter (the one matching the configured
  provider) as fallback for cascade delegation and harness-less environments
- The generic `Adapter` interface, `Session`, `Capabilities`, and
  `SessionResult` remain — they're the contract
- `Resume()` stays for reconnecting to in-flight sessions
- `internal/repair/` is deleted — the harness validates tool calls natively
- `internal/classify/` is merged into the delegate flow (no standalone package)

### Agent type per role

Each `roles/<role>/ROLE.md` gains an `agent` field in its YAML frontmatter:

```yaml
---
role: sr-dev-be
agent: cavecrew-builder   # ← NEW
model: free
delegates_to: [qa-docs]
reviewed_by: tech-lead
skills: [tdd, codebase-design, systematic-debugging]
---
```

Resolution rules:
- `agent` field present → use it as `task(agent=...)` when available natively
- `agent` field absent → default to `task` (full capabilities)
- Harness doesn't support that agent type → fall back to CLI dispatch
- Harness not detected (bare terminal) → CLI dispatch with generic agent

Known agent types: `task`, `scout`, `cavecrew-investigator`,
`cavecrew-builder`, `cavecrew-reviewer`. These are harness-defined, not
Mill-defined. Mill only references them.

### Tool detection

At skill startup:
1. Check for `.omp/` directory → omp harness
2. Check for `.claude/` directory → claude code
3. Check for `.opencode/` directory → opencode
4. Check for `.github/copilot-instructions.md` → github copilot
5. None found → bare terminal (CLI fallback only)

Context files are copied to the detected tool's expected locations. For omp,
this means `.omp/AGENTS.md` lives next to the session. For claude code,
`.claude/CLAUDE.md`. The skill copies from the live repo root — not from an
embedded binary snapshot — eliminating context divergence.

### Blocked workflow

When a subagent cannot proceed (ambiguous requirements, missing information,
conflicting constraints):

1. Agent returns with `BLOCKED` signal + description of what's unclear
2. Staff (or delegating role) documents the blocker in the ledger
3. Staff resolves: makes the decision, fetches missing info, or escalates to CTO
4. Staff re-spawns the agent with amplified context including the resolution
5. The original agent's partial work is preserved in its worktree

### CTO experience

The CTO speaks naturally to their agent. No manual CLI commands:

```
CTO: "necesito un dashboard de analytics"
Agent: [Mill · PM] → classifies, spawns UX → spawns UI → spawns QA
       → "dashboard listo en #51, coverage 92%, verdict: approved"
```

The skill makes Mill invisible. The agent IS Mill.

## Alternatives considered

- **Cascade real (cada rol spawnea al siguiente):** Each role spawns the next
  in its own session. Architect spawns Tech Lead, Tech Lead spawns Sr Dev.
  Rejected because `task` is not available inside subagent sessions (verified
  2026-08-10). The harness would need to propagate the tool.

- **Delete adapter entirely:** Remove `internal/adapter/` and rely exclusively
  on harness-native `task`. Rejected because `task` doesn't work in subagents
  → cascade delegation impossible without CLI fallback. Also leaves bare-terminal
  and non-omp harnesses unsupported.

- **Keep adapter as-is (status quo):** Two full adapter implementations,
  generic interface, process management, repair pipeline. Rejected because the
  adapter fights the harness (bypasses context discovery, reimplements session
  management) and Mill isn't autonomous (manual CLI invocations).

- **Orchestrator central sin skill:** Staff agent orchestrates via `mill
  delegate` commands but still requires manual CTO invocation. Rejected because
  it doesn't solve the autonomy problem — the CTO still drives delegation.

## Consequences

### Positive

- CTO session is autonomous: the agent classifies, delegates, verifies, reports
- Mill works with the harness instead of fighting it (uses `task` when possible)
- Single context channel (live files, not stale embed copies)
- One adapter instead of two — less code, less maintenance
- Agent type selection is explicit and role-scoped (`agent` in frontmatter)
- Harness-less environments still work via CLI fallback

### Negative

- Cascade is sequential, not parallel. Architect → Tech Lead → Sr Dev run in
  series from the Staff hub. Mitigation: independent sibling tasks can be
  spawned in parallel via `tasks[]` when Tech Lead decomposes into isolated
  units.
- `agent` field is Mill-specific YAML that non-Mill tooling ignores. This is
  acceptable — it's frontmatter in a Mill-owned file.
- Skill must duplicate some logic currently in Go (ledger writes, state
  updates). Mitigation: keep `mill` CLI subcommands (`mill ledger append`,
  `mill state upsert`) as thin wrappers the skill calls via bash.

### Risks

- **Retry/backoff without adapter:** The 3-attempt exponential backoff loop
  (`delegate.go:134-177`) must be reimplemented either in the skill (prompt
  instructions) or in a thin `mill run` wrapper. Skill-based retry is less
  precise (no exit codes). Wrapper-based retry keeps precision but adds a Go
  component.
- **Classification granularity:** `BLOCKED`, `AUTH`, `NO_CREDIT`,
  `RATE_LIMITED` signals currently come from exit codes and stderr scraping.
  The skill path must detect these from agent output text. Less reliable than
  exit codes.

## Migration plan

1. **ADR accepted** (this document)
2. **Add `agent` field** to all ROLE.md frontmatters
3. **Create `skills/mill.md`** — the Mill skill with tool detection, role
   classification, delegation logic
4. **Reduce adapter** to one implementation, merge classify, delete repair
5. **Add `mill run`** wrapper (optional) — thin binary for retry/classify
   when skill-based retry is insufficient
6. **Validate** — run the same issue through old and new paths, compare
   ledger + state + outcome
7. **Cutover** — CTO session loads Mill skill; `mill delegate` becomes
   secondary path

## References

- Issue #48 (Mill: Multi-Agent Delegation Harness as Framework)
- Issue #1 (Mill: Multi-Agent Delegation Harness)
- Commit 6cc1729 (scaffold worktree band-aid)
- `local:/scaffold-worktree-brief.md` (band-aid problem statement)
- Harness `task` subagent limitation verified 2026-08-10
