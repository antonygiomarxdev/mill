// Package config manages mill configuration: provider selection, model preference,
// and review round limits. Persisted to .mill/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the mill configuration.
type Config struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	MaxRounds int    `json:"max_rounds"`
}

// Default returns the default mill configuration.
//
// Provider defaults to "commandcode" (CommandCode CLI headless adapter).
// Model defaults to "laguna-free" (the *barato* model for production dispatch).
// MaxRounds defaults to 4 (max review rounds before REJECTED).
func Default() Config {
	return Config{
		Provider:  "commandcode",
		Model:     "laguna-free",
		MaxRounds: 4,
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
