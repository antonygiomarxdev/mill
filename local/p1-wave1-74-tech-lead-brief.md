# Tech Lead Brief — Issue #74: ROLE.md Frontmatter YAML Syntax Errors

> **Role:** tech-lead | **Model:** pro | **Reviewed by:** architect

## Context

The hand-rolled YAML frontmatter parser in `internal/role/role.go` recognizes only 6 keys: `role`, `model`, `agent`, `reviewed_by`, `delegates_to` (block-style list), `skills` (block-style list). It does NOT recognize `allowed_files` — those values are silently dropped. Additionally, `staff/ROLE.md` is missing the `agent:` field entirely.

All 11 ROLE.md files use YAML flow-style `allowed_files: [...]` which the parser treats as an opaque scalar string. For the parser to read `allowed_files` items, the files must use block-style `- item` syntax, and the parser must be taught to recognize the key.

## Affected Files

### Go code (1 file, 2 changes)
1. **`internal/role/role.go`**
   - Add `AllowedFiles []string` to `Frontmatter` struct
   - Add `case "allowed_files":` in parser switch

### ROLE.md files (11 files, 12 changes — staff gets 2)
2. `.mill/roles/staff/ROLE.md` — add `agent: task` AND convert `allowed_files` to block
3. `.mill/roles/architect/ROLE.md` — convert `allowed_files` to block
4. `.mill/roles/tech-lead/ROLE.md` — convert `allowed_files` to block
5. `.mill/roles/sr-dev-fe/ROLE.md` — convert `allowed_files` to block
6. `.mill/roles/sr-dev-be/ROLE.md` — convert `allowed_files` to block
7. `.mill/roles/sr-dev-data/ROLE.md` — convert `allowed_files` to block
8. `.mill/roles/pm/ROLE.md` — convert `allowed_files` to block
9. `.mill/roles/ux-designer/ROLE.md` — convert `allowed_files` to block
10. `.mill/roles/ui-designer/ROLE.md` — convert `allowed_files` to block
11. `.mill/roles/reviewer/ROLE.md` — convert `allowed_files` to block
12. `.mill/roles/qa-docs/ROLE.md` — convert `allowed_files` to block

### Test file (1 new test)
13. **`internal/role/role_test.go`** — add `TestParseAllRoleFiles`

## Acceptance Criteria → Implementation Steps

### AC #2: staff/ROLE.md has `agent:` field
- [ ] Insert `agent: task` after `model: pro` in `.mill/roles/staff/ROLE.md`
- [ ] Verify: `grep "^agent:" .mill/roles/staff/ROLE.md` returns `agent: task`

### AC #1: All 11 files parse correctly; `skills:` items under `skills:`, `allowed_files:` is separate field
- [ ] Add `AllowedFiles []string` to `Frontmatter` struct in `internal/role/role.go`
- [ ] Add `case "allowed_files":` to parser switch (identical pattern as `skills:`/`delegates_to:`)
- [ ] Convert every `allowed_files: [...]` line to block-style across all 11 files
- [ ] Verify: write a small Go program or test that calls `ParseFrontmatter` on each file and confirms `Skills` and `AllowedFiles` are populated correctly

### AC #3: `role.ParseFrontmatter()` test validates all 11 files
- [ ] Add `TestParseAllRoleFiles` to `internal/role/role_test.go`
  - Uses `filepath.Glob(".mill/roles/*/ROLE.md")` to discover files
  - Calls `ParseFrontmatter(path)` on each
  - Asserts: no error, `Skills` non-empty (all roles have skills), `AllowedFiles` non-empty (all roles have allowed_files), `Agent` non-empty (all roles have agent)

## Line-by-Line Changes

### `internal/role/role.go` — Struct
```
AFTER line 17 (after `Skills []string`):
+	AllowedFiles []string
```

### `internal/role/role.go` — Parser switch
```
AFTER case "skills": block (after line 127):
+		case "allowed_files":
+			fm.AllowedFiles = []string{}
+			currentList = &fm.AllowedFiles
```

### `.mill/roles/staff/ROLE.md` — Two changes
```
AFTER `model: pro`:
+agent: task

BEFORE: allowed_files: []
AFTER:
+allowed_files: []
```
(Note: for staff, `allowed_files: []` stays as-is since it's already an empty list. Still add the parser case.)

### All other 10 ROLE.md files — Convert flow to block
```
BEFORE: allowed_files: [.md, .go]
AFTER:  allowed_files:
          - .md
          - .go
```

Per-file mappings:
| File | From | To |
|------|------|----|
| architect | `[.md, .yml, .yaml]` | `- .md` / `- .yml` / `- .yaml` |
| tech-lead | `[.md, .go]` | `- .md` / `- .go` |
| sr-dev-fe | `[.go, .md, .yml, .yaml, .json]` | `- .go` / `- .md` / `- .yml` / `- .yaml` / `- .json` |
| sr-dev-be | `[.go, .md, .yml, .yaml, .json]` | same |
| sr-dev-data | `[.go, .md, .yml, .yaml, .json]` | same |
| pm | `[.md]` | `- .md` |
| ux-designer | `[.md, .pen]` | `- .md` / `- .pen` |
| ui-designer | `[.md, .pen]` | `- .md` / `- .pen` |
| reviewer | `[.md]` | `- .md` |
| qa-docs | `[.md, .yml]` | `- .md` / `- .yml` |

## Do NOT
- Change role semantics — only fix YAML structure
- Remove or rename existing fields
- Run project-wide builds or test suites (that's the Sr Dev's verification step)
- Touch `effort_scaling` in staff/ROLE.md — out of scope

## Deliverable
- Commits: 1
- Files: `internal/role/role.go`, `.mill/roles/*/ROLE.md` (11 files), `internal/role/role_test.go`

## Steps (for Sr Dev)
- [ ] 1. Add `AllowedFiles` field to `Frontmatter` struct
- [ ] 2. Add `allowed_files` case to parser
- [ ] 3. Convert all 11 ROLE.md files' `allowed_files` to block-style YAML
- [ ] 4. Add `agent: task` to staff/ROLE.md
- [ ] 5. Write `TestParseAllRoleFiles` test
- [ ] 6. Run `go test ./internal/role/ -run TestParseAllRoleFiles -v`
- [ ] 7. Commit: `fix(roles): add allowed_files parser support and missing agent field`
