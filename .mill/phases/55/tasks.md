# Tasks: Read issue body and acceptance criteria for delegation prompts

All tasks modify/update only files listed in the SPEC Components affected table.

## Wave 1 (sequential — reader before integration)

### Task 1: Create `internal/issue/reader.go` — ReadBody + ExtractAcceptanceCriteria

- role: sr-dev-be
- deps: none
- est: 20m
- file: `internal/issue/reader.go` (NEW)

#### Acceptance Criteria

1. Implement `ReadBody(issueNum int) (body string, labels []string, err error)`:
   - Shells out to `gh issue view <N> --json body,labels`
   - Parses JSON into struct `{ Body string, Labels []struct{ Name string } }`
   - Returns `body` as raw markdown, `labels` as string slice of label names
   - If `gh` not found on PATH (exec.ErrNotFound): return error `"gh CLI not found. Install from github.com/cli/cli"`
   - If `gh issue view` exits nonzero: parse stderr for "not found" → `"issue #N not found"`; otherwise wrap exit error
   - Network/timeout errors: wrap with `fmt.Errorf("failed to read issue #%d: %w", issueNum, err)`

2. Implement `ExtractAcceptanceCriteria(body string) []string`:
   - Scan for checkbox lists: lines matching `- [ ]` or `- [x]` → collect the text after `] `
   - Scan for numbered criteria: lines matching `^\d+\.\s+\*\*(.+)\*\*` → collect the bold label
   - Scan for "Acceptance criteria" or "Acceptance Criteria" section headers (case-insensitive `## ` or `### ` heading → lines starting `#`) → collect subsequent bullet/numbered items until next heading
   - Returns deduplicated, ordered list; nil if no criteria matched
   - No regex on the entire body — iterate lines once, track section state

3. Follow existing conventions in `internal/issue/issue.go`: same package, same import style, same error patterns

### Task 2: Create `internal/issue/reader_test.go` — Tests for body parsing and criteria extraction

- role: sr-dev-be
- deps: Task 1
- est: 25m
- file: `internal/issue/reader_test.go` (NEW)

#### Acceptance Criteria

1. Test `ReadBody` (unit tests with mocked `gh` — use `os.Setenv("PATH", ...)` or test helper that intercepts exec):
   - `TestReadBodySuccess`: mock `gh issue view 55 --json body,labels` → valid JSON with body and labels → assert body matches, labels match
   - `TestReadBodyGHNotFound`: gh not on PATH → error contains "gh CLI not found"
   - `TestReadBodyIssueNotFound`: gh exits 1 with "not found" → error contains "issue #N not found"
   - `TestReadBodyNetworkError`: gh exits 1 with network error → error wraps underlying cause

2. Test `ExtractAcceptanceCriteria` (pure function — no mocking needed):
   - `TestExtractCheckboxCriteria`: body with `- [ ] do thing\n- [x] did thing\n- [ ] other` → returns `["do thing", "did thing", "other"]`
   - `TestExtractNumberedBoldCriteria`: body with `1. **Do X**\n2. **Do Y**` → returns `["Do X", "Do Y"]`
   - `TestExtractSectionCriteria`: body with `## Acceptance Criteria\n- item one\n- item two\n## Next Section` → returns `["item one", "item two"]`
   - `TestExtractNoCriteria`: body with no recognizable criteria → returns nil
   - `TestExtractDeduplication`: body with same criterion in checkbox and section → returns it once

3. Follow existing test conventions from `internal/issue/issue_test.go`: table-driven tests, same assertion style

### Task 3: Refactor `buildRolePrompt` to accept body + criteria

- role: sr-dev-be
- deps: Task 1
- est: 15m
- file: `internal/cli/delegate.go` (MODIFY)

#### Acceptance Criteria

1. Change signature: `buildRolePrompt(issueNum int, body string, ac []string, targetRole string) string`

2. Prompt format (exactly as SPEC — Section "Prompt construction"):
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

3. Title extraction: first `# ` heading line in body. Strip leading `# ` and trailing whitespace. If no `# ` heading found, title = `""` (omit `: <title>` suffix from header).

4. Spec reference: check `os.Stat(".mill/phases/<N>/spec.md")` or `filepath.Join(root, ".mill", "phases", ...)`. Only include the "## Spec Reference" section if file exists. Use the resolved project root path.

5. Role instructions: same as current — load `role.LoadFrom(root, targetRole)`. On failure, fall back to generic prompt with the full body and criteria included (not the old bare prompt).

