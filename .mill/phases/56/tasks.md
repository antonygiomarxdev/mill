# Tasks: Model routing — cheap models write, expensive models review

All tasks modify/update only files listed in the SPEC Components affected table.

## Wave 1 (parallel — config and template are independent)

- [ ] **Add `Models` and `Rate` to Config struct** — role: sr-dev-be, deps: none, est: 25m
  1. `internal/config/config.go` — `Config` struct gains `Models map[string]string` with `json:"models"` tag and `Rate float64` with `json:"rate,omitempty"` tag; zero `Rate` == cost tracking disabled (estimate is null)
  2. `Default()` returns: `Models: map[string]string{"free": "laguna-free", "paid": "laguna-pro", "pro": "laguna-ultra"}`, `Rate: 0` (no cost rate by default — user must configure)
  3. `Load`/`Save` round-trip `Models` and `Rate` correctly; existing `.mill/config.json` files without these fields deserialize with zero values (backward compatible)
  4. Update `TestDefault` in `internal/config/config_test.go`: assert `Models` contains the three tier keys and `Rate == 0`
  5. Add `TestConfigModelsRoundTrip` to `internal/config/config_test.go`: save Config with custom Models+Rate, reload, assert equality
  6. Add `TestConfigModelsBackwardCompat` to `internal/config/config_test.go`: load a JSON blob without `models`/`rate`, assert `Models` is non-nil but empty (or nil is fine — zero value), `Rate == 0`
  7. Verify `go test ./internal/config/` passes all tests

- [ ] **Update `mill.yml` template with models/rate sections** — role: sr-dev-be, deps: none, est: 15m
  1. Replace the existing `mill.yml:5-26` (`models:` block with provider lists) with a flat tier→model-name map matching the Config struct:
     ```yaml
     models:
       free: "laguna-free"
       paid: "laguna-pro"
       pro: "laguna-ultra"
     ```
  2. Add `rate: 0.00001` after the `models:` block (dollars per token, approximate; `0.00001` = $10/MTok placeholder — user tunes to their provider pricing). Add a YAML comment: `# dollars per token — set to your provider's rate for cost estimates`
  3. Ensure YAML is valid: `python3 -c "import yaml; yaml.safe_load(open('mill.yml'))"` exits 0

## Wave 2 (depends on Task 1 — needs Config.Models)

- [ ] **Refactor `resolveModel` with escalation logic + availability check** — role: sr-dev-be, deps: Config.Models, est: 45m
  1. Remove the hardcoded `modelTier` variable from `internal/cli/delegate.go:332-337` (obsolete — tiers now resolved from `cfg.Models`)
  2. Add `escalateTier(tier string) (string, error)` function: `"free"` → `"paid"`, `"paid"` → `"pro"`, `"pro"` → `"", fmt.Errorf("no model available for role")`. Never downgrade.
  3. Add `modelAvailable(model string) bool` function: runs `model --version` via `exec.Command`; exit 0 → true, any failure → false. Cache the result for 60 seconds in a package-level `var modelCache map[string]modelCacheEntry` with a `cacheUntil time.Time` field. Use `sync.RWMutex` for concurrent access.
  4. Refactor `resolveModel(targetRole string, cfg config.Config) string` to return `(string, error)`:
     - Read ROLE.md frontmatter for `model:` tier (existing logic)
     - If no tier in frontmatter, default to `"paid"` (spec table: Sr. Devs/Lead/UX default to paid)
     - Look up `cfg.Models[tier]`; if not found, escalate via `escalateTier`
     - Check `modelAvailable(model)`; if unavailable, escalate to next tier
     - Return error when `"pro"` is unavailable (no further escalation)
  5. Update all callers of `resolveModel` in `runDelegate` (line 83) to handle the new `(string, error)` return. On error, return a descriptive message: `fmt.Errorf("no model available for role %q: %w", targetRole, err)`
  6. The `--model` flag in `runDelegate` already sets the model field; change behavior: when `--model pro` is passed, use that as the tier (skip ROLE.md lookup), then resolve through `cfg.Models` + escalation + availability. The override must be logged in the ledger entry as `"model-override: pro"` in a new `Override` field (or appended to the Event field as `"dispatch (model-override: pro)"`).
  7. Update `TestResolveModelMissingRoleFile`, `TestResolveModelEmptyModelTier`, `TestResolveModelKnownTier` in `internal/cli/delegate_test.go` for the new `(string, error)` return signature and `cfg.Models`-based lookup (no longer using hardcoded `modelTier`)
  8. Verify `go test ./internal/cli/ -run TestResolveModel` passes

