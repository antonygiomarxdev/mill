# Spec: Adapter capability introspection, version tracking, prompt enrichment

## Architecture

**Problem:** Mill's `Adapter` interface only reports supported models. When delegating an agent session, mill cannot tell the agent what read-tool features are available (selectors, ceilings, recovery notes), cannot detect when concurrent agents silently overwrite each other's files, and cannot enrich prompts with harness-specific guidance. This leads to agents reading entire 80K-line files instead of using selectors, and to silent data loss when two agents race on the same file.

**Solution:** Three interconnected changes:

1. **Capability model** — Extend `Capabilities` with read-tool features. Each adapter self-reports its harness's capabilities. The model covers line/byte/char ceilings, selector support, and recovery notes.

2. **Version tracking** — Add per-file version counters in the ledger. Before an agent writes a file, the pre-commit hook checks whether another agent has written a newer version since the current agent last read it. On conflict, the commit is blocked and the agent must re-read.

3. **Prompt enrichment** — `buildRolePrompt` includes a "Read Tool Capabilities" section generated from the adapter's capabilities. This teaches the agent to use selectors, raw mode, and recovery notes where the harness supports them.

### Design principles

- **Eager, not lazy.** Query capabilities once at delegation start, before side effects, and pass through to prompt construction. Capabilities don't change mid-session.
- **Fail-open for version tracking.** If the ledger is unreadable, conflict detection is disabled (with a warning), not fatal. Better to risk a conflict than to block all delegation.
- **Fail-safe for capabilities.** If an adapter's capability introspection fails, use conservative defaults (no features assumed). The agent gets a degraded prompt, not a crash.
- **Hook-enforced, not agent-enforced.** The agent doesn't check versions; the pre-commit hook does. This keeps the agent simple and enforcement reliable.

### What does NOT change

- The `Adapter` interface shape remains: `Capabilities() Capabilities` returns a struct (no error). Failures are handled inside the adapter, which returns default/fallback values.
- The dispatch loop (`runDispatchLoop`) structure is unchanged — capability query and prompt enrichment are inserted before the loop, not inside it.
- Existing ledger entries (`dispatch`, `review`, `complete`) are unchanged. Version entries are a new event type, not a modification to existing events.
- The slot manager, worktree lifecycle, scaffold, and role hooks are unchanged.

---

## 1. Capability model

### Extended `Capabilities` struct

```go
// Capabilities describes what an adapter can do.
type Capabilities struct {
    Models          []string `json:"models"`

    // ReadTool describes the harness read tool's limits and features.
    // A zero value means "not applicable / use defaults".
    ReadTool ReadToolCapabilities `json:"read_tool"`
}

// ReadToolCapabilities describes the forwarder harness's read tool.
type ReadToolCapabilities struct {
    // LineCeiling is the maximum number of lines the read tool returns
    // in a single call. 0 means unlimited.
    LineCeiling int `json:"line_ceiling"`

    // ByteCeiling is the maximum total bytes the read tool returns
    // in a single call. 0 means unlimited.
    ByteCeiling int `json:"byte_ceiling"`

    // CharCeiling is the maximum characters per displayed line before
    // the tool truncates a line mid-display. 0 means unlimited.
    CharCeiling int `json:"char_ceiling"`

    // HasSelectorSupport is true when the read tool accepts line-range
    // selectors (e.g. :50-200, :raw, :50+150, :conflicts).
    HasSelectorSupport bool `json:"has_selector_support"`

    // HasRecoveryNotes is true when the read tool emits truncation
    // indicators (e.g. "[TRUNCATED: 1200 lines omitted]") instead of
    // silently dropping content.
    HasRecoveryNotes bool `json:"has_recovery_notes"`
}
```

### Capability semantics

