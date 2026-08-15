# Spec: Model routing — cheap models write, expensive models review

## Architecture

**Problem:** `runDispatchLoop` (delegate.go:227) hardcodes `reviewModel := "laguna-pro"` (line 247) and resolves the produce model from stage labels only. The existing `resolveModel` in delegate.go:691 uses role frontmatter tiers but does not distinguish produce-vs-review roles by category — it depends on convention (the Reviewer role happens to declare `model: pro`). The PM spec requires explicit `models.review` / `models.implement` keys so routing is enforced, not coincidental.

**Solution:** Extend the `Config.Models` map with two specialized keys (`review`, `implement`) that take precedence over tier-based resolution when the target role falls into the review or implement category. Integrate the availability-check logic from `routing_56.go` (lines 58-106) into the resolution pipeline so unavailability escalates the tier before the model is returned.

### Config schema

The `Config.Models` map (already `map[string]string` with `json:"models"` on config.go:41) gains two optional keys:

```yaml
# mill.yml
models:
  free: "laguna-free"
  paid: "laguna-pro"
  pro: "laguna-ultra"
  review: "laguna-ultra"          # NEW: override for Reviewer role
  implement: "laguna-free"        # NEW: override for implementor roles (sr-dev-*)
```

**Backward compatibility:** Existing configs without `review`/`implement` keys continue to work. The Config struct (`map[string]string`) already accepts arbitrary keys — no struct change needed. When `review` or `implement` is absent from the map, the resolver falls through to tier-based resolution (existing behavior). The legacy `model` field on Config (line 34, `json:"model"`) is the ultimate fallback when no tier or category key matches.

**Key semantics:**
- `review` — when present, used for the Reviewer role regardless of ROLE.md `model:` tier
- `implement` — when present, used for implementor roles (sr-dev-fe, sr-dev-be, sr-dev-data) regardless of ROLE.md `model:` tier
- All other keys (`free`, `paid`, `pro`, custom tiers) — tier-based resolution as today

### Role-to-model mapping

**Role classification** (new helper, not in existing code):

Roles belong to exactly one of three categories for model routing:

| Category | Roles | Determination |
|---|---|---|
| `review` | reviewer | Role name is `"reviewer"` |
| `implement` | sr-dev-fe, sr-dev-be, sr-dev-data | Role name starts with `"sr-dev-"` |
| `general` | staff, architect, tech-lead, pm, qa-docs, ux-designer, ui-designer, any unknown | Fallthrough: not review, not implement |

**Resolver algorithm** (replaces `resolveModel` in delegate.go:691 and `resolveModel56` in routing_56.go:58):

```
resolveModel(targetRole, modelOverride, cfg Config) → (model string, error)
```

Priority chain (first match wins):

1. **Flag override** (`--model pro`): `modelOverride` is non-empty → treat it as a tier key, look up `cfg.Models[modelOverride]`. If not found, error.

2. **Category override** (NEW): Determine role category.
   - `review` category + `cfg.Models["review"]` exists → return that model string directly.
   - `implement` category + `cfg.Models["implement"]` exists → return that model string directly.
   - `general` category OR category key absent in config → fall through to step 3.

3. **Role frontmatter tier**: Read ROLE.md frontmatter `model:` field (existing `role.ParseFrontmatter`). If present, resolve through `cfg.Models`. "free→paid" resolves to "free" on first lookup (existing behavior, delegate.go:714-717).

4. **Default tier**: If no frontmatter or no `model:` field, use `"paid"` as default tier (matches existing behavior in routing_56.go:79: `tier = "paid"`).

5. **Tier lookup + escalation**: Look up `cfg.Models[tier]`. If not found, escalate via `escalateTier` (routing_56.go:47-56): free→paid→pro. Pro has no fallback → error.

6. **Availability check**: For the resolved model string, call `modelAvailableFn(model)`. If unavailable, escalate tier again and retry. If pro tier is unavailable → error "no model available for role".

7. **Global fallback**: If all resolution fails, return `cfg.Model` (the legacy top-level model field).

**Key difference from current `resolveModel` (delegate.go:691):** The existing function returns a string (no error) and does not do availability checking. The new function returns `(string, error)` and incorporates the availability loop from `routing_56.go:95-103`.

**Key difference from current `resolveModel56` (routing_56.go:58):** The existing function does not have category-based overrides (review/implement). It only has stage-label shortcuts (lines 59-68) which are orthogonal — stage labels still work for the produce phase, but the new algorithm resolves first by role category, then by tier.

