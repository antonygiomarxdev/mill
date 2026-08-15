# Review: #62 — `mill init` overwrite warning

## Verdict

**APPROVED**

## Gate Results

- `go build ./...` — PASS
- `go test ./internal/cli/ -run "TestInit" -count=1` — PASS (all init tests: TestInitCreatesMillYAML, TestInitCreatesDirectories, TestInitCopiesRoleFiles, TestInitCopiesCheckFiles, TestInitWithCustomFlags, TestInitInteractiveUsesDefaults, TestInitPrintOutput, TestGenerateMillYAMLWriteError, TestPromptDefaultPath, TestPromptEOF, TestCopyScaffoldWriteError, TestProjectRootGoModPresent, TestProjectRootNoMarker, TestRunInitParseError, TestRunInitMissingTargetDir, TestRunInitCustomNameFlag, TestInitOverwriteInteractiveReject, TestInitOverwriteInteractiveAccept, TestInitOverwriteInteractiveAcceptYes, TestInitOverwriteInteractiveCaseInsensitive, TestInitOverwriteForceBypass, TestInitOverwriteForceWithoutYes, TestInitOverwriteNoMillDir, TestInitOverwriteMillDirIsFile, TestInitOverwriteForceYesSkipsAllPrompts)

## Acceptance Criteria Verification

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | Warning + prompt when `.mill/` exists | ✅ | `promptOverwrite` (init.go:155-187) shows "⚠ .mill/ already exists." followed by listing of what will be overwritten, then "Overwrite? [y/N]: ". |
| 2 | `n` (or anything other than `y`/`yes`) aborts | ✅ | `line != "y" && line != "yes"` returns `fmt.Errorf("init aborted")` (init.go:182-184). Case-insensitive comparison via `strings.ToLower`. |
| 3 | `y` proceeds with normal init | ✅ | When `line == "y" || line == "yes"`, returns nil (proceed). Verified by TestInitOverwriteInteractiveAccept. |
| 4 | `--force` skips prompt, overwrites unconditionally | ✅ | `if !force { promptOverwrite(...) }` — entire check bypassed when `--force` is set (init.go:89-92). TestInitOverwriteForceBypass verifies. |
| 5 | `--force --yes` works for scripting | ✅ | Orthogonal flags: `--force` skips overwrite prompt, `--yes` skips config prompts. TestInitOverwriteForceYesSkipsAllPrompts verifies both. |
| 6 | No `.mill/` → no prompt, works as before | ✅ | `os.IsNotExist(err)` returns nil (proceed) immediately (init.go:158-160). TestInitOverwriteNoMillDir verifies. |
| 7 | `.mill/` is file → specific error | ✅ | `!entry.IsDir()` returns `.mill/ exists but is a file, not a directory — remove it manually and retry` (init.go:164). TestInitOverwriteMillDirIsFile verifies. |
| 8 | `go test ./internal/cli/` passes | ✅ | All 25 init tests pass (9 new overwrite-specific tests). |

## Architecture Compliance

- **Pre-init check runs before any filesystem writes:** `promptOverwrite` is called after flag parsing but before `os.MkdirAll(target)` (init.go:89 vs. init.go:101). ✅
- **`-yes` does NOT bypass the overwrite check:** Only `--force` does. ✅
- **Scaffold copy, mill.yml generation, directory creation unchanged** ✅

## Quality Checks

- No `any`, `unknown`, `Record<string, T>`, or `object` types introduced ✅
- `promptOverwrite` walks `.mill/` subdirectories safely (ignores `os.ReadDir` errors, uses `continue`) ✅
- `formatSize` handles B/KB/MB thresholds correctly ✅
- Tests cover uppercase `Y`, `YES`, and `Yes` variants (case-insensitive) ✅
- `TestInitOverwriteForceWithoutYes` verifies that `--force` alone still asks config prompts (no stdin → error from config prompts, not from overwrite check) ✅

## Files Reviewed

- `internal/cli/init.go` (MODIFY) — `runInit` (+ `--force` flag, `promptOverwrite` call), `promptOverwrite` (NEW function), `formatSize` (NEW function)
- `internal/cli/init_test.go` (MODIFY) — 9 new overwrite tests

## Notes

- The `--force` flag help text includes "(DESTRUCTIVE)" as specified in the risk mitigation.
- When `.mill/` directory walk encounters a missing `state.json` (new project without delegated tasks), `os.Stat` error is silently skipped via `continue` — correct behavior.
- `promptOverwrite` reuses the same `bufio.Reader` created for config prompts, avoiding redundant reader creation.
