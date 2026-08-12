package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/config"
)

func TestResolveModel56EscalationFreeToPaid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: free\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return model != "free-model" }

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{
		"free": "free-model", "paid": "paid-model", "pro": "pro-model",
	}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "paid-model" {
		t.Errorf("expected escalation to paid-model, got %q", got)
	}
}

func TestResolveModel56EscalationPaidToPro(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: paid\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return model != "paid-model" }

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{
		"free": "free-model", "paid": "paid-model", "pro": "pro-model",
	}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "pro-model" {
		t.Errorf("expected escalation to pro-model, got %q", got)
	}
}

func TestResolveModel56EscalationProError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "architect")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: architect\nmodel: pro\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return false }

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{
		"free": "free-model", "paid": "paid-model", "pro": "pro-model",
	}}
	_, err := app.resolveModel("architect", "", cfg)
	if err == nil {
		t.Fatal("expected error when pro is unavailable")
	}
	if !strings.Contains(err.Error(), "no model available") {
		t.Errorf("expected 'no model available' error, got %v", err)
	}
}

func TestResolveModel56NeverDowngrades(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: paid\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool {
		return model == "free-model" || model == "pro-model"
	}

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{
		"free": "free-model", "paid": "paid-model", "pro": "pro-model",
	}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "pro-model" {
		t.Errorf("expected escalation to pro (not downgrade to free), got %q", got)
	}
}

func TestResolveModel56TierFromConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: paid\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	modelAvailableFn = func(model string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{
		"free": "laguna-free",
		"paid": "custom-paid-v2",
		"pro":  "laguna-ultra",
	}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "custom-paid-v2" {
		t.Errorf("expected config model custom-paid-v2, got %q", got)
	}
}

func TestResolveModel56FlagOverride(t *testing.T) {
	origFn := modelAvailableFn
	modelAvailableFn = func(model string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: "."}
	cfg := config.Config{Models: map[string]string{
		"free": "laguna-free",
		"paid": "laguna-pro",
		"pro":  "laguna-ultra",
	}}
	got, err := app.resolveModel("", "pro", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-ultra" {
		t.Errorf("expected laguna-ultra from --model pro, got %q", got)
	}
}

func TestResolveModelLegacyBridge(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: paid\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	modelAvailableFn = func(model string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{
		"free": "laguna-free",
		"paid": "laguna-pro",
		"pro":  "laguna-ultra",
	}}
	got := app.resolveModelLegacy("sr-dev-be", cfg)
	if got != "laguna-pro" {
		t.Errorf("expected laguna-pro via legacy bridge, got %q", got)
	}
}

func TestModelAvailableCacheHit(t *testing.T) {
	modelCacheMu.Lock()
	modelCache = make(map[string]modelCacheEntry)
	modelCacheMu.Unlock()

	// Restore real defaultModelAvailable to test cache behavior
	origFn := modelAvailableFn
	modelAvailableFn = defaultModelAvailable
	defer func() { modelAvailableFn = origFn }()

	// Use a model name that is 'available' — we use "true" command which always exits 0
	result1 := modelAvailableFn("true")
	if !result1 {
		t.Fatal("expected true to be available")
	}

	// Verify cache was populated
	modelCacheMu.RLock()
	_, cached := modelCache["true"]
	modelCacheMu.RUnlock()
	if !cached {
		t.Fatal("expected cache entry for 'true'")
	}

	// Second call should hit cache
	result2 := modelAvailableFn("true")
	if !result2 {
		t.Error("expected cached true to be available")
	}
}

func TestModelAvailableCacheExpiry(t *testing.T) {
	modelCacheMu.Lock()
	modelCache = make(map[string]modelCacheEntry)
	modelCacheMu.Unlock()

	callCount := 0
	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool {
		modelCacheMu.RLock()
		if e, ok := modelCache[model]; ok && time.Now().Before(e.cacheUntil) {
			modelCacheMu.RUnlock()
			return e.available
		}
		modelCacheMu.RUnlock()
		callCount++
		modelCacheMu.Lock()
		modelCache[model] = modelCacheEntry{available: true, cacheUntil: time.Now().Add(-1 * time.Second)}
		modelCacheMu.Unlock()
		return true
	}

	modelAvailableFn("test-model")
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
	modelAvailableFn("test-model")
	if callCount != 2 {
		t.Errorf("expected 2 calls (cache expired), got %d", callCount)
	}
}