### Review loop integration

**Current state:** `runDispatchLoop` (delegate.go:227) hardcodes:
```go
produceModel := a.resolveModel("", stageLabel, cfg)  // line 242
reviewModel := "laguna-pro"                           // line 247
```

**Required change:** Both models MUST come from `resolveModel`:

```go
produceModel, err := a.resolveModel(produceRole, modelOverride, cfg)
reviewModel, err := a.resolveModel("reviewer", modelOverride, cfg)
```

Where `produceRole` is the role assigned to the produce phase (the implementor role from the task or the default sr-dev). The `modelOverride` comes from the `--model` flag, passed through `runDelegate` → `runDispatchLoop`. When `--model` is set, BOTH produce and review use the override (same as today's behavior where a single model is used for everything).

**Model preference propagation:** The review loop must receive the resolved model as part of dispatch options, not a hardcoded string. This is already supported: `adapter.DispatchOpts.Model` accepts any model string (adapter.go:21). No adapter changes needed.

**Stage labels still matter:** When issue labels include `stage:produce` or `stage:review`, the stage shortcut in the resolver (routing_56.go:59-68) takes priority — this is the existing behavior and remains for backward compat. The stage labels bypass the entire role-based resolution.

### Validation

**Availability check:** `modelAvailableFn` (routing_56.go:25-45) checks whether a model binary is reachable by running `<model> --version`. Results are cached for 60 seconds with `sync.RWMutex`-protected map.

**Fallback chain when model is unavailable:**

1. Model string from resolution (e.g. `"laguna-pro"`)
2. `modelAvailableFn("laguna-pro")` → false
3. Determine current tier: which key in `cfg.Models` maps to this string? Iterate the map.
4. Escalate tier: free→paid, paid→pro, pro→ERROR
5. Resolve new model from escalated tier: `cfg.Models[nextTier]`
6. `modelAvailableFn(newModel)` → check again
7. If pro tier unavailable → return error: `"no model available for role"`

**Edge cases:**
- Category override (review/implement) model is unavailable: treat as if that tier is the starting tier. Escalate from whatever tier corresponds to the model string.
- Multiple models map to same string (e.g. `review: "laguna-ultra"` and `pro: "laguna-ultra"`): escalation resolves to the same model, which will fail availability again → pro-tier error.
- Cache staleness: if a model was cached as unavailable but has since recovered, the next delegation (after 60s cache expiry) will re-check and succeed.
- Unknown model string not in `cfg.Models`: fall through to `cfg.Model`. If `cfg.Model` is also unavailable → error.

### Test strategy

Because model routing is stateless (reads config + frontmatter, returns string), it is testable without real model execution.

**Mock `modelAvailableFn`:** The function is already a package-level variable (routing_56.go:25). Tests replace it:

```go
// In test:
originalFn := modelAvailableFn
modelAvailableFn = func(model string) bool {
    return model != "broken-model"  // simulate unavailability
}
defer func() { modelAvailableFn = originalFn }()
```

**Test categories:**

1. **Config resolution tests** (no role files needed):
   - `cfg.Models["review"]` = "expensive-model", role="reviewer" → returns "expensive-model"
   - `cfg.Models["implement"]` = "cheap-model", role="sr-dev-be" → returns "cheap-model"
   - `cfg.Models` has no `review` key, role="reviewer" → falls through to tier-based (ROLE.md)
   - `cfg.Models` has no `implement` key, role="sr-dev-fe" → falls through to tier-based

2. **Override priority tests:**
   - `--model pro` with role="reviewer" and `cfg.Models["review"]` set → uses `cfg.Models["pro"]` (flag wins)
   - `--model pro` with role="sr-dev-be" and `cfg.Models["implement"]` set → uses `cfg.Models["pro"]`

3. **Tier escalation tests:**
   - Tier "paid" unavailable → escalates to "pro" → returns pro model
   - Tier "pro" unavailable → returns error
   - Never downgrades: "paid" unavailable, "free" available → still escalates to "pro"

4. **Availability check tests:**
   - Model available → returns model
   - Model unavailable → escalates tier → retries
   - Cache: second call for same model within 60s → no re-exec of version check

5. **Backward compat tests:**
   - Config without `review`/`implement` keys → tier-based resolution still works
   - Config with only legacy `model` field → `cfg.Model` is ultimate fallback
   - ROLE.md with `model: free→paid` → resolves to "free" tier on first pass

6. **Integration-level (review loop):**
   - `runDispatchLoop` with role="sr-dev-be" + `models.implement` set → produce uses cheap model
   - `runDispatchLoop` with role="reviewer" + `models.review` set → review uses expensive model
   - Both phases use flag override when `--model` is passed

**Test files:** `internal/cli/routing_56_test.go` (new or added to existing) and `internal/cli/delegate_test.go` (review loop tests).

## Components affected

| File | Change |
|---|---|
| `internal/cli/routing_56.go` | MODIFY: `resolveModel56` gains category detection + review/implement key lookup. Remove stage-label shortcuts (moved to caller). Rename to `resolveModel`. |
| `internal/cli/delegate.go` | MODIFY: Remove old `resolveModel` (line 691). `runDispatchLoop` calls new `resolveModel` for both produce and review. Remove hardcoded `reviewModel := "laguna-pro"`. Remove obsolete `modelTier` var (line 675). |
| `internal/cli/routing_56_test.go` | NEW: Category override, escalation, availability, backward compat tests |
| `internal/cli/delegate_test.go` | MODIFY: Review loop tests verify model routing per role |
| `internal/config/config.go` | No schema change needed — `Models map[string]string` already accepts arbitrary keys |
| `mill.yml` template | MODIFY: Add `review:` and `implement:` keys with defaults |

### Files NOT affected
- `internal/adapter/` — model name is opaque to adapter
- `internal/domain/` — no new types
- `internal/state/` — no schema changes
- `internal/role/` — ROLE.md frontmatter unchanged; `model:` field semantics unchanged

## Risks

### Risk 1: Config map key collision
**Severity:** Low. If a user names a tier `review` or `implement`, it collides with the category keys. **Mitigation:** The resolver checks role category BEFORE tier lookup. If the role is "reviewer" and `cfg.Models["review"]` exists, it uses that key as a category override, not a tier. This is unambiguous because the category check is gated on the role name. A tier named "review" used by a non-reviewer role would still work as a tier — the category check only activates for the Reviewer role.

### Risk 2: Review loop model resolution adds latency
**Severity:** Low. **Mitigation:** `resolveModel` is called once per phase (twice per round). The availability check is cached for 60s. ROLE.md parsing is a single `os.ReadFile`. Total added latency: ~1ms per call. Negligible compared to agent session time (minutes).

### Risk 3: Hardcoded tier names in escalateTier
**Severity:** Low. **Mitigation:** `escalateTier` (routing_56.go:47) already has the free→paid→pro chain. If custom tier names are added to `cfg.Models`, they won't be in the escalation chain and will fail with the default error. This is acceptable — escalation should be explicit. Future work could make escalation configurable.

## ADR

**UPDATED ADR: ADR 0007 — Model routing with category overrides.** Original ADR (existing spec.md:113-118) established three-tier routing. This update adds:

- **Category-based model selection** (`review` / `implement` keys) takes precedence over tier-based selection when the role matches. This enforces "barato writes, caro reviews" at the config level rather than relying on role frontmatter convention.
- **Resolution priority:** flag override > category override > role frontmatter tier > global model. This is a strict total order — no ambiguity.
- **Availability escalation** runs AFTER the model string is resolved from config, not before. This ensures the escalation chain starts from the correct tier.
- **Stage labels** (stage:produce / stage:review) remain as shortcuts that bypass the entire role-based resolver. These are used by the review loop caller, not by `resolveModel` itself.

## Acceptance criteria

1. `mill.yml` `models:` map supports optional `review` and `implement` keys (no schema change needed)
2. When `models.review` is set, Reviewer role uses that model (overrides ROLE.md tier)
3. When `models.implement` is set, Sr Dev roles use that model (overrides ROLE.md tier)
4. `--model` flag overrides both category and tier settings for that delegation
5. Existing tier-based resolution (ROLE.md model → cfg.Models[tier]) works unchanged when review/implement keys are absent
6. Unavailable model escalates tier: free→paid→pro; pro unavailable → error
7. `runDispatchLoop` uses resolved models for both produce and review phases (no hardcoded strings)
8. `go test ./internal/cli/ -run "TestResolveModel"` passes (new routing tests)
9. `go test ./internal/cli/ -run "TestDispatchLoop"` passes (review loop model routing)