| Field | Meaning | Zero value |
|---|---|---|
| `LineCeiling` | Max lines per read call | Unlimited (harness has no line cap) |
| `ByteCeiling` | Max bytes per read call | Unlimited |
| `CharCeiling` | Max chars per displayed line before truncation | Unlimited (lines are never truncated) |
| `HasSelectorSupport` | Selectors like `:50-200`, `:raw` work | Selectors unavailable (agent must read whole files) |
| `HasRecoveryNotes` | Truncation emits recovery indicators | Truncation is silent (agent won't know data was dropped) |

### How adapters report capabilities

**CommandCodeAdapter** — hardcoded values matching the harness:
```go
func (a *CommandCodeAdapter) Capabilities() Capabilities {
    return Capabilities{
        Models: []string{
            "claude-sonnet-5", "claude-sonnet-4-6",
            "deepseek-v4-pro", "deepseek-v4-flash", "gpt-5",
        },
        ReadTool: ReadToolCapabilities{
            LineCeiling:         2000,
            ByteCeiling:         128 * 1024, // 128KB
            CharCeiling:         500,
            HasSelectorSupport:  true,
            HasRecoveryNotes:    true,
        },
    }
}
```

**Future adapters** (OpenCode, Claude) set appropriate values. An adapter whose harness has no read tool (e.g., a mock/test adapter) returns the zero value for `ReadTool` — this signals "use conservative defaults" to the prompt builder.

**Fallback contract:** If an adapter's capability introspection involves an external process call (e.g., `cmd --read-tool-info`) and that call fails, the adapter MUST return the zero value for `ReadToolCapabilities` and MUST NOT return an error. The prompt builder treats the zero value as "no features available."

---

## 2. Capability query flow

### Placement in dispatch

Capabilities are queried **once, early, before side effects.** The optimal insertion point is in `runDelegate`, after binary validation and before worktree creation:

```
runDelegate flow (new insertion in bold):

 1. Parse args, extract --role, --model
 2. Read issue body + labels from GitHub
 3. Determine active role, validate delegation chain
 4. Load config
 5. validateDelegateBinaries(cfg)          ← existing
 6. **caps := a.Adapter.Capabilities()**    ← NEW: query capabilities
 7. **log capabilities to stderr (debug)**  ← NEW: observable per AC 1
 8. Initialize slot manager
 9. Create task + persist state
10. Append dispatch ledger entry
11. createWorktree(issueNum)
12. Scaffold, write role, install hooks
13. **buildRolePrompt(..., caps)**           ← MODIFIED: passes caps
14. DispatchOpts → acquire slot → Dispatch
```

### Rationale

- **Before worktree creation** because the worktree is a heavyweight side effect. If capability query somehow panics (shouldn't happen per fallback contract), we fail fast before creating git state.
- **After binary validation** because `validateDelegateBinaries` confirms the adapter's CLI is on PATH — the adapter instance is valid at this point.
- **Eager, not lazy.** Querying once and passing through avoids repeated calls, thread-safety issues, and stale reads. Capabilities don't change between adapter restarts.
- **Not inside `runDispatchLoop`** because capabilities are invariant across the produce→review cycle. The same adapter handles both phases.

### Thread safety

For async delegation (`mill delegate 42` without `--wait`), the goroutine captures `caps` by value before spawning — no shared mutable state. The synchronous path (`--wait`) also captures `caps` and passes it into `runDispatchLoop`.

---

## 3. Version tracking

### Schema: version ledger entries

Version data lives in the same ledger file as event entries (`.mill/ledger/<issue>.jsonl`), using two new event types:

```go
// VersionedEntry extends ledger.Entry with file-level version fields.
// Only populated for "file_read" and "file_write" events.
type VersionedEntry struct {
    Timestamp time.Time `json:"timestamp"`
    Issue     int       `json:"issue"`
    Event     string    `json:"event"`    // "file_read" or "file_write"
    File      string    `json:"file"`     // worktree-relative path (e.g. "internal/cli/delegate.go")
    Version   int       `json:"version"`  // monotonically increasing per-file counter
    AgentID   string    `json:"agent_id"` // "produce" or "review" (matches dispatch phase)
}
```

**Note on implementation:** Rather than creating a new entry struct, the existing `ledger.Entry` gains optional `File`, `Version`, and `AgentID` fields (zero-valued when not applicable). This keeps a single `Append` path and a single JSONL file. For clarity, this spec uses `VersionedEntry` as a conceptual type.

### Version lifecycle

```
Agent reads file F:
  → ledger.Append("file_read", File: "internal/cli/delegate.go", Version: V, AgentID: "produce")
  (records the version the agent read — the anchor for conflict detection)

Agent modifies and commits file F:
  → pre-commit hook fires
  → hook checks ledger for conflicts (see algorithm below)
  → if no conflict: ledger.Append("file_write", File: "internal/cli/delegate.go", Version: V+1, AgentID: "produce")
  → commit proceeds
```

Initial state: a file with no ledger entries is at version 0. The first agent to read it records `file_read` at version 0. The first agent to write it records `file_write` at version 1.

### How versions are captured

The agent process emits NDJSON output. After `session.Wait()`, mill scans the output for read-tool and write-tool calls:

- **Read events:** Look for NDJSON frames where `tool_name == "read"` (or harness equivalent). Extract the first file path argument. Record a `file_read` entry at the file's current version (latest `file_write` version, or 0 if none).
- **Write events:** Version bump happens in the pre-commit hook, not in output scanning. The hook knows which files are in the staged commit (`git diff --cached --name-only`).

This means version tracking has two phases:
1. **Post-session output scan** — record `file_read` events from the agent's tool calls
2. **Pre-commit hook** — check for conflicts, then record `file_write` events on successful commit

### Conflict detection algorithm

The algorithm runs in the pre-commit hook, invoked before every `git commit` inside the worktree:

```
For each file F in staged commit (git diff --cached --name-only):

  1. READ_HISTORY = all "file_read" entries for F, by this agent, ordered by timestamp DESC
     → if empty: no read tracked → SKIP (first write is always allowed)

  2. LATEST_READ = READ_HISTORY[0].Version   // version agent last read

  3. WRITE_HISTORY = all "file_write" entries for F, by ANY agent, ordered by timestamp DESC

  4. LATEST_WRITE = WRITE_HISTORY[0].Version   // version last written by anyone

  5. LATEST_WRITER = WRITE_HISTORY[0].AgentID  // who wrote it last

  6. CONFLICT if:
     a. LATEST_WRITE > LATEST_READ  (someone wrote after we read)
     OR
     b. LATEST_WRITE == LATEST_READ AND LATEST_WRITER != this agent
        (we're writing at a version we didn't read from)

  7. On CONFLICT:
     - Print to stderr: "BLOCKED: version conflict on <F> (read v<LATEST_READ>, current v<LATEST_WRITE> by <LATEST_WRITER>)"
     - Append to .mill/enforcement.log: "BLOCKED\t<F>\tread v<LATEST_READ>\tcurrent v<LATEST_WRITE>\tby <LATEST_WRITER>"
     - Exit with non-zero (blocks commit)

  8. On NO CONFLICT:
     - Record "file_write" with Version = LATEST_WRITE + 1, AgentID = this agent
     - Allow commit
```

### Edge case: first commit

If a file has never been written (no `file_write` entries), LATEST_WRITE is 0 and LATEST_WRITER is empty. The first agent to write always succeeds (0 is not > any read version, and empty writer triggers no ownership check).

### How the hook knows its agent identity

The agent identity ("produce" or "review") is written into the worktree during scaffold: `.mill/agent_id` contains the phase name. The pre-commit hook reads this file to get its identity. This file is created in `runDelegate` alongside the `.mill/role` file.

### What does NOT happen

- The agent does NOT check versions before calling its write tool. Enforcement is hook-side, not agent-side.
- Version entries do NOT replace existing `dispatch`/`review`/`complete` entries. They coexist in the same JSONL file.
- Files not tracked by git are not versioned. The hook only checks staged files.
- The hook does NOT prevent the agent from reading stale data. It only prevents committing stale writes. If the agent reads stale data and produces wrong output, the review loop catches it.

---

## 4. Prompt enrichment

### Modified `buildRolePrompt`

The function signature changes to accept capabilities:

```go
func buildRolePrompt(issueNum int, targetRole string, issueBody string, caps adapter.Capabilities) string
```

### Capabilities section format

When `caps.ReadTool` is non-zero (at least one field set), the prompt includes a "Read Tool Capabilities" section after the role frontmatter and before the issue body. When `caps.ReadTool` is zero-valued, the section is omitted.

### Example: CommandCode adapter (full capabilities)

```
<role frontmatter>
---
Work on GitHub issue #65.

## Read Tool Capabilities

Your harness provides a read tool with the following features:

- **Line ceiling:** 2000 lines per read. Files longer than this are truncated.
- **Byte ceiling:** 128KB per read.
- **Char ceiling:** 500 chars per displayed line. Longer lines are truncated mid-display.
- **Selectors:** Supported. Append `:<range>` to file paths to read specific portions:
  - `path:50-200` — lines 50 through 200 (inclusive)
  - `path:50` — line 50 only
  - `path:50-` — from line 50 to end
  - `path:50+150` — 150 lines starting at line 50
  - `path:raw` — verbatim output (no line numbers)
  - `path:50-200:raw` — combined selector + raw mode
- **Recovery notes:** YES — when output is truncated, the harness appends a note like
  `[TRUNCATED: 1200 lines omitted — re-read with a narrower selector]`.
  If you see a recovery note, narrow your selector range and re-read.

**Guidance:**
- Prefer reading specific ranges with selectors over whole-file reads.
- When recovery notes indicate truncation, re-read with a narrower selector.
- Re-read a file before writing it to ensure you have the latest version.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.

**Issue Body:**
<issue body>
```

### Example: minimal adapter (no read tool features)

When `ReadToolCapabilities` is all zeros (no selector support, no ceilings, no recovery notes):

```
<role frontmatter>
---
Work on GitHub issue #65.

Read the codebase, make the necessary changes, and when you are done,
end your response with a verdict line: APPROVED, NEEDS CHANGES, or REJECTED.

**Issue Body:**
<issue body>
```

No capabilities section is emitted. This is backward-compatible with existing prompt format.

### Example: partial capabilities (ceilings but no selectors)

```
## Read Tool Capabilities

Your harness provides a read tool with the following features:

- **Line ceiling:** 1000 lines per read.
- **Byte ceiling:** 64KB per read.
- **Recovery notes:** YES.

**Guidance:**
- Be aware that large files may be truncated. If a file appears incomplete, focus on specific sections.
- When recovery notes indicate truncation, read smaller portions of the file.

```

Only non-zero fields are rendered. Selector guidance is omitted because `HasSelectorSupport` is false.

### `buildReviewPrompt` changes

The review prompt also receives capabilities, but the capabilities section is lightweight — the reviewer agent doesn't read code, it reviews output:

```
You are a code reviewer for mill, an agent delegation harness.
Review the following work product for GitHub issue #65.

**Issue Body:**
<issue body>

**Work Product (produce agent output):**
<output>

Evaluate whether the work product satisfies all acceptance criteria in the issue body.
End your review by emitting EXACTLY ONE of these signals on stderr:
- APPROVED: if the work is complete and correct
- CHANGES_REQUESTED: if the work needs modifications
- BLOCKED: if you cannot complete the review
```

No read-tool section is added because the reviewer reads `output.txt`, not source files. If future review flows need the reviewer to read source, the section can be added.

---

## 5. Error handling

### 5a. Capability query failure

**Contract:** `Capabilities()` returns a struct, never an error. If the adapter's internal capability detection fails (e.g., external process call times out), the adapter MUST return the zero value for `ReadToolCapabilities` and MUST log a warning to its own output.

**Mill's response:** No special handling needed — mill receives a valid struct. The zero-valued `ReadToolCapabilities` causes prompt enrichment to emit no capabilities section (degraded, but functional). The `Models` field is still expected to be populated; if it's empty (adapter entirely broken), `resolveModel` will fall back to `cfg.Model`.

**Observability:** After querying capabilities, mill writes a debug line to stderr:
```
delegate: adapter capabilities — models=5 selectors=true recovery=true line_ceiling=2000 byte_ceiling=131072
```

This satisfies AC 1 ("observable: capability check in dispatch flow").

### 5b. Version conflict detected

**Where:** Pre-commit hook inside the worktree.

**What happens:**
1. Hook prints conflict details to stderr — the agent process captures this in its `Stderr` field.
2. Hook appends to `.mill/enforcement.log`: `BLOCKED	<file>	read v<N>	current v<M>	by <agent>`
3. Hook exits with code 1 — git commit is blocked.

**Downstream effects:**
- The agent sees the commit failure in its tool output. If it's adaptive, it re-reads the file and retries the commit.
- When the agent session ends, `session.Wait()` returns `Stderr` containing the conflict messages.
- `classifyResult` does NOT have a specific case for version conflicts — the stderr won't match any classification signal, so it falls through to exit-code mapping. A blocked commit exits with exit code 1, which maps to `FATAL` (in `classifyResult`, exit code 1 is not listed, so it maps to `FATAL` via the default case).
- **This is correct behavior.** A version conflict during the agent session means the agent produced output based on stale data. The review loop's `FATAL` path triggers a retry on the next round, where the agent gets fresh state.

**Enforcement.log processing** (existing at `runDispatchLoop:382-391`):
- After the session, `runDispatchLoop` reads `.mill/enforcement.log`
- If it contains `BLOCKED`, the `needs:rework` label is added to the issue
- This labels the issue for human attention even if the agent eventually recovers

### 5c. Ledger read failure

Three sub-cases:

| Case | Behavior |
|---|---|
| **Ledger file does not exist** (`.mill/ledger/<issue>.jsonl` missing) | Normal cold-start. All files assumed at version 0. No `file_read`/`file_write` entries. First writes always succeed. |
| **Ledger file exists, line fails to parse** | Skip the malformed line, log a warning to stderr: `ledger: skipping malformed line N in <path>: <parse error>`. Continue scanning remaining lines. A corrupt line does not invalidate the whole file. |
| **Ledger file cannot be opened** (permissions, disk full, I/O error) | **Fail open.** Log a warning to stderr: `ledger: cannot read version data from <path>: <error> — conflict detection disabled`. Treat as if no version data exists. The commit proceeds without conflict checks. This prevents a disk error from blocking all delegation. |

**Rationale for fail-open:** The ledger is a safety net, not a security boundary. Losing conflict detection is better than losing the ability to delegate. The review loop catches stale-output problems at a higher level.

### 5d. Ledger write failure

If `ledger.Append` fails (disk full, permissions):
- **For `file_read` entries:** Log a warning, continue. The agent can still work; conflict detection will be incomplete for this file.
- **For `file_write` entries:** Log a warning, allow the commit. Better to lose version tracking than to block legitimate work.
- **For core events** (`dispatch`, `review`, `complete`): Existing behavior unchanged — return the error to the caller.

---

## Components affected

| File | Change |
|---|---|
| `internal/adapter/adapter.go` | MODIFY: Extend `Capabilities` with `ReadTool ReadToolCapabilities`. Add `ReadToolCapabilities` struct. |
| `internal/adapter/commandcode.go` | MODIFY: `Capabilities()` returns populated `ReadToolCapabilities`. |
| `internal/adapter/adapter_test.go` | MODIFY: Update mock adapter and capability tests for new fields. |
| `internal/adapter/commandcode_test.go` | MODIFY: Verify `ReadToolCapabilities` fields are populated. |
| `internal/cli/delegate.go` | MODIFY: Query capabilities in `runDelegate` after binary validation. Pass `caps` to `buildRolePrompt` and `buildReviewPrompt`. Write `.mill/agent_id` to worktree. |
| `internal/cli/delegate.go` (`buildRolePrompt`) | MODIFY: Accept `caps adapter.Capabilities`, emit capabilities section when non-zero. |
| `internal/cli/delegate.go` (`buildReviewPrompt`) | MODIFY: Accept `caps adapter.Capabilities` (currently unused but wired for future). |
| `internal/cli/delegate.go` (`preCommitHookScript`) | MODIFY: Add version conflict detection logic to the pre-commit hook script. |
| `internal/cli/delegate_test.go` | MODIFY: Update fake adapter, test prompt enrichment, test capability query placement. |
| `internal/cli/review_loop_test.go` | MODIFY: Update multiResultAdapter capabilities. |
| `internal/ledger/ledger.go` | MODIFY: Add `File`, `Version`, `AgentID` fields to `Entry`. Add `ReadEntries` function to scan and parse a ledger file. |
| `internal/ledger/ledger_test.go` | MODIFY: Test new Entry fields, test `ReadEntries` with valid, empty, and malformed lines. |

### Files NOT affected

- `internal/domain/` — no new domain types needed (version fields are in ledger, not domain)
- `internal/state/` — no state schema changes
- `internal/cli/app.go` — no routing changes
- `internal/cli/watch.go` — no changes
- Any files outside `internal/adapter/`, `internal/cli/`, `internal/ledger/`

---

## Risks

### Risk 1: Pre-commit hook performance on large commits
**Severity:** Low. **Mitigation:** The hook scans only files in the staged commit (`git diff --cached --name-only`). Typical agent commits touch 1–5 files. The ledger scan reads the entire JSONL file linearly, but ledger files are small (hundreds of lines, not thousands). If performance becomes an issue, a future optimization could index versions in a separate file.

### Risk 2: Version tracking gaps — agent reads without mill detection
**Severity:** Medium. **Mitigation:** Version tracking depends on scanning the agent's NDJSON output for `read` tool calls. If the agent uses a different tool name or the output format changes, reads go undetected. The pre-commit hook treats "no read tracked" as "skip conflict check" (not "block the commit"), so undetected reads don't cause false conflicts — they just lose protection. The risk is reduced conflict detection coverage, not broken commits.

### Risk 3: Two agents in the same phase (e.g., two "produce" agents)
**Severity:** Low (current architecture doesn't allow it). **Mitigation:** The slot manager prevents concurrent delegates for the same issue. Within a single dispatch loop, produce and review never overlap — review starts after produce finishes. If this changes in the future, the `AgentID` field distinguishes agents even within the same phase.

### Risk 4: Capabilities drift between query time and session time
**Severity:** Very low. **Mitigation:** Capabilities are queried once per delegation. The adapter binary doesn't change mid-session. If an operator upgrades the harness between delegations, the next delegation picks up new capabilities.

### Risk 5: Malformed ledger line blocks version scanning
**Severity:** Low. **Mitigation:** `ReadEntries` skips unparseable lines and continues. A single corrupt line doesn't block the scan. The warning is printed to stderr for operator visibility.

---

## ADR

### ADR-0065: Version tracking uses pre-commit hook, not agent-side enforcement

**Decision:** File version conflict detection runs in the git pre-commit hook, not in the agent's tool-calling loop.

**Rationale:**
- The agent is a black-box process controlled by a third-party harness. We cannot inject version-checking logic into its tool calls.
- The pre-commit hook is the natural gate for writes — it runs before every commit, has access to the file list, and can read the ledger.
- Hook-side enforcement is more reliable than scanning agent output (which can vary by harness version).

**Alternatives considered:**
1. **Agent-side enforcement via prompt instruction:** Tell the agent "re-read before writing." Rejected — advisory, not enforceable. Agents may ignore it.
2. **Post-commit detection:** Detect conflicts after the commit lands and revert. Rejected — more complex, irreversible, and the revert itself could conflict.

**Consequences:**
- The hook must have access to the ledger path (written into the worktree during scaffold).
- The hook must be fast — scanning all version entries linearly in bash could be slow. This is acceptable for small ledger files typical of single-issue work.

---

## Acceptance criteria

1. `mill delegate <N>` queries adapter capabilities and logs them to stderr before worktree creation (AC 1)
2. `Capabilities()` on `CommandCodeAdapter` returns populated `ReadToolCapabilities` with line ceiling 2000, byte ceiling 128KB, char ceiling 500, selector support true, recovery notes true
3. `Capabilities()` on a mock adapter with zero-valued `ReadTool` produces no capabilities section in the prompt
4. Agent prompt for CommandCode includes "Read Tool Capabilities" section with selector guidance and ceiling values
5. Agent prompt for a minimal adapter omits the capabilities section entirely
6. Version entries (`file_read`, `file_write`) are appended to the ledger with correct `File`, `Version`, and `AgentID` fields
7. Pre-commit hook blocks a commit when `LATEST_WRITE > LATEST_READ` and writes conflict details to `.mill/enforcement.log`
8. Pre-commit hook allows a commit when no conflicting writes exist
9. Pre-commit hook allows a first-ever write (no prior version entries)
10. `ReadEntries` skips malformed JSON lines and continues scanning (fail-open)
11. When ledger file does not exist, version tracking treats all files as version 0
12. `go test ./internal/adapter/` passes
13. `go test ./internal/cli/` passes
14. `go test ./internal/ledger/` passes
15. `go build ./...` passes
