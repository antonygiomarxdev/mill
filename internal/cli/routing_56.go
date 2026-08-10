package cli

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
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

func (a *App) resolveModel56(targetRole string, modelOverride string, stageLabel string, cfg config.Config) (string, error) {
	if stageLabel != "" {
		switch stageLabel {
		case "stage:produce":
			return "laguna-free", nil
		case "stage:review":
			return "laguna-pro", nil
		case "stage:implement":
			return "laguna-free", nil
		}
	}

	tier := modelOverride
	if tier == "" {
		root, err := projectRoot()
		if err != nil {
			return cfg.Model, nil
		}
		rolePath := filepath.Join(root, ".mill", "roles", targetRole, "ROLE.md")
		fm, err := role.ParseFrontmatter(rolePath)
		if err != nil || fm.Model == "" {
			tier = "paid"
		} else {
			tier = fm.Model
		}
	}

	model, ok := cfg.Models[tier]
	if !ok {
		var err error
		tier, err = escalateTier(tier)
		if err != nil {
			return "", err
		}
		model = cfg.Models[tier]
	}

	for !modelAvailableFn(model) {
		log.Printf("Model tier %q unavailable, escalating", tier)
		nextTier, err := escalateTier(tier)
		if err != nil {
			return "", err
		}
		tier = nextTier
		model = cfg.Models[tier]
	}

	return model, nil
}

func (a *App) resolveModelLegacy(targetRole string, stageLabel string, cfg config.Config) string {
	model, err := a.resolveModel56(targetRole, "", stageLabel, cfg)
	if err != nil {
		return cfg.Model
	}
	return model
}