6. Criteria section: if `ac` is nil or empty, omit the "## Acceptance Criteria" section entirely. If non-empty, format as:
   ```
   ## Acceptance Criteria
   1. <first>
   2. <second>
   ...
   ```

7. All parts optional: only emit sections that have content. Minimum: `# Issue #N` + body + role.

### Task 4: Integrate `ReadBody` into `runDelegate` with fallback

- role: sr-dev-be
- deps: Task 3
- est: 15m
- file: `internal/cli/delegate.go` (MODIFY)

#### Acceptance Criteria

1. After `issueNum` is parsed (line 54) and before `buildRolePrompt` is called (line 125), call `issue.ReadBody(issueNum)`.

2. On success: pass `body`, `labels`, and `issue.ExtractAcceptanceCriteria(body)` to `buildRolePrompt`.

3. On failure (`gh` not found, network error, issue not found): construct degraded prompt:
   ```
   # Issue #N

   ⚠ Issue body could not be read: <error message>
   Proceeding with limited context. Read the issue at: https://github.com/<owner>/<repo>/issues/N
   ```
   - Owner/repo should be derived from the git remote: `gh repo view --json owner,name` or parse `git remote get-url origin`. If unavailable, use `OWNER/REPO` as placeholder.
   - Pass `""` (empty body) and `nil` (no criteria) to `buildRolePrompt` — the degraded prompt path is handled inside `buildRolePrompt`.

4. Do NOT block delegation on read failure — the fallback always kicks in.

5. The degraded prompt still includes role instructions (via `buildRolePrompt` fallback path).

### Task 5: Update `internal/cli/delegate_test.go` — Body integration and prompt format

- role: sr-dev-be
- deps: Task 4
- est: 30m
- file: `internal/cli/delegate_test.go` (MODIFY)

#### Acceptance Criteria

1. Test `buildRolePrompt` with body + criteria:
   - `TestBuildRolePromptWithBodyAndCriteria`: creates temp `.mill/roles/sr-dev-be/ROLE.md` with frontmatter, `.mill/phases/55/spec.md` (empty file for existence check), calls `buildRolePrompt(55, "# Title\n\nBody text", []string{"Do X", "Do Y"}, "sr-dev-be")`. Asserts output contains `"# Issue #55: Title"`, `"Body text"`, `"## Acceptance Criteria"`, `"1. Do X"`, `"2. Do Y"`, `"## Spec Reference"`, `.mill/phases/55/spec.md`, `"## Role"`, role instructions.
   - `TestBuildRolePromptNoCriteria`: `ac` is nil → output does NOT contain `"## Acceptance Criteria"`.
   - `TestBuildRolePromptNoSpecFile`: `.mill/phases/55/spec.md` does not exist → output does NOT contain `"## Spec Reference"`.
   - `TestBuildRolePromptFallbackNoRoleFile`: role file missing → generic prompt still includes `"# Issue #55"`, body, and criteria.
   - `TestBuildRolePromptTitleExtraction`: body starts with `# My Title\n\ncontent` → header contains `: My Title`. Body starts with plain text (no `# ` heading) → header is just `# Issue #55`.

2. Test `runDelegate` integration:
   - `TestRunDelegateWithReadBodyError`: set `PATH` to empty dir so `gh` not found → delegation still proceeds (does not error out). Prompt contains `"⚠ Issue body could not be read"`.
   - Update `TestDelegateValidIssueDispatchesAndRecords`: mock `gh` or inject `ReadBody` through a testable interface. Dispatch opts prompt contains issue content.
   - If `runDelegate` cannot be easily tested with real `gh` calls, add a test-only hook (e.g., `var readBodyFunc = issue.ReadBody`) that tests can override.

3. All existing delegate tests continue to pass.

4. Follow existing test conventions: table-driven, `*testing.T`, same assertion style, `fakeAdapter`/`fakeSession` where applicable.

---

## Acceptance criteria (cross-cutting)

1. `mill delegate <issue>` reads issue body via `gh issue view` and includes it in the prompt
2. Acceptance criteria are extracted and listed in a structured section
3. SPEC/FRD reference is included when `.mill/phases/<N>/spec.md` exists
4. Fallback works when `gh` is unavailable (warning + proceed)
5. `buildRolePrompt` output follows the defined format (issue, criteria, spec, role)
6. `go test ./internal/issue/` passes (new reader tests)
7. `go test ./internal/cli/` passes (updated delegate tests)
