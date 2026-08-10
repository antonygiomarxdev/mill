package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultModelsAndRate(t *testing.T) {
	c := Default()

	if c.Models == nil {
		t.Fatal("expected Models to be non-nil")
	}
	if c.Models["free"] != "laguna-free" {
		t.Errorf("expected Models[free]=laguna-free, got %q", c.Models["free"])
	}
	if c.Models["paid"] != "laguna-pro" {
		t.Errorf("expected Models[paid]=laguna-pro, got %q", c.Models["paid"])
	}
	if c.Models["pro"] != "laguna-ultra" {
		t.Errorf("expected Models[pro]=laguna-ultra, got %q", c.Models["pro"])
	}
	if c.Rate != 0 {
		t.Errorf("expected Rate 0, got %f", c.Rate)
	}
}

func TestConfigModelsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Config{
		Provider:  "commandcode",
		Model:     "laguna-free",
		MaxRounds: 4,
		Models: map[string]string{
			"free": "custom-free",
			"paid": "custom-paid",
			"pro":  "custom-pro",
		},
		Rate: 0.00001,
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Models["free"] != "custom-free" {
		t.Errorf("Models[free] mismatch: got %q, want %q", loaded.Models["free"], "custom-free")
	}
	if loaded.Models["paid"] != "custom-paid" {
		t.Errorf("Models[paid] mismatch: got %q, want %q", loaded.Models["paid"], "custom-paid")
	}
	if loaded.Models["pro"] != "custom-pro" {
		t.Errorf("Models[pro] mismatch: got %q, want %q", loaded.Models["pro"], "custom-pro")
	}
	if loaded.Rate != 0.00001 {
		t.Errorf("Rate mismatch: got %f, want %f", loaded.Rate, 0.00001)
	}
}

func TestConfigModelsBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	oldJSON := `{"provider": "openai", "model": "gpt-5", "max_rounds": 3}`
	if err := os.WriteFile(path, []byte(oldJSON), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if c.Rate != 0 {
		t.Errorf("expected Rate 0 for backward compat, got %f", c.Rate)
	}
	if c.Models != nil {
		t.Errorf("expected Models nil for backward compat, got %v", c.Models)
	}
}
