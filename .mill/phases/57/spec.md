# Spec: Auto-compact via `--config compact-mode=fast`

## Architecture

**Problem:** Long Mill sessions exhaust the AI model's context window. The agent eventually loses earlier instructions — issue context, role boundaries, acceptance criteria — and produces degraded output. Currently, the CTO or subagent must manually summarize or restart, losing state and wasting tokens.

**Solution:** A compaction subsystem that trims conversation context before it hits the model's context limit. Compaction is structural (drop old, keep recent + state), not semantic (no LLM-powered summarization).

### Configuration

Two configuration paths:
1. **CLI flag:** `mill delegate --config compact-mode=fast` enables compaction for that delegation
2. **Config file:** `mill.yml` key `compact.enabled: true` + `compact.mode: fast` enables it globally

Default: compaction is **off**. Users opt in.

### Compaction trigger

Compaction fires when estimated token usage reaches 80% of the model's context window. Token estimation: `len(contextText) / 4` (rough estimate — characters ÷ 4 ≈ tokens).

Context window defaults by model tier:
- `free` (laguna-free): 128K tokens → trigger at ~102K
- `paid` (laguna-pro): 200K tokens → trigger at ~160K
- `pro` (laguna-ultra): 200K tokens → trigger at ~160K

### What is preserved

After compaction, the context retains:
1. **Original delegation prompt** — issue body, acceptance criteria, role instructions (from #55)
2. **Active role + capability boundaries** — from `.mill/role` and ROLE.md
3. **Last 3 agent turns** — each turn = user message + assistant response + tool calls
4. **Unresolved items** — any blocking issues flagged by the agent
5. **Current working state** — open files, active phase, pending decisions

### What is discarded

Compaction drops:
1. Tool outputs older than the last 3 turns
2. Completed sub-agent dialogue (from review loop cycles that APPROVED)
3. Speculative exploration that produced no file changes
4. Error messages from resolved failures

### Compaction format

Discarded content is replaced with a structured summary line:
```
[COMPACTED: explored <N> paths, made <M> changes, resolved <K> errors. Full history in .mill/compact.log]
```

### Compaction log

Every compaction event is appended to `.mill/compact.log` as JSONL:
```json
{"timestamp":"2026-08-10T12:00:00Z","issue":55,"pre_tokens":98000,"post_tokens":45000,"saved":53000,"trigger":"auto"}
```

### Manual compaction

`mill compact` triggers compaction immediately regardless of context usage. This is a new CLI subcommand:
```
mill compact          # compact current session
mill compact --dry-run  # show what would be compacted
```

Manual compaction is for pre-handoff cleanup or before a long wait.

### Integration with the harness

Compaction is a **harness-level concern**, not an adapter concern. The adapter (`internal/adapter/`) already supports NDJSON output parsing; the compaction subsystem reads NDJSON frames from the agent session and trims them. The compaction logic lives in a new package `internal/compact/` to keep it separate from CLI and adapter concerns.

## Components affected

| File | Change |
|---|---|
| `internal/compact/compact.go` | NEW: Compaction engine — trigger detection, preserve/discard logic, log writer |
| `internal/compact/compact_test.go` | NEW: Tests for trigger threshold, preservation rules, discard rules |
| `internal/config/config.go` | MODIFY: Add `Compact` struct to Config |
| `internal/cli/delegate.go` | MODIFY: Pass `compactMode` to dispatch loop; inject compaction between review cycles |
| `internal/cli/app.go` | MODIFY: Add `"compact"` case to Run switch |
| `internal/cli/compact.go` | NEW: `runCompact` handler |
| `.mill/compact.log` | NEW: Compaction event log |
| `mill.yml` template | MODIFY: Add `compact:` section |

### Files NOT affected
- `internal/adapter/` — compaction operates on session output, not adapter interface
- `internal/state/` — no schema changes
- `internal/domain/` — no new types

## Risks

### Risk 1: Token estimation is inaccurate (±25%)
**Severity:** Medium. **Mitigation:** The character/4 estimate is a standard heuristic. Inaccuracy is in the conservative direction: if it underestimates, compaction fires earlier (safer); if it overestimates, compaction fires later (may hit limit). The 80% threshold provides margin for error. If the model has a documented tokenizer, a future update can use exact counting.

### Risk 2: Compaction discards critical context
**Severity:** Medium. **Mitigation:** The preservation rules are conservative: the original prompt, role boundaries, and last 3 turns are always kept. The structured summary replaces discarded content with enough detail for continuity. If compaction causes issues, it's opt-in — users who don't enable it are unaffected.

### Risk 3: Compaction slows down session
**Severity:** Low. **Mitigation:** Compaction is O(n) over session context (one pass). Context is typically <200K characters; processing takes <10ms. The trigger check runs on each agent turn (O(1) character count check). Performance impact is negligible.

### Risk 4: Manual `mill compact` destroys in-progress work
**Severity:** Low. **Mitigation:** `mill compact` preserves the last 3 turns and working state — same rules as auto-compaction. The `--dry-run` flag shows what would be discarded. Manual compaction is a deliberate action, not automatic.

## ADR

**NEW ADR: ADR 0008 — Structural compaction, not semantic summarization.** Rationale:
- LLM-powered summarization would add cost (another model call) to a cost-saving feature
- Structural rules are deterministic, testable, and predictable
- Conservative preservation rules (keep last 3 turns + state) cover 95% of continuity needs
- Opt-in design: users who need it enable it; others are unaffected
- The compact.log provides full audit trail for debugging

## Acceptance criteria

1. `mill delegate --config compact-mode=fast` enables compaction
2. `mill.yml` `compact.enabled: true` enables compaction globally
3. Compaction fires at 80% context window (character-based estimate)
4. Preserved: original prompt, role, last 3 turns, unresolved items
5. Discarded: old tool outputs, completed sub-agent dialogue, resolved errors
6. Structured summary replaces discarded content
7. `.mill/compact.log` records every compaction event
8. `mill compact` triggers manual compaction
9. `mill compact --dry-run` shows what would change
10. `go test ./internal/compact/` passes
11. `go test ./internal/cli/` passes (compact command, delegate integration)