func TestLogCost56WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir}
	app.logCost(config.Config{Rate: 0}, 55, "sr-dev-be", "paid", "laguna-pro", 0, "dispatch")
	data, err := os.ReadFile(app.costsPath())
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	var entry costEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry.Issue != 55 || entry.Role != "sr-dev-be" || entry.Tier != "paid" || entry.Event != "dispatch" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestLogCost56NullEstimateWhenNoRate(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir}
	app.logCost(config.Config{Rate: 0}, 42, "reviewer", "pro", "laguna-ultra", 5000, "review")
	data, _ := os.ReadFile(app.costsPath())
	var entry costEntry
	json.Unmarshal(data, &entry)
	if entry.CostEstimate != nil {
		t.Errorf("expected null, got %v", *entry.CostEstimate)
	}
}

func TestLogCost56ComputesEstimate(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir}
	app.logCost(config.Config{Rate: 0.00001}, 55, "sr-dev-be", "paid", "laguna-pro", 4500, "dispatch")
	data, _ := os.ReadFile(app.costsPath())
	var entry costEntry
	json.Unmarshal(data, &entry)
	if entry.CostEstimate == nil {
		t.Fatal("expected cost_estimate")
	}
	diff := *entry.CostEstimate - 0.045
	if diff < -0.0001 || diff > 0.0001 {
		t.Errorf("expected ~0.045, got %f", *entry.CostEstimate)
	}
}

func TestLogCost56AppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir}
	cfg := config.Config{Rate: 0.00001}
	app.logCost(cfg, 55, "sr-dev-be", "paid", "laguna-pro", 1000, "dispatch")
	app.logCost(cfg, 55, "reviewer", "pro", "laguna-ultra", 2000, "review")
	data, _ := os.ReadFile(app.costsPath())
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestStageLabelToModelProduce(t *testing.T) {
	got := stageLabelToModel("stage:produce")
	if got != "laguna-free" {
		t.Errorf("expected 'laguna-free' for stage:produce, got %q", got)
	}
}

func TestStageLabelToModelReview(t *testing.T) {
	got := stageLabelToModel("stage:review")
	if got != "laguna-pro" {
		t.Errorf("expected 'laguna-pro' for stage:review, got %q", got)
	}
}

func TestStageLabelToModelImplement(t *testing.T) {
	got := stageLabelToModel("stage:implement")
	if got != "laguna-free" {
		t.Errorf("expected 'laguna-free' for stage:implement, got %q", got)
	}
}

func TestStageLabelToModelUnknown(t *testing.T) {
	got := stageLabelToModel("stage:unknown")
	if got != "laguna-free" {
		t.Errorf("expected 'laguna-free' fallback for unknown, got %q", got)
	}
}

func TestResolveModelProjectRootError(t *testing.T) {
	// Change to a directory where projectRoot fails (no go.mod).
	// resolveModelLegacy wraps resolveModel and falls back to cfg.Model on error.
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "fallback-model"}
	got := app.resolveModelLegacy("sr-dev-be", cfg)
	if got != "fallback-model" {
		t.Errorf("expected fallback model, got %q", got)
	}
}

func TestResolveModelEscalateTierFromRole(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: free\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	// free tier not in Models map → escalateTier to paid
	cfg := config.Config{Models: map[string]string{"paid": "paid-model", "pro": "pro-model"}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "paid-model" {
		t.Errorf("expected 'paid-model' after escalation, got %q", got)
	}
}

func TestResolveModel56Legacy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel: free\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	cfg := config.Config{Model: "legacy-model", Models: map[string]string{"free": "free-model", "paid": "paid-model"}}
	got := app.resolveModelLegacy("sr-dev-be", cfg)
	if got != "free-model" {
		t.Errorf("expected 'free-model', got %q", got)
	}
}

func TestResolveModel56EmptyModelInRole(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	// Model field is empty in frontmatter
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: sr-dev-be\nmodel:\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{"paid": "paid-model", "pro": "pro-model"}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "paid-model" {
		t.Errorf("expected 'paid-model' when model is empty in role, got %q", got)
	}
}

func TestResolveModelLegacyErrorFallback(t *testing.T) {
	app := &App{MillDir: t.TempDir()}
	// resolveModel56 will fail because projectRoot can't find go.mod
	// and there's no valid cfg.Model
	cfg := config.Config{Model: "fallback-model"}
	got := app.resolveModelLegacy("sr-dev-be", cfg)
	if got != "fallback-model" {
		t.Errorf("expected 'fallback-model' on error, got %q", got)
	}
}

