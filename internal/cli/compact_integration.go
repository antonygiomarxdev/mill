package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
		return compact.Mode(cfg.Compact.Mode)
	}
	return ""
}

// maybeAutoCompactSession checks config and conditionally compacts the session
// context. Called from the dispatch loop after each produce/review phase.
func (a *App) maybeAutoCompactSession(session adapter.Session, model string, issueNum int, worktree string, cfg config.Config) {
	if cfg.Compact == nil || !cfg.Compact.Enabled {
		return
	}
	mode := compact.Mode(cfg.Compact.Mode)
	if mode == "" {
		mode = compact.ModeFast
	}
	a.compactSession(session, model, issueNum, mode, worktree)
}

func (a *App) compactSession(session adapter.Session, model string, issueNum int, mode compact.Mode, worktree string) {
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

	millDir := filepath.Join(worktree, ".mill")
	if werr := os.MkdirAll(millDir, 0755); werr != nil {
		fmt.Fprintf(a.Err, "warning: failed to create .mill dir: %v\n", werr)
		return
	}
	sessionPath := filepath.Join(millDir, "session.ndjson")
	if werr := os.WriteFile(sessionPath, []byte(compacted), 0644); werr != nil {
		fmt.Fprintf(a.Err, "warning: failed to write compacted session: %v\n", werr)
	}

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
