package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()

	if c.Provider != "commandcode" {
		t.Errorf("expected provider %q, got %q", "commandcode", c.Provider)
	}
	if c.Model != "laguna-free" {
		t.Errorf("expected model %q, got %q", "laguna-free", c.Model)
	}
	if c.MaxRounds != 4 {
		t.Errorf("expected max_rounds %d, got %d", 4, c.MaxRounds)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Default()
	original.Model = "deepseek-v4-flash"
	original.MaxRounds = 6

	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected config.json to be created")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Provider != original.Provider {
		t.Errorf("provider mismatch: %q vs %q", loaded.Provider, original.Provider)
	}
	if loaded.Model != original.Model {
		t.Errorf("model mismatch: %q vs %q", loaded.Model, original.Model)
	}
	if loaded.MaxRounds != original.MaxRounds {
		t.Errorf("max_rounds mismatch: %d vs %d", loaded.MaxRounds, original.MaxRounds)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}

	def := Default()
	if loaded.Provider != def.Provider {
		t.Errorf("expected default provider %q, got %q", def.Provider, loaded.Provider)
	}
}

func TestSaveCreatesMillDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "config.json")

	c := Default()
	if err := c.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected .mill/config.json to be created")
	}
}

func TestConfigJSONStructure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	c := Default()
	if err := c.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expectedFields := []string{"provider", "model", "max_rounds"}
	for _, f := range expectedFields {
		if _, ok := fields[f]; !ok {
			t.Errorf("expected field %q in config JSON", f)
		}
	}
}
