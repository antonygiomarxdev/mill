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

// adapterModelFallback returns a model from the adapter's fallback chain
// for the given tier, or the adapter's default model if the chain is empty.
func (a *App) adapterModelFallback(tier string) (string, bool) {
	if a.Adapter == nil {
		return "", false
	}
	chain := a.Adapter.DefaultFallbackChain()[tier]
	if len(chain) > 0 {
		return chain[0], true
	}
	if m := a.Adapter.DefaultModel(); m != "" {
		return m, true
	}
	return "", false
}

// resolveFallbackModel attempts to resolve cfg.Model through the adapter's
// fallback chain. If cfg.Model is a tier alias (e.g. "free"), it returns
// the first model in that chain. Otherwise returns cfg.Model as-is.
func (a *App) resolveFallbackModel(cfg config.Config) string {
	if cfg.Model == "" {
		return ""
	}
	if a.Adapter != nil {
		if chain := a.Adapter.DefaultFallbackChain()[cfg.Model]; len(chain) > 0 {
			return chain[0]
		}
	}
	return cfg.Model
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
			if m, found := a.adapterModelFallback(modelOverride); found {
				return m, nil
			}
			return "", fmt.Errorf("model tier %q not found in config", modelOverride)
		}
		tier := modelOverride
		for !modelAvailableFn(model) {
			log.Printf("Model tier %q unavailable, escalating", tier)
			nextTier, err := escalateTier(tier)
			if err != nil {
				if m, found := a.adapterModelFallback(tier); found {
					return m, nil
				}
				if cfg.Model != "" {
					return a.resolveFallbackModel(cfg), nil
				}
				return "", err
			}
			tier = nextTier
			var ok2 bool
			model, ok2 = cfg.Models[tier]
			if !ok2 {
				if m, found := a.adapterModelFallback(tier); found {
					return m, nil
				}
				if cfg.Model != "" {
					return a.resolveFallbackModel(cfg), nil
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
					if m, found := a.adapterModelFallback(tier); found {
						return m, nil
					}
					if cfg.Model != "" {
						return a.resolveFallbackModel(cfg), nil
					}
					return "", fmt.Errorf("no model available for role")
				}
				nextTier, err := escalateTier(tier)
				if err != nil {
					if m, found := a.adapterModelFallback(tier); found {
						return m, nil
					}
					if cfg.Model != "" {
						return a.resolveFallbackModel(cfg), nil
					}
					return "", err
				}
				var rok bool
				m, rok = cfg.Models[nextTier]
				if !rok {
					if m, found := a.adapterModelFallback(tier); found {
						return m, nil
					}
					if cfg.Model != "" {
						return a.resolveFallbackModel(cfg), nil
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
					if m, found := a.adapterModelFallback(tier); found {
						return m, nil
					}
					if cfg.Model != "" {
						return a.resolveFallbackModel(cfg), nil
					}
					return "", fmt.Errorf("no model available for role")
				}
				nextTier, err := escalateTier(tier)
				if err != nil {
					if m, found := a.adapterModelFallback(tier); found {
						return m, nil
					}
					if cfg.Model != "" {
						return a.resolveFallbackModel(cfg), nil
					}
					return "", err
				}
				var iok bool
				m, iok = cfg.Models[nextTier]
				if !iok {
					if m, found := a.adapterModelFallback(tier); found {
						return m, nil
					}
					if cfg.Model != "" {
						return a.resolveFallbackModel(cfg), nil
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
			if m, found := a.adapterModelFallback(tier); found {
				return m, nil
			}
			if cfg.Model != "" {
				return a.resolveFallbackModel(cfg), nil
			}
			return "", fmt.Errorf("no model available for tier %q", tier)
		}
		model = cfg.Models[tier]
		if model == "" {
			if m, found := a.adapterModelFallback(tier); found {
				return m, nil
			}
			if cfg.Model != "" {
				return a.resolveFallbackModel(cfg), nil
			}
		}
	}

	if model == "" {
		if m, found := a.adapterModelFallback(tier); found {
			return m, nil
		}
		if cfg.Model != "" {
			return a.resolveFallbackModel(cfg), nil
		}
	}

	// Step 6: Availability check with escalation
	for !modelAvailableFn(model) {
		log.Printf("Model tier %q unavailable, escalating", tier)
		nextTier, err := escalateTier(tier)
		if err != nil {
			if m, found := a.adapterModelFallback(tier); found {
				return m, nil
			}
			if cfg.Model != "" {
				return a.resolveFallbackModel(cfg), nil
			}
			return "", fmt.Errorf("no model available for role")
		}
		tier = nextTier
		var ok2 bool
		model, ok2 = cfg.Models[tier]
		if !ok2 {
			if m, found := a.adapterModelFallback(tier); found {
				return m, nil
			}
			if cfg.Model != "" {
				return a.resolveFallbackModel(cfg), nil
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

// defaultMaxDepth is the deepest org-chart delegation chain, equal to the
// number of hops from staff down to the leaf role:
// staff -> pm -> ux-designer -> ui-designer -> qa-docs (4 hops).
const defaultMaxDepth = 4

// escalateToParent resolves the next automatable parent role for issueNum by
// walking the reviewed_by chain recorded in .mill/roles/<role>/ROLE.md.
// It enforces delegation validity and a configurable depth bound; the caller
// is expected to re-dispatch to the returned parent.
func (a *App) escalateToParent(issueNum int, roleName string, cfg config.Config) (string, error) {
	if roleName == "" {
		return "", fmt.Errorf("escalateToParent: roleName must not be empty")
	}

	// HARD-STOP at Staff: staff has no automatable parent (reviewed_by: cto).
	if roleName == "staff" {
		fmt.Fprintf(a.Err, "escalation: cannot escalate beyond staff for issue %d; notifying CTO\n", issueNum)
		return "", fmt.Errorf("escalation hard-stop: staff has no automatable parent (reviewed_by: cto)")
	}

	root, err := projectRoot()
	if err != nil {
		return "", fmt.Errorf("cannot find project root: %w", err)
	}

	fm, err := role.ParseFrontmatter(filepath.Join(root, ".mill", "roles", roleName, "ROLE.md"))
	if err != nil {
		return "", fmt.Errorf("cannot read role %s: %w", roleName, err)
	}

	parent := strings.TrimSpace(fm.ReviewedBy)
	if parent == "" || parent == "delegator" {
		return "", fmt.Errorf("escalation: %s has no automatable parent (reviewed_by %q empty or delegator)", roleName, fm.ReviewedBy)
	}
	if parent == roleName {
		return "", fmt.Errorf("escalation: cyclic delegation for role %s (reviewed_by points to itself)", roleName)
	}

	// Reuse validation logic: ensure the parent actually lists this role in its
	// delegates_to frontmatter, rejecting invalid/cyclic delegation.
	if err := a.validateDelegation(parent, roleName); err != nil {
		return "", fmt.Errorf("invalid escalation: %w", err)
	}

	// Depth guard: walk the reviewed_by chain from roleName up to staff,
	// counting hops. The chain must terminate at staff; otherwise the role
	// graph is invalid (broken, delegating, or self-referential).
	maxDepth := cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	hops := 0
	current := roleName
	for current != "staff" {
		fm, err = role.ParseFrontmatter(filepath.Join(root, ".mill", "roles", current, "ROLE.md"))
		if err != nil {
			return "", fmt.Errorf("cannot read role %s: %w", current, err)
		}
		next := strings.TrimSpace(fm.ReviewedBy)
		if next == "" || next == "delegator" {
			return "", fmt.Errorf("escalation: chain broken at %s (reviewed_by %q empty or delegator)", current, fm.ReviewedBy)
		}
		if next == current {
			return "", fmt.Errorf("escalation: cyclic delegation for role %s (reviewed_by points to itself)", current)
		}
		hops++
		if hops > maxDepth {
			fmt.Fprintf(a.Err, "escalation: depth limit %d exceeded for issue %d; notifying CTO\n", maxDepth, issueNum)
			return "", fmt.Errorf("escalation hard-stop: depth limit %d exceeded (reviewed_by: cto)", maxDepth)
		}
		current = next
	}

	// Reaches the top automatable role.
	if parent == "staff" {
		fmt.Fprintf(a.Err, "escalation: issue %d reaches the top automatable role (staff)\n", issueNum)
		return parent, nil
	}

	return parent, nil
}
