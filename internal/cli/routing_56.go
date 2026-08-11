package cli

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/role"
)

type modelCacheEntry struct {
	available  bool
	cacheUntil time.Time
}

var (
	modelCache   = make(map[string]modelCacheEntry)
	modelCacheMu sync.RWMutex
)

var modelAvailableFn = defaultModelAvailable

func defaultModelAvailable(model string) bool {
	modelCacheMu.RLock()
	if e, ok := modelCache[model]; ok && time.Now().Before(e.cacheUntil) {
		modelCacheMu.RUnlock()
		return e.available
	}
	modelCacheMu.RUnlock()

	available := exec.Command(model, "--version").Run() == nil

	modelCacheMu.Lock()
	modelCache[model] = modelCacheEntry{
		available:  available,
		cacheUntil: time.Now().Add(60 * time.Second),
	}
	modelCacheMu.Unlock()

	return available
}

func escalateTier(tier string) (string, error) {
	switch tier {
	case "free":
		return "paid", nil
	case "paid":
		return "pro", nil
	default:
		return "", fmt.Errorf("no model available for role")
	}
}

// roleCategory returns the model routing category for a role name:
// "review" for reviewer, "implement" for sr-dev-*, "general" otherwise.
func roleCategory(roleName string) string {
	if roleName == "reviewer" {
		return "review"
	}
	if strings.HasPrefix(roleName, "sr-dev-") {
		return "implement"
	}
	return "general"
}
// tierKeyForModel finds the tier key in models that maps to the given model string.
// Standard tiers (free, paid, pro) are checked first for deterministic results.
func tierKeyForModel(models map[string]string, model string) string {
	for _, t := range []string{"free", "paid", "pro"} {
		if models[t] == model {
			return t
		}
	}
	for k, v := range models {
		if v == model {
			return k
		}
	}
	return ""
}
// resolveModel resolves the model to use for a target role.
//
// Priority chain (first match wins):
//  1. Flag override: modelOverride is non-empty → look up cfg.Models[modelOverride],
//     then availability-check. Error if tier not found.
//  2. Category override: if targetRole is "reviewer" and cfg.Models["review"] exists,
//     or targetRole starts with "sr-dev-" and cfg.Models["implement"] exists,
//     use that model with availability escalation.
//  3. Role frontmatter tier: read ROLE.md model field, resolve "free→paid" → "free".
//  4. Default tier "paid" if no frontmatter or no model field.
//  5. Tier lookup in cfg.Models with escalation: free→paid→pro.
//  6. Availability check with tier escalation.
//  7. Global fallback: cfg.Model if all else fails.
func (a *App) resolveModel(targetRole string, modelOverride string, cfg config.Config) (string, error) {
	// Step 1: Flag override (--model flag)
	if modelOverride != "" {
		model, ok := cfg.Models[modelOverride]
		if !ok {
			return "", fmt.Errorf("model tier %q not found in config", modelOverride)
		}
		tier := modelOverride
		for !modelAvailableFn(model) {
			log.Printf("Model tier %q unavailable, escalating", tier)
			nextTier, err := escalateTier(tier)
			if err != nil {
				if cfg.Model != "" {
					return cfg.Model, nil
				}
				return "", err
			}
			tier = nextTier
			var ok bool
			model, ok = cfg.Models[tier]
			if !ok {
				if cfg.Model != "" {
					return cfg.Model, nil
				}
				return "", fmt.Errorf("no model available for role")
			}
		}
		return model, nil
	}

	// Step 2: Category override (review / implement keys)
	cat := roleCategory(targetRole)
	if cat == "review" {
		if m, ok := cfg.Models["review"]; ok {
			for !modelAvailableFn(m) {
				tier := tierKeyForModel(cfg.Models, m)
				if tier == "" {
					if cfg.Model != "" {
						return cfg.Model, nil
					}
					return "", fmt.Errorf("no model available for role")
				}
				nextTier, err := escalateTier(tier)
				if err != nil {
					if cfg.Model != "" {
						return cfg.Model, nil
					}
					return "", err
				}
				var rok bool
				m, rok = cfg.Models[nextTier]
				if !rok {
					if cfg.Model != "" {
						return cfg.Model, nil
					}
					return "", fmt.Errorf("no model available for role")
				}
			}
			return m, nil
		}
	}
	if cat == "implement" {
		if m, ok := cfg.Models["implement"]; ok {
			for !modelAvailableFn(m) {
				tier := tierKeyForModel(cfg.Models, m)
				if tier == "" {
					if cfg.Model != "" {
						return cfg.Model, nil
					}
					return "", fmt.Errorf("no model available for role")
				}
				nextTier, err := escalateTier(tier)
				if err != nil {
					if cfg.Model != "" {
						return cfg.Model, nil
					}
					return "", err
				}
				var iok bool
				m, iok = cfg.Models[nextTier]
				if !iok {
					if cfg.Model != "" {
						return cfg.Model, nil
					}
					return "", fmt.Errorf("no model available for role")
				}
			}
			return m, nil
		}
	}

	// Step 3-5: Role frontmatter tier → default tier → tier lookup + escalation
	tier := "paid"
	root, err := projectRoot()
	if err == nil {
		rolePath := filepath.Join(root, ".mill", "roles", targetRole, "ROLE.md")
		fm, ferr := role.ParseFrontmatter(rolePath)
		if ferr == nil && fm.Model != "" {
			tier = fm.Model
			if tier == "free→paid" {
				tier = "free"
			}
		}
	}

	model, ok := cfg.Models[tier]
	if !ok {
		var err error
		tier, err = escalateTier(tier)
		if err != nil {
			return "", fmt.Errorf("no model available for tier %q", tier)
		}
		model = cfg.Models[tier]
		if model == "" && cfg.Model != "" {
			return cfg.Model, nil
		}
	}

	if model == "" && cfg.Model != "" {
		return cfg.Model, nil
	}

	// Step 6: Availability check with escalation
	for !modelAvailableFn(model) {
		log.Printf("Model tier %q unavailable, escalating", tier)
		nextTier, err := escalateTier(tier)
		if err != nil {
			if cfg.Model != "" {
				return cfg.Model, nil
			}
			return "", fmt.Errorf("no model available for role")
		}
		tier = nextTier
		var ok bool
		model, ok = cfg.Models[tier]
		if !ok {
			if cfg.Model != "" {
				return cfg.Model, nil
			}
			return "", fmt.Errorf("no model available for role")
		}
	}

	return model, nil
}

// resolveModelLegacy wraps resolveModel for callers that expect a string return
// (no error). On error, returns cfg.Model as the fallback.
func (a *App) resolveModelLegacy(targetRole string, cfg config.Config) string {
	model, err := a.resolveModel(targetRole, "", cfg)
	if err != nil {
		return cfg.Model
	}
	return model
}
