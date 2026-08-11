// Package config manages mill configuration: provider selection, model preference,
// and review round limits. Persisted to .mill/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/antonygiomarxdev/mill/internal/adapter"
)

// Concurrency controls agent dispatch parallelism.
type Concurrency struct {
	MaxSlots int `json:"max_slots"`
}

// CompactMode is a compaction strategy identifier.
type CompactMode string

const (
	CompactModeFast CompactMode = "fast"
)

// CompactConfig holds the configuration for context compaction.
type CompactConfig struct {
	Enabled bool        `json:"enabled"`
	Mode    CompactMode `json:"mode,omitempty"`
}

// Config holds the mill configuration.
type Config struct {
	Provider             string            `json:"provider"`
	Model                string            `json:"model"`
	Concurrency          Concurrency       `json:"concurrency,omitempty"`
	MaxRounds            int               `json:"max_rounds"`
	MaxRetries           int               `json:"max_retries"`
	Budget               *adapter.Budget   `json:"budget,omitempty"`
	Compact              *CompactConfig    `json:"compact,omitempty"`
	ReviewTimeoutSeconds int               `json:"review_timeout_seconds"`
	Models               map[string]string `json:"models"`
	Rate                 float64           `json:"rate,omitempty"`
}

// Default returns the default mill configuration.
//
func Default() Config {
	return Config{
		Provider:    "commandcode",
		Model:       "laguna-free",
		Concurrency: Concurrency{MaxSlots: 4},
		MaxRounds:   4,
		MaxRetries:  4,
		Compact:     &CompactConfig{Enabled: false, Mode: CompactModeFast},
		Models: map[string]string{
			"free": "laguna-free",
			"paid": "laguna-pro",
			"pro":  "laguna-ultra",
		},
		ReviewTimeoutSeconds: 300,
		Rate:                  0,
	}
}

// Load reads configuration from path. If the file does not exist,
// Default() is returned with no error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Config{}, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}

	return c, nil
}

// Save writes the configuration to path as JSON, creating parent directories.
func (c Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
