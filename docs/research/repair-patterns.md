# Tool Call Failure & Repair Patterns

Catalog of known tool call failure patterns from Command Code harness engineering
and rumai harness classification logic. Intended as input for mill's deterministic
repair layer.

---

## 1. Command Code Patterns (4+ documented, verified)

Source: https://commandcode.ai/docs/harness-engineering/tool-call-repairs
Author: Ahmad Awais, 2026-05-03

### 1.1 null-for-optional

| Field | Detail |
|-------|--------|
| **Symptom** | Model emits `{ timeoutMs: null }` for a field declared optional in schema |
| **Classification** | Shape error — valid JSON, invalid against Zod schema (null where field should be omitted) |
| **Root cause** | Post-training distribution doesn't distinguish "absent" from "null-valued"; model treats `null` as the sentinel for "not provided" |
| **Deterministic repair** | Walk Zod issue paths; for each `invalid_type` at an `.optional()` field where received value is `null`, delete the key from the input object |
| **Mechanisable in Go?** | **Yes.** Walk JSON object by issue path, delete key when Zod reports `expected: "undefined"` / `received: "null"` at an optional field |
| **Evidence** | Verified — primary source lines ~101–113 of doc |

### 1.2 json-array-parse

| Field | Detail |
|-------|--------|
| **Symptom** | Model emits `"[\"a\",\"b\"]"` — a JSON-serialized string — where schema expects `["a","b"]` |
| **Classification** | Shape error — valid JSON, wrong type (string instead of array) |
| **Root cause** | Model double-serializes: it composes a JSON string inside a JSON object, likely because its training distribution rewarded quoting array-like content in conversational text |
| **Deterministic repair** | At each Zod issue path where `expected: "array"` / `received: "string"`, attempt `JSON.parse` on the string value. If parse succeeds and yields an array, replace in-place |
| **Mechanisable in Go?** | **Yes.** Trivial: `json.Unmarshal` the string value, check type assertion for `[]interface{}`, replace |
| **Evidence** | Verified — primary source lines ~114–126 |
| **Ordering constraint** | MUST run before bare-string-wrap. If run after, `'["a","b"]'` becomes `['["a","b"]']` — a string inside a singleton array, wrong semantics |

### 1.3 empty-placeholder

| Field | Detail |
|-------|--------|
| **Symptom** | Model emits `{}` where schema expects an array like `["src/index.ts"]` |
| **Classification** | Shape error — valid JSON, wrong container type (object instead of array) |
| **Root cause** | Model uses `{}` as a generic "no shape" placeholder when it doesn't know how to populate a required array argument |
| **Deterministic repair** | At Zod issue path where `expected: "array"` / `received: "object"`, check if object is empty (`{}`). If so, cannot deterministically fill — but *can* detect and flag for retry with schema-aware error message |
| **Mechanisable in Go?** | **Partial.** Detection is mechanisable. Repair is not deterministic — you can't guess what array the model intended. Flagging for retry (with hint: "expected array") is the mechanisable path |
| **Evidence** | Verified — primary source lines ~127–137 |

### 1.4 bare-string-wrap

| Field | Detail |
|-------|--------|
| **Symptom** | Model emits `"foo"` (bare string) where schema expects `["foo"]` (array) |
| **Classification** | Shape error — valid JSON, wrong type (string instead of array) |
| **Root cause** | Model collapses a singleton array into its element; the `[]` wrapper is dropped, likely because in conversational text there's no distinction |
| **Deterministic repair** | At Zod issue path where `expected: "array"` / `received: "string"` AND `json-array-parse` already failed (not a stringified array), wrap the string in a singleton array: `["foo"]` |
| **Mechanisable in Go?** | **Yes.** After json-array-parse fails for that path, wrap in `[]interface{}{value}` |
| **Evidence** | Verified — primary source lines ~137–146 |
| **Ordering constraint** | MUST run after json-array-parse |

### 1.5 markdown-auto-link (path leakage)

