# Spec: Raise CLI test coverage to ≥90%

## Architecture

**No architectural change.** This is a test-only effort within `internal/cli/`. The package structure, public API, and dependency graph remain unchanged. All existing tests continue to pass.

The coverage gate in `checks/pre-push` uses the minimum across all packages — `internal/cli/` is the sole package below 90%. Closing the gap unblocks the gate for every contributor.

### Coverage target decomposition

Current coverage: **81.5%** (34 covered functions, 0 uncovered except `buildPrompt` at 0%). Need ≥8.5 percentage points. The functions with the most uncovered statements are prioritized:

| Function | Current | Target | Uncovered branches |
|---|---|---|---|
| `buildPrompt` | 0.0% | 100% | Dead code — deprecated path, never called from `runDelegate` which uses `buildRolePrompt` |
| `(a *App) runLand` | 64.7% | ≥85% | No-args error, parse error, help flag, gate execution path |
| `resolveModel` | 69.2% | ≥85% | Missing role file, empty model tier, custom tier |
| `buildRolePrompt` | 71.4% | ≥85% | Role with no skills, role with skills |
| `runDispatchLoop` | 72.5% | ≥85% | Retry exhaustion, dispatch failure after retries |
| `classifyResult` | 76.2% | ≥85% | Uncovered stderr signals (AUTH via 401, NO_CREDIT, TRANSIENT) |
| `generateMillYAML` | 71.4% | ≥85% | Template parse error, write error |
| `prompt` | 75.0% | ≥85% | Empty input (default path), EOF |
| `projectRoot` | 75.0% | ≥85% | go.mod present, neither go.mod nor mill.yml present |
| `copyScaffold` | 76.2% | ≥85% | File copy error, directory traversal edge case |
| `installHooks` | 77.8% | ≥85% | Source directory read error, file write error |
| `validateDelegation` | 78.6% | ≥85% | Role file parse error, role not in delegates_to |
| `runInit` | 81.0% | ≥85% | parse error, custom flags, missing target dir error |
| `runDelegate` | 81.4% | ≥85% | Scaffold failure, adapter dispatch error, blocked result |
| `runRole` | 81.8% | ≥90% | Unknown subcommand, delegation-only error |
| `roleGet` | 83.3% | ≥90% | Empty role file |
| `roleSet` | 84.6% | ≥90% | Write error, already-set short-circuit |
| `readActiveRole` | 87.5% | ≥90% | File read error |

### Strategy

1. **Each test targets one uncovered branch** in a specific function. No integration tests that cross multiple functions.
2. **Table-driven where the function is a classifier** (`classifyResult`, `detectRole` — both already table-driven in tests; extend existing tables).
3. **Temp directories for filesystem** (`t.TempDir()`). No shared state between tests.
4. **No new subprocesses** except where the function under test already spawns them (`runLand` gates, `projectRoot` with real `go.mod`).
5. **`buildPrompt` gets a one-line test** — the function is deprecated but must be covered per gate rules.

## Components affected

All changes are in test files within `internal/cli/`:

| File | Change |
|---|---|
| `internal/cli/app_test.go` | Add `runLand` success-path test with temp git repo; add `buildPrompt` test |
| `internal/cli/delegate_test.go` | Extend `classifyResult` table with uncovered stderr signals; add `resolveModel` role-file-missing case; add `buildRolePrompt` skills-empty case; add `validateDelegation` parse-error case; add `readActiveRole` error case |
| `internal/cli/init_test.go` | Add `generateMillYAML` error-path tests; add `prompt` default/EOF tests; add `copyScaffold` error test |
| `internal/cli/role_test.go` | Extend existing tests for `runRole` unknown subcommand, `roleGet` empty file, `roleSet` write-error |
| `internal/cli/status_test.go` | No changes needed (already 90.9%) |

No source files are modified. No new files are created except test additions within existing `*_test.go` files.

## Risks

### Risk 1: Flaky coverage from randomized inputs
**Severity:** Low. **Mitigation:** Tests use deterministic inputs. `t.TempDir()` guarantees isolation. No `math/rand`, no time-dependent logic.

### Risk 2: `runLand` success test requires real `git` binary
**Severity:** Low. **Mitigation:** The test uses `exec.Command("git", ...)` against `t.TempDir()`. If `git` is absent, the test fails fast with a clear message. CI and developer machines have `git`. This matches the existing `runLand` test pattern (`TestRunLandEmptyGates` already spawns `git`).

### Risk 3: Test interleaving or shared state
**Severity:** Low. **Mitigation:** Every test uses its own `t.TempDir()`. No package-level variables are mutated. Existing tests already follow this pattern.

### Risk 4: Coverage approaching but not reaching 90%
**Severity:** Medium. **Mitigation:** After each test addition, run `go test -coverprofile` and verify the delta. If the target is still not met, identify the next-highest uncovered function and add a targeted test. The list above covers all functions below 85% — the weighted sum should exceed 90%.

### Risk 5: `buildPrompt` test feels artificial
**Severity:** Low. **Mitigation:** The function is deprecated but not deleted. Testing it is correct per gate rules. A one-line assertion on the output string suffices — it adds negligible maintenance cost.

## ADR

No new ADR. This is a test coverage gap within a single package — not a cross-cutting architectural decision. Existing ADRs apply:

- **ADR 0001** (Mill as Framework): The CLI adapter remains as escape hatch. Coverage improvements here protect the fallback path.
- **ADR 0002** (Budget Enforcement): `classifyResult` exit code mapping for `-1`/`-2` is already tested; this spec extends coverage to stderr signal paths that ADR 0002 introduced.

## Acceptance criteria

1. `go test -count=1 ./internal/cli/ -cover` reports ≥90.0% statement coverage
2. `bash checks/pre-push` passes with zero failures
3. All existing tests continue to pass (`go test ./internal/cli/`)
4. No new third-party dependencies introduced
5. No source file modified (test files only)
6. `go vet ./internal/cli/` reports zero issues
