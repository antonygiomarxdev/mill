package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/antonygiomarxdev/mill/internal/config"
)

type costEntry struct {
	Timestamp    string   `json:"timestamp"`
	Issue        int      `json:"issue"`
	Role         string   `json:"role"`
	Tier         string   `json:"tier"`
	Model        string   `json:"model"`
	Tokens       int      `json:"tokens"`
	CostEstimate *float64 `json:"cost_estimate"`
	Event        string   `json:"event"`
}

func (a *App) costsPath() string {
	return filepath.Join(a.MillDir, "costs.jsonl")
}

func (a *App) logCost(cfg config.Config, issueNum int, role string, tier string, model string, tokens int, event string) {
	entry := costEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Issue:     issueNum,
		Role:      role,
		Tier:      tier,
		Model:     model,
		Tokens:    tokens,
		Event:     event,
	}

	if cfg.Rate > 0 {
		est := float64(tokens) * cfg.Rate
		entry.CostEstimate = &est
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(a.Err, "warning: failed to marshal cost entry: %v\n", err)
		return
	}
	data = append(data, '\n')

	path := a.costsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to create costs dir: %v\n", err)
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(a.Err, "warning: failed to open costs file: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		fmt.Fprintf(a.Err, "warning: failed to write cost entry: %v\n", err)
	}
}