## Wave 3 (depends on Tasks 1, 2)

- [ ] **Cost tracking log `.mill/costs.jsonl`** — role: sr-dev-be, deps: Config.Rate + resolveModel refactor, est: 30m
  1. Create `internal/cli/costs.go` with `func (a *App) logCost(issueNum int, role string, tier string, model string, tokens int, event string)`:
     - Reads `cfg.Rate` from loaded config
     - Computes `costEstimate`: `tokens × rate` if `rate > 0`, else `null` (JSON null via `*float64`)
     - Writes a JSON line to `.mill/costs.jsonl` in the project root: `{"timestamp":"…","issue":N,"role":"…","tier":"…","model":"…","tokens":N,"cost_estimate":…,"event":"…"}`
     - Uses `os.O_CREATE|os.O_APPEND|os.O_WRONLY` (same pattern as `ledger.Append`)
  2. Call `a.logCost()` from `runDispatchLoop` in `internal/cli/delegate.go` after the agent session completes (after `session.Wait()` returns a result): extract token count from the session result (adapter-dependent; use `SessionResult` — if the result doesn't expose tokens yet, pass `tokens=0` as a placeholder with a `// TODO: extract token count from adapter` comment). Wire the tier and model resolved by `resolveModel`.
  3. Add `TestLogCostWritesJSONL` to `internal/cli/delegate_test.go` (or `costs_test.go`): dispatch with a fake adapter, verify `.mill/costs.jsonl` exists and contains a valid JSON line with `event: "dispatch"`
  4. Add `TestLogCostNullEstimateWhenNoRate` to `internal/cli/delegate_test.go`: config Rate=0, dispatch, verify `cost_estimate` is `null` in the JSONL line
  5. Add `TestLogCostComputesEstimate` to `internal/cli/delegate_test.go`: config Rate=0.00001, tokens=4500, verify `cost_estimate` is `0.045` in the JSONL line
  6. Verify `go test ./internal/cli/ -run TestLogCost` passes

- [ ] **Test escalation, override, and availability paths** — role: sr-dev-be, deps: resolveModel refactor, est: 35m
  1. `TestResolveModelEscalationFreeToPaid`: set `cfg.Models["free"] = "free-model"`, make `modelAvailable("free-model")` return false (mock via a test hook or temporary command), `modelAvailable("paid-model")` return true; verify `resolveModel` returns `"paid-model"` and no error
  2. `TestResolveModelEscalationPaidToPro`: `paid` unavailable, `pro` available → returns pro model
  3. `TestResolveModelEscalationProError`: `pro` unavailable → returns error `"no model available for role"`
  4. `TestResolveModelNeverDowngrades`: `paid` unavailable, `free` available but `paid` was the tier → escalates to `pro`, never falls back to `free`
  5. `TestDelegateModelFlagOverride`: run `mill delegate 1 --model pro`, verify the resolved model comes from `cfg.Models["pro"]` and the ledger entry records `"model-override: pro"`
  6. `TestResolveModelTierFromConfig`: when `model:` frontmatter says `"paid"` but `cfg.Models["paid"]` is `"custom-paid-v2"`, verify the resolved model is `"custom-paid-v2"` (uses config, not hardcoded tier map)
  7. `TestModelAvailableCacheHit`: call `modelAvailable` twice for the same model — second call must not re-execute the command (verify via a call-counting wrapper or by checking cache timestamp)
  8. All new tests pass: `go test -count=1 ./internal/cli/ -run "TestResolveModel|TestDelegateModel|TestModelAvailable"`
