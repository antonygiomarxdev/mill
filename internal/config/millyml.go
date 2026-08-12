package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MillYMLConcurrency holds the concurrency section of mill.yml.
type MillYMLConcurrency struct {
	MaxSlots int `yaml:"max-slots"`
}

// RecursionConfig holds the recursion section of mill.yml.
type RecursionConfig struct {
	View     string            `yaml:"view"`      // "result" or "tree"
	Models   map[string]string `yaml:"models"`    // tier -> model name
	MaxDepth int               `yaml:"max_depth"` // default 4
}

// IsZero reports whether no recursion settings are configured.
func (r RecursionConfig) IsZero() bool {
	return r.View == "" && r.MaxDepth == 0 && len(r.Models) == 0
}

// MillYML holds the parsed contents of mill.yml.
type MillYML struct {
	Project     string             `yaml:"project"`
	Provider    string             `yaml:"provider"`
	Concurrency MillYMLConcurrency `yaml:"concurrency"`
	Recursion   RecursionConfig    `yaml:"recursion,omitempty"`
}

// LoadAndValidate reads mill.yml at path, parses it as YAML, and validates
// that required fields are present.
func LoadAndValidate(path string) (*MillYML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MillYML{}, nil
		}
		return nil, fmt.Errorf("mill.yml: read error: %w", err)
	}

	var m MillYML
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("mill.yml: invalid YAML: %w", err)
	}

	if m.Project == "" {
		return nil, fmt.Errorf("mill.yml: missing required field 'project'")
	}

	if m.Provider == "" {
		return nil, fmt.Errorf("mill.yml: missing required field 'provider'")
	}

	return &m, nil
}
