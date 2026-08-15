# Spec: Read issue body and acceptance criteria for delegation prompts

## Architecture

**Problem:** `mill delegate` builds a prompt via `buildRolePrompt` but never reads the issue body. The subagent receives a generic prompt like "Work on issue #55 with target role sr-dev-fe" — no acceptance criteria, no spec reference, no issue context. The subagent must look up the issue itself, wasting tokens and risking misinterpretation.

**Solution:** Extend the `internal/issue` package with body reading, and integrate it into `buildRolePrompt`.

### Issue body reader (`internal/issue/reader.go`)

New function: `ReadBody(issueNum int) (body string, labels []string, err error)`

Implementation: shells out to `gh issue view <N> --json body,labels`. Parses the JSON output:
- `body` → raw issue body markdown
- `labels` → array of label name strings

Error handling:
- `gh` not found → `err` explains: "gh CLI not found. Install from github.com/cli/cli"
- Issue not found → `err` explains: "issue #N not found"
- Network failure → `err` wraps the underlying cause

### Acceptance criteria extraction

New function: `ExtractAcceptanceCriteria(body string) []string`

Scans the issue body for:
1. Checkbox lists: `- [ ]` or `- [x]` items
2. Numbered criteria: lines matching `^\d+\.\s+\*\*` (bold labels)
3. "Acceptance criteria" or "Acceptance Criteria" section headers followed by list items

Returns a structured list of criterion strings. If no criteria found, returns nil (not an error — some issues lack formal criteria).

### Prompt construction

`buildRolePrompt` is refactored to accept the issue body as a parameter:

```
buildRolePrompt(issueNum int, body string, ac []string, targetRole string) string
```

The prompt format:
```
# Issue #N: <title>

<full issue body>

## Acceptance Criteria
<numbered list of criteria>

## Spec Reference
.mill/phases/N/spec.md (read this file for architecture details)

## Role
You are: <targetRole>. Read .mill/roles/<targetRole>/ROLE.md before acting.
```

The title is extracted from the first `# ` heading in the body.

### Fallback

If `gh issue view` fails (network, missing CLI), `runDelegate` proceeds with a degraded prompt:
```
# Issue #N

⚠ Issue body could not be read: <reason>
Proceeding with limited context. Read the issue at: https://github.com/<owner>/<repo>/issues/N
```

### Integration with #60

This SPEC defines the *input source* for delegation prompts. #60 (delegate invoke AI) consumes this input. The two are co-dependent but independently spec'd:
- #55: where does the prompt come from?
- #60: how is the prompt used (review loop, stage routing)?

## Components affected

| File | Change |
|---|---|
| `internal/issue/reader.go` | NEW: `ReadBody`, `ExtractAcceptanceCriteria` |
| `internal/issue/reader_test.go` | NEW: Tests for body parsing, criteria extraction, error paths |
| `internal/cli/delegate.go` | MODIFY: `runDelegate` calls `ReadBody`, passes body to `buildRolePrompt` |
| `internal/cli/delegate_test.go` | MODIFY: Tests for body-integrated prompts, fallback behavior |

### Files NOT affected
- `internal/adapter/` — no changes; prompt construction is CLI-level
- `internal/state/` — no schema changes
- `internal/domain/` — no new types

## Risks

### Risk 1: `gh` CLI dependency for core delegation flow
**Severity:** Medium. **Mitigation:** `gh` is already a hard dependency (Mill is GitHub-issue-driven). The fallback path allows delegation without issue body — the agent gets a warning and works from the issue URL. The FRD explicitly requires this fallback (#5). If `gh` is absent, the user gets a clear install instruction, not a cryptic error.

### Risk 2: Issue body is very large (>10KB) and bloats the prompt
**Severity:** Low. **Mitigation:** The body is included as-is — no truncation. Large issue bodies are the user's problem (they wrote the body). If this becomes an issue, a future config flag `max_prompt_size` can truncate. For now, the existing 100-turn limit in `runDelegate` caps the conversation length regardless of prompt size.

### Risk 3: Acceptance criteria extraction is heuristic and may miss criteria
**Severity:** Low. **Mitigation:** The full body is included regardless — criteria extraction is for *highlighting* only. If extraction misses criteria, they are still present in the body text. The extraction is a convenience, not a dependency. Tests cover the three detection patterns; edge cases degrade gracefully.

### Risk 4: `gh issue view` network call adds latency
**Severity:** Low. **Mitigation:** One HTTP call per delegation (sub-second for GitHub API). This is negligible compared to the agent session time (minutes). The call is made synchronously before dispatching — if it fails, the fallback path takes the same time.

## ADR

No new ADR. Issue body reading is a data access concern within `internal/issue/` — not a cross-cutting architectural decision.

## Acceptance criteria

1. `mill delegate <issue>` reads issue body via `gh issue view` and includes it in the prompt
2. Acceptance criteria are extracted and listed in a structured section
3. SPEC/FRD reference is included when `.mill/phases/<N>/spec.md` exists
4. Fallback works when `gh` is unavailable (warning + proceed)
5. `buildRolePrompt` output follows the defined format (issue, criteria, spec, role)
6. `go test ./internal/issue/` passes (new reader tests)
7. `go test ./internal/cli/` passes (updated delegate tests)
