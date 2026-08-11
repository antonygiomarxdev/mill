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

// MillYML holds the parsed contents of mill.yml.
type MillYML struct {
	Project     string              `yaml:"project"`
	Provider    string              `yaml:"provider"`
	Concurrency MillYMLConcurrency  `yaml:"concurrency"`
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