func TestResolveModel56BrokenRoleFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "sr-dev-be")
	os.MkdirAll(roleDir, 0o755)
	// Invalid frontmatter (no closing ---)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("garbage content"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return true }

	app := &App{MillDir: dir}
	cfg := config.Config{Models: map[string]string{"paid": "paid-model", "pro": "pro-model"}}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "paid-model" {
		t.Errorf("expected 'paid-model' when role file is broken, got %q", got)
	}
}

// --- Category override tests (AC 1-3) ---

func TestResolveModelReviewCategoryOverride(t *testing.T) {
	// AC 2: models.review set → Reviewer role uses that model
	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: "."}
	cfg := config.Config{
		Models: map[string]string{
			"free":   "laguna-free",
			"paid":   "laguna-pro",
			"pro":    "laguna-ultra",
			"review": "laguna-mega",
		},
	}
	got, err := app.resolveModel("reviewer", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-mega" {
		t.Errorf("expected 'laguna-mega' from review category override, got %q", got)
	}
}

func TestResolveModelImplementCategoryOverride(t *testing.T) {
	// AC 3: models.implement set → Sr Dev roles use that model
	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: "."}
	cfg := config.Config{
		Models: map[string]string{
			"free":      "laguna-free",
			"paid":      "laguna-pro",
			"pro":       "laguna-ultra",
			"implement": "laguna-cheap",
		},
	}
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-cheap" {
		t.Errorf("expected 'laguna-cheap' from implement category override, got %q", got)
	}
}

func TestResolveModelImplementCategorySrDevFe(t *testing.T) {
	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: "."}
	cfg := config.Config{
		Models: map[string]string{"free": "laguna-free", "paid": "laguna-pro", "implement": "laguna-cheap"},
	}
	got, err := app.resolveModel("sr-dev-fe", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-cheap" {
		t.Errorf("expected 'laguna-cheap' for sr-dev-fe, got %q", got)
	}
}

func TestResolveModelCategoryFallthroughWhenKeyMissing(t *testing.T) {
	// When review key is missing, fall through to tier-based resolution
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644)
	roleDir := filepath.Join(dir, ".mill", "roles", "reviewer")
	os.MkdirAll(roleDir, 0o755)
	os.WriteFile(filepath.Join(roleDir, "ROLE.md"), []byte("---\nrole: reviewer\nmodel: paid\n---\n"), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: dir}
	cfg := config.Config{
		Models: map[string]string{"free": "laguna-free", "paid": "laguna-pro", "pro": "laguna-ultra"},
	}
	// No "review" key → uses tier from ROLE.md (paid)
	got, err := app.resolveModel("reviewer", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-pro" {
		t.Errorf("expected 'laguna-pro' from tier fallthrough, got %q", got)
	}
}

func TestResolveModelFlagOverridesCategoryOverride(t *testing.T) {
	// --model flag takes priority over category override
	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	t.Cleanup(func() { modelAvailableFn = origFn })

	app := &App{MillDir: "."}
	cfg := config.Config{
		Models: map[string]string{
			"free":   "laguna-free",
			"paid":   "laguna-pro",
			"pro":    "laguna-ultra",
			"review": "laguna-mega",
		},
	}
	// Flag override "pro" takes priority over review category
	got, err := app.resolveModel("reviewer", "pro", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-ultra" {
		t.Errorf("expected 'laguna-ultra' from flag override, got %q", got)
	}
}

func TestResolveModelCategoryUnavailableEscalates(t *testing.T) {
	// When category override model is unavailable, escalate tier
	origFn := modelAvailableFn
	defer func() { modelAvailableFn = origFn }()
	modelAvailableFn = func(model string) bool { return model != "laguna-cheap" }

	app := &App{MillDir: "."}
	cfg := config.Config{
		Models: map[string]string{
			"free":      "laguna-free",
			"paid":      "laguna-pro",
			"implement": "laguna-cheap",
		},
	}
	// "laguna-cheap" unavailable → tierKeyForModel returns "implement" (custom key)
	// → escalateTier("implement") → error → fallback to cfg.Model
	cfg.Model = "laguna-free"
	got, err := app.resolveModel("sr-dev-be", "", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "laguna-free" {
		t.Errorf("expected fallback to cfg.Model, got %q", got)
	}
}
