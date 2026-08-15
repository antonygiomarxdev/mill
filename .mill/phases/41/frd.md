# FRD: Raise CLI test coverage to ≥90%

## User need

Developers cannot land code. The pre-push gauntlet enforces ≥90% test coverage across all packages, and `internal/cli/` is the only remaining package below that threshold at 81.3%. Every land attempt is blocked by the coverage gate until this gap is closed.

This is a developer-productivity defect, not a feature request. The gap affects every contributor regardless of what they are working on.

## Functional requirements

1. **Coverage threshold.** `go test -count=1 ./internal/cli/ -cover` must report ≥90.0% statement coverage on every run. The result must be deterministic — no flaky coverage numbers driven by randomized test inputs.

2. **Functions under test.** Every function in `internal/cli/` that is currently under-tested must gain coverage. The lowest-coverage functions are, in descending priority:
   - `runLand` (currently 64.7%) — success-path test exercising a `git land` command in a temp repo. Must cover at least one additional branch.
   - `generateMillYAML` (currently 71.4%) — test template rendering output matches expected YAML content.
   - `prompt` (currently 75%) — test user-input path produces expected output.
   - `copyScaffold` (currently 76.2%) — test file-copy path with real or temp filesystem.
   - `runDelegate` (currently 77.3%) — test dry-run path.
   - `runInit` (currently 81%) — test init flow.

3. **Gate integration.** After the coverage threshold is met, running `bash checks/pre-push` must pass with zero failures. The coverage step in that script must report no violations for `internal/cli/`.

4. **No dependency regression.** The fix must not introduce new third-party dependencies. All tests must use only the Go standard library and packages already in the module graph.

## Out of scope

- Achieving 100% coverage in `internal/cli/`. The requirement is ≥90%, not perfect coverage.
- Coverage improvements in any other package. All other packages already meet the threshold.
- Refactoring `internal/cli/` to make it more testable. Add tests only; do not redesign the API, extract interfaces, or add dependency-injection scaffolding beyond what already exists.
- Mutation testing. That is a post-land gate (land), not the pre-push gauntlet this issue addresses.

## Priority

**P0** — blocks all lands. Every developer who pushes a branch hits this gate failure. No other work can ship until this is resolved.
