package cli

import (
	"fmt"
	"strings"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/compact"
	"github.com/antonygiomarxdev/mill/internal/config"
)

func resolveCompactMode(configFlag string, cfg config.Config) compact.Mode {
	if configFlag != "" {
		key, value, ok := strings.Cut(configFlag, "=")
		if !ok || key != "compact-mode" {
			return ""
		}
		if value == "fast" {
			return compact.ModeFast
		}
		return ""
	}
	if cfg.Compact != nil && cfg.Compact.Enabled {
		return cfg.Compact.Mode
	}
	return ""
}

func (a *App) compactSession(session adapter.Session, model string, issueNum int, mode compact.Mode) {
	ctx, err := session.ContextText()
	if err != nil || ctx == "" {
		return
	}

	tier := tierForModel(model)
	should, _ := compact.ShouldCompact(ctx, tier)
	if !should {
		return
	}

	compacted, event := compact.Compact(ctx, tier, issueNum)
	event.Trigger = "auto"
	_ = compacted

	if err := compact.WriteLog(event); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to write compaction log: %v\n", err)
	}
}

func tierForModel(model string) string {
	switch {
	case strings.Contains(model, "free"):
		return "free"
	case strings.Contains(model, "pro") || strings.Contains(model, "ultra"):
		return "pro"
	default:
		return "paid"
	}
}

func ExtractCompactConfig(args []string) (configFlag string, remaining []string) {
	return extractFlag(args, "config")
}
