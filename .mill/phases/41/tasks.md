# Tasks: Raise CLI test coverage to ≥90%

Baseline: `go test -count=1 ./internal/cli/ -cover` → **81.5%**. Target: ≥90.0%.

All tasks are test-only — no source files modified. Every test uses `t.TempDir()` for isolation.
Table-driven where the function is a classifier; targeted unit tests otherwise.

## Wave 1 (parallel — independent files)

- [ ] **Extend `app_test.go` — `runLand` success-path, help, parse-error + `buildPrompt`** — role: sr-dev-be, deps: none, est: 25m
  1. Add `TestRunLandSuccessWithGates`: creates temp git repo via `setupTestGitRepo`, runs `app.Run("land", "-worktree", dir, "main", "echo ok")`, asserts no error and `git branch --show-current` reports "main"
  2. Add `TestRunLandHelpFlag`: runs `app.Run("land", "-h")`, asserts nil error (`flag.ErrHelp` → nil per `runLand`)
  3. Add `TestRunLandParseError`: runs `app.Run("land", "--nonexistent")`, asserts non-nil error returned
  4. Add `TestBuildPrompt`: calls `buildPrompt(42)`, asserts output string contains "42" and is non-empty

- [ ] **Extend `delegate_test.go` — `classifyResult` stderr, `resolveModel`, `buildRolePrompt`, `readActiveRole`** — role: sr-dev-be, deps: none, est: 40m
  1. Extend `TestClassifyResultExitCodes` table with 3 stderr-signal rows: `{exit: 1, stderr: "401 Unauthorized", want: domain.ClassificationAuth}`, `{exit: 1, stderr: "insufficient credits", want: domain.ClassificationNoCredit}`, `{exit: 1, stderr: "network timeout", want: domain.ClassificationTransient}`. Assert `classifyResult(tc.exit, tc.stderr) == tc.want`
  2. Add `TestResolveModelMissingRoleFile`: creates `t.TempDir()` with no `roles/sr-dev-be/ROLE.md`, calls `resolveModel("sr-dev-be", cfg)`, asserts fallback to `cfg.Model`
  3. Add `TestResolveModelEmptyModelTier`: writes `roles/sr-dev-be/ROLE.md` with `model:` empty or missing, asserts fallback
  4. Add `TestResolveModelCustomTier`: writes `roles/sr-dev-be/ROLE.md` with `model: gpt-5`, asserts `resolveModel` returns `"gpt-5"` (custom tier passed through)
  5. Add `TestBuildRolePromptWithSkills`: writes `roles/sr-dev-be/ROLE.md` with `skills:\n  - tdd\n  - code-review`, calls `buildRolePrompt(1, "sr-dev-be")`, asserts output contains "tdd" and "code-review"
  6. Add `TestBuildRolePromptNoSkills`: writes `roles/sr-dev-be/ROLE.md` with `skills:` empty or absent, calls `buildRolePrompt(1, "sr-dev-be")`, asserts output contains issue number "1" and role name
  7. Add `TestReadActiveRoleError`: creates `MillDir` with unreadable `role` file (create dir at `role` path), calls `readActiveRole()`, asserts fallback to `"staff"`

- [ ] **Extend `init_test.go` — `generateMillYAML`, `prompt`, `copyScaffold`, `projectRoot`, `runInit`** — role: sr-dev-be, deps: none, est: 40m
  1. Add `TestGenerateMillYAMLWriteError`: creates `t.TempDir()`, pre-creates `target/mill.yml` as a directory (so `os.WriteFile` fails), calls `generateMillYAML(target, cfg)`, asserts error returned
  2. Add `TestPromptDefaultPath`: supplies `strings.NewReader("\n")`, calls `prompt(r, buf, "Name", "default")`, asserts returns `"default"`
  3. Add `TestPromptEOF`: supplies `strings.NewReader("")` (no newline — immediate EOF), calls `prompt`, asserts returns default string
  4. Add `TestCopyScaffoldWriteError`: creates `t.TempDir()`, pre-creates a read-only subdirectory at a path the scaffold would write, calls `copyScaffold(target)`, asserts error returned
  5. Add `TestProjectRootGoModPresent`: creates `t.TempDir()` with `go.mod` file, calls `projectRoot()`, asserts returns the dir path and nil error (may need `os.Chdir` + restore with `t.Cleanup`)
  6. Add `TestProjectRootNoMarker`: creates `t.TempDir()` with neither `go.mod` nor `mill.yml`, calls `projectRoot()` from within it, asserts error returned
  7. Add `TestRunInitParseError`: runs `app.Run("init", "--nonexistent")`, asserts non-nil error returned from `flagSet.Parse`
  8. Add `TestRunInitMissingTargetDir`: runs with `-target /nonexistent/path/child` where parent does not exist, asserts `os.MkdirAll` creates it and init succeeds
  9. Add `TestRunInitCustomNameFlag`: runs `app.Run("init", "-yes", "-name", "testproj", "-target", dir)`, asserts generated `mill.yml` contains `project: testproj`

- [ ] **Extend `role_test.go` — `roleGet` empty file, `roleSet` write-error, already-set** — role: sr-dev-be, deps: none, est: 20m
  1. Add `TestRoleGetEmptyFile`: creates `role` file with empty content (`os.WriteFile` with `[]byte("")`), calls `app.Run("role", "get")`, asserts output is `"none\n"`
  2. Add `TestRoleSetWriteError`: creates `MillDir` with read-only permissions (`0o444` on dir so `os.WriteFile` fails), calls `roleSet("staff")`, asserts error returned
  3. Add `TestRoleSetAlreadySet`: creates `role` file containing `"staff"`, calls `roleSet("staff")`, asserts no error returned and file unchanged (short-circuit path)