| Field | Detail |
|-------|--------|
| **Symptom** | Model emits `filePath: "/Users/x/proj/[notes.md](http://notes.md)"` — a path field containing a Markdown autolink |
| **Classification** | Distribution leak — the chat distribution's auto-link behavior bleeds through the tool boundary |
| **Root cause** | Model was RLHF-rewarded for auto-linking in conversational output; it applies that prior even when the field is destined for `fopen`, not a chat bubble |
| **Deterministic repair** | Regex: match `[text](url)` where link text equals the URL without protocol. Unwrap only that degenerate case: `[notes.md](http://notes.md)` → `notes.md`. Real Markdown like `[click](https://x.com)` passes through untouched |
| **Mechanisable in Go?** | **Yes.** Two-line regex: `regexp.MustCompile(`\[([^\]]+)\]\(https?://\1\)`)` → replace with `$1` |
| **Evidence** | Verified — primary source lines ~147–176 |
| **Schema-level prevention** | `pathString()` schema type instead of `z.string()` encodes the hint that this is a path, not chat text — plugs the leak for every path field at once |

### 1.6 relational-invariant (offset/limit)

| Field | Detail |
|-------|--------|
| **Symptom** | Model calls `readFile({ absolutePath, limit: 30 })` — provides `limit` without `offset`, or vice versa |
| **Classification** | Relational invariant violation — each field independently valid, but the relationship `(offset && limit) || (!offset && !limit)` is broken |
| **Root cause** | Model understands each parameter but not their coupling constraint |
| **Deterministic repair** | NOT input repair. Instead, extend tool semantics: `limit` alone → `offset = 0`; `offset` alone → `limit = 2000`. Surface the decision back to the model in result text (no `ERROR:` prefix, model can self-correct next turn) |
| **Mechanisable in Go?** | **Yes.** Pre-execution defaulting, not input repair. `if limit > 0 && offset == 0 { offset = 0 }` etc. Result message appended to output |
| **Evidence** | Verified — primary source lines ~177–208 |

---

## 2. Command Code Design Principles (verified)

Source: same document — structural insights from the repair layer design.

### 2.1 validate-then-repair (not preprocess-then-validate)

| Principle | Detail |
|-----------|--------|
| **Insight** | Preprocessing normalizes inputs before validation — but risks corrupting valid inputs (e.g., `writeFile` content that happens to be JSON-shaped gets rewritten) |
| **Mechanism** | Parse as-is. On success, ship it (valid inputs never touched). On failure, walk the validator's own issue list; apply repairs only at paths the schema disagreed at. Parse again |
| **Outcome** | Repair budget is spent only where the schema says something is wrong. Gives per-tool telemetry for free (`tool_input_repaired:${toolName}` vs `tool_input_invalid:${toolName}`) |
| **Applicable to mill** | Core pattern: the Zod schema (or Go validator) is the prior; repair is fallback, not preprocessing |

### 2.2 shape vs relational invariants

| Principle | Detail |
|-----------|--------|
| **Insight** | Shape problems (wrong type, missing key, wrong container) are fixable by repair. Relational problems (field A depends on field B) need semantic extension, not repair |
| **Applicable to mill** | Classify each schema failure as shape or relational; apply different repair strategies |

### 2.3 repair ordering

| Principle | Detail |
|-----------|--------|
| **Insight** | Repair order matters. `json-array-parse` MUST run before `bare-string-wrap`, or stringified arrays get double-wrapped |
| **Applicable to mill** | Repair chain is ordered: parse stringified arrays → strip nulls → wrap bare strings → detect empty placeholders |

### 2.4 transparency over silent magic

| Principle | Detail |
|-----------|--------|
| **Insight** | When a repair is applied or a default chosen, surface it to the model so it can self-correct next turn. No `ERROR:` prefix (the TUI shouldn't paint it red) |
| **Applicable to mill** | Return repaired inputs with annotation in result; model sees what changed |

---

## 3. rumai Harness Classification Patterns (verified)

Source: `/home/ksante/dev/rumai-labs/rumai/tools/harness/adapters/{commandcode,opencode,pi}.sh` `clasificar` function
and `/home/ksante/dev/rumai-labs/rumai/tools/harness/adapters/common.sh` `_adapter_detect_failure_signal`

### 3.1 Classification Taxonomy

The rumai harness classifies every agent run into one of these categories:

| Label | Meaning | Exit Code Sources | Stderr Signals |
|-------|---------|-------------------|----------------|
| **OK** | Agent completed successfully | `0` (all adapters) | `agent_settled` event (pi) or `"subtype":"success"` (CommandCode) |
| **FATAL** | Unrecoverable error | `1`, `4`, `9`, `130`, `137`, `143`, unknown codes | Generic error without auth/credit/rate signals |
| **MAX_TURNS** | Turn limit reached (work may or may not exist) | `8` (CommandCode) | `"stopReason":"max_turns"` (pi) |
| **AUTH** | Authentication/authorization failure | `3` (CommandCode) | `no api key found`, `not authenticated`, `login required`, `unauthorized`, `401`, `RegionError`, `403` |
| **LIMITE** | Rate limit or usage quota | `5` (CommandCode) | `usage limit`, `rate limit`, `429`, `too many requests`, `weekly usage` |
| **SIN_CREDITO** | Credit/balance exhaustion | `10` (CommandCode) | `insufficient credits`, `no credits`, `credit limit`, `billing`, `quota exceeded` |
| **TRANSITORIO** | Transient infrastructure error | `6` (network), `7` (5xx) | `connection refused`, `ECONNREFUSED`, `network` |

### 3.2 Classification Algorithm

```
clasificar(exit_code, last_result, stderr_content):
  1. Check stderr for auth/credit/rate-limit signals (takes priority over exit code)
     - AUTH signals: no api key, not authenticated, 401, unauthorized, RegionError, 403
     - SIN_CREDITO signals: insufficient credits, no credits, billing, quota exceeded
     - LIMITE signals: usage limit, rate limit, 429, too many requests
  2. Check last_result for backend-specific failure events
     - pi: "stopReason":"max_turns" -> MAX_TURNS
     - pi: "type":"agent_settled" -> OK
  3. Fall back to exit code mapping
```

### 3.3 Per-Adapter Exit Code Mappings

**CommandCode adapter** (structured exit codes):
```
0=OK, 1=FATAL, 3=AUTH, 4=FATAL, 5=LIMITE, 6=TRANSITORIO,
7=TRANSITORIO, 8=MAX_TURNS, 9=FATAL, 10=SIN_CREDITO, 130=FATAL, *=FATAL
```

**opencode / pi adapters** (minimal exit codes):
```
0=OK, 130=FATAL(SIGINT), 137=FATAL(SIGKILL), 143=FATAL(SIGTERM), *=FATAL
```
These rely heavily on stderr inspection because exit codes carry little signal.

### 3.4 Completion Detection (agent_settled)

| Adapter | Detection Mechanism | Reliability |
|---------|-------------------|-------------|
| **pi** | `grep -q '"type":"agent_settled"' "$jsonl_file"` | High — pi emits explicit settled event |
| **CommandCode** | `grep -q '"type":"result".*"subtype":"success"' "$jsonl_file"` | Medium — result frame may exist without work actually done |
| **opencode** | Always returns 1 (no signal) | None — must use watchdog/timeout |

---

## 4. rumai Lessons: Mechanised Failure Checks (verified)

Source: `/home/ksante/dev/rumai-labs/rumai/tools/harness/roles/lessons.md`

### 4.1 Stanza: "Mechanised (checks already exist)"

These are lessons that were learned as prose and then encoded as automated checks:

| Lesson | Mechanisation | Pattern |
|--------|--------------|---------|
| "A green commit is not a green build" | `scripts/guard-ota.mjs` | Gate OTA on CI green, not on commit message |
| "A surface token as text color" | `scripts/check-foreground-tokens.mjs` (CA-13) | Audit that text uses `*Foreground` tokens, not surface tokens |
| "A locale file wrapped in its own name" | `scripts/check-i18n-nesting.mjs` (CA-14) | Verify i18n JSON structure |
| "A watcher can match itself" | `wait.sh` watches PIDs not process names | Use PID-based waiting, not `pgrep -f` |
| "Derive state, do not appoint something to remember it" | `status.sh` reads logs, not verdict files | Compute state from durable artifacts |
| "Work in flight must not depend on editable files" | `task.sh` snapshots scripts before launch | Copy executables to private dir before running |
| "A cleanup must be owned by whatever outlives the work" | `loop.sh` removes its own snapshot | Resource releaser = resource user |

### 4.2 Stanza: "Still prose (not yet mechanised)"

| Lesson | Description | Mechanisable? |
|--------|-------------|---------------|
| "Rewording is not restructuring" | Criteria with verb "say"/"show" can be satisfied by rewording; use "renders zero"/"returns null" | Partially — acceptance criteria linter |
| "Building the gate is not passing through it" | New process must be used on the very next delivery | No — process discipline |
| "The CTO names a defect by its symptom, not its cause" | Measure what they pointed at, not what they named | No — human judgment |
| "Inconsistency of construction is invisible one component at a time" | Audit by family, not by element | Yes — family-based schema checker |
| "A completion event is not a completion" | Check artifact existence, not terminal events | Already mechanised via `agent_settled` + artifact check |

### 4.3 Key Failure Patterns from Lessons

| Pattern | Symptom | Root Cause | Mechanised Fix |
|---------|---------|------------|----------------|
| **Stale generated artifacts** | Token source updated, generated file stale, CI green because check runs against ephemeral DB | Generated artifacts not updated by editing source; CI environment differs from production | `guard-ota.mjs` blocks deploy if CI not green for this commit |
| **Wrong token in correct position** | `color: colors.muted` where `mutedForeground` needed; text invisible (1.06:1) | Token name is valid, type-check passes, but semantic meaning is wrong | `check-foreground-tokens.mjs` — only surface tokens allowed in `backgroundColor`, only foreground tokens allowed in `color` |
| **Watcher self-match** | `pgrep -f "task.sh tokens"` matches its own command line; "everything finished" branch unreachable | Process grepping matches the grepper | Watch PIDs, not process-name patterns |
| **Silent shell failure** | Agent produced nothing; real cause was `set -- $pair` failing silently in shell, files never created | Shell construct fails without error; missing input path causes instant agent death | `_adapter_validate_prompt` checks file exists and is non-empty before launch |
| **Completion event ≠ completion** | pi emits `agent_end` 5 times, commits nothing, wrapper reports success | Terminal events describe model state, not work state | `agent_settled()` check for pi; artifact existence check for all backends |
| **Land without review** | `land.sh` waved through unreviewed branches because absence of verdict ≠ rejection | Gate was strictest where most was known; silent where least was known | Absence of verdict now blocks; `review-pr.sh` always produces a verdict |
| **Editable scripts mid-flight** | `run-cc.sh` edited while tasks running → bash syntax error, lost output | Gap between checking script validity and using it | `task.sh` copies executables to private directory before launch |
| **Detached process orphan** | `nohup ... & disown` → process runs correctly but no completion notification ever fires | Detaching removes process from harness tree | Tracked parallelism: `cmd_a & cmd_b & wait` in one foreground call |
| **Brief confirms its own finding** | Agent ran one turn, 824K tokens, confirmed gap exists, no file produced, reported success | Describing a problem invites agreement; agreement looks like progress | Open with imperative deliverable; check `git log`, not exit code |

---

## 5. Combined Catalog: Pattern Name → Repair Strategy

### 5.1 Input Shape Repairs (mechanisable in Go)

| # | Pattern Name | Symptom | Classification | Deterministic Repair | Mechanisable |
|---|-------------|---------|---------------|---------------------|-------------|
| 1 | **null-for-optional** | `{ field: null }` at optional field | Shape — wrong type (`null` vs `undefined`) | Delete key from input at issue path | **Yes** |
| 2 | **json-array-parse** | `"[\"a\",\"b\"]"` where `["a","b"]` expected | Shape — string instead of array | `JSON.parse` the string; if array, replace | **Yes** |
| 3 | **bare-string-wrap** | `"foo"` where `["foo"]` expected | Shape — string instead of array (not stringified) | Wrap in `[value]` after json-array-parse fails | **Yes** |
| 4 | **empty-placeholder** | `{}` where array expected | Shape — object instead of array | Detect; flag for retry with schema hint | **Detection only** |
| 5 | **markdown-auto-link** | `[notes.md](http://notes.md)` in path field | Distribution leak — chat autolink bleeds to tool boundary | Regex unwrap degenerate case: `[text](proto://text)` → `text` | **Yes** |
| 6 | **string-instead-of-number** | `"42"` where `42` expected | Shape — string instead of number | `strconv.Atoi` / `strconv.ParseFloat` at issue path | **Yes** [INFERRED — not in CC doc but follows pattern] |
| 7 | **boolean-as-string** | `"true"` where `true` expected | Shape — string instead of boolean | `strconv.ParseBool` at issue path | **Yes** [INFERRED] |

### 5.2 Relational Repairs (mechanisable in Go, but different strategy)

| # | Pattern Name | Symptom | Classification | Deterministic Repair | Mechanisable |
|---|-------------|---------|---------------|---------------------|-------------|
| 8 | **missing-co-required** | `{ limit: 30 }` without `offset` | Relational invariant — each field valid, relationship violated | Default missing partner: `offset=0` for bare `limit`, `limit=2000` for bare `offset`. Surface in result | **Yes** (pre-exec defaulting) |
| 9 | **mutual-exclusion** | Both `path` and `url` provided where exactly one required | Relational invariant | Pick one by priority rule; surface decision | **Yes** [INFERRED] |

### 5.3 Harness Classification Repairs (mechanisable in Go)

| # | Pattern Name | Symptom | Classification | Deterministic Repair | Mechanisable |
|---|-------------|---------|---------------|---------------------|-------------|
| 10 | **auth-failure** | Exit 1/3, stderr: "not authenticated", "401", "No API key" | AUTH — unrecoverable, abort | Set up new API key; restart run | **Partial** — detect + abort is mechanisable; key provisioning is not |
| 11 | **rate-limit** | Exit 5, stderr: "rate limit", "429" | LIMITE — recoverable by waiting | Exponential backoff, retry | **Yes** |
| 12 | **credit-exhausted** | Exit 10, stderr: "insufficient credits" | SIN_CREDITO — unrecoverable, abort | Alert human; switch to different model/provider | **Partial** — detect + abort; switch may be mechanisable if multiple providers configured |
| 13 | **transient-infra** | Exit 6/7, stderr: "connection refused", "ECONNREFUSED" | TRANSITORIO — recoverable by waiting | Retry with backoff | **Yes** |
| 14 | **max-turns** | Exit 8 or '"stopReason":"max_turns"' | MAX_TURNS — work may or may not exist | Check if artifact exists; if yes, treat as OK; if no, retry with continuation prompt | **Yes** |
| 15 | **exit-0-no-work** | Exit 0 but no commit/file produced | False positive — model closed turn without doing work | Check artifact existence post-run; classify as FATAL if nothing changed | **Yes** — `git diff --stat` check |

### 5.4 Harness-Infrastructure Repairs (mechanisable)

| # | Pattern Name | Symptom | Classification | Deterministic Repair | Mechanisable |
|---|-------------|---------|---------------|---------------------|-------------|
| 16 | **silent-shell-failure** | Agent run exits 0 instantly; prompt file missing/empty | Pre-flight — input missing | `_adapter_validate_prompt` checks file exists and non-empty before launch | **Yes** |
| 17 | **editable-scripts-race** | Script edited mid-execution → bash syntax error | Race condition — mutable executable | Copy executables to private dir before launch; execute copies | **Yes** |
| 18 | **detached-process-orphan** | `nohup ... & disown` — process runs but completion never reported | Process tree detachment | Use `cmd_a & cmd_b & wait` for tracked parallelism; never `disown` | **Yes** |
| 19 | **watcher-self-match** | `pgrep -f pattern` matches watcher's own command line | Process inspection false positive | Watch PIDs not process names; or wait for positive evidence (verdict file) | **Yes** |
| 20 | **land-without-review** | Gate accepts unreviewed work because absence of verdict ≠ rejection | Gate polarity error | Absence of verdict = block (strictest state). Always produce verdict before landing | **Yes** |

---

## 6. Repair Ordering Contract

For mill's deterministic repair layer, the ordering from Command Code's implementation matters:

```
1. json-array-parse      — MUST run first (catches stringified arrays before wrapping)
2. markdown-auto-link    — path fields only; run early before type coercion
3. null-for-optional     — strip nulls at optional field paths
4. bare-string-wrap      — MUST run AFTER json-array-parse (avoids double-wrapping)
5. empty-placeholder     — detect-only; flag for retry
6. relational-defaults   — pre-execution defaulting (separate from input repair)
```

The classic mistake: running `bare-string-wrap` before `json-array-parse`.
Input `"[\"a\",\"b\"]"` → bare-string-wrap produces `["[\"a\",\"b\"]"]` → schema-valid but semantically wrong (array contains one JSON string, not two elements).

---

## 7. rumai Classification State Machine

For mill's harness, the classification flow determines retry/abort decisions:

```
                    ┌──────────┐
          ┌─────────│   OK     │──────────┐
          │         └──────────┘          │
          │         artifact check        │
          │         exists? ───no───► FATAL
          │                              │
   ┌──────┴──────┐                       │
   │  MAX_TURNS  │──artifact exists?─────┤
   └──────┬──────┘  yes→OK  no→retry     │
          │                              │
   ┌──────┴──────┐                       │
   │   LIMITE    │──backoff→retry────────┤
   └──────┬──────┘                       │
          │                              │
   ┌──────┴──────┐                       │
   │ TRANSITORIO │──backoff→retry────────┤
   └──────┬──────┘                       │
          │                              │
   ┌──────┴──────┐                       │
   │    AUTH     │──abort (no retry)─────┤
   └──────┬──────┘                       │
          │                              │
   ┌──────┴──────┐                       │
   │ SIN_CREDITO │──abort (no retry)─────┤
   └──────┬──────┘                       │
          │                              │
   ┌──────┴──────┐                       │
   │   FATAL     │──abort (no retry)─────┘
   └─────────────┘
```

---

## 8. Evidence Classification

| Pattern ID | Source | Verification Status |
|-----------|--------|-------------------|
| 1–6 | Command Code docs (primary source) | **Verified** — direct citation from author |
| 7 | Pattern extrapolation | **Inferred** — follows json-array-parse shape for other primitive types |
| 8–9 | Command Code docs §4 | **Verified** — direct citation |
| 10–15 | rumai `clasificar()` + `_adapter_detect_failure_signal()` | **Verified** — primary source code |
| 16–20 | rumai `lessons.md` | **Verified** — documented real failures with mechanised fixes |
| 2.1–2.4 | Command Code docs design principles | **Verified** — direct citation |
| 3.1–3.4 | rumai adapters + harness (4 source files) | **Verified** — primary source code |
| 4.1–4.3 | rumai `lessons.md` | **Verified** — documented real failures |
| 6 (ordering) | Command Code docs §1 ordering note | **Verified** — direct citation |
| 7 (state machine) | rumai `harness` lines 228–253 | **Verified** — primary source code |
