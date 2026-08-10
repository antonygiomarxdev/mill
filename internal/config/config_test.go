package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/adapter"
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

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadReadErrorNotNotExist(t *testing.T) {
	dir := t.TempDir()
	// Reading a directory returns an error that is NOT os.IsNotExist.
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when loading a directory path, got nil")
	}
}

func TestSaveMkdirError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file that blocks directory creation at that path.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("blocking"), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	path := filepath.Join(blocker, "config.json")
	c := Default()
	if err := c.Save(path); err == nil {
		t.Fatal("expected error when MkdirAll fails on a file path, got nil")
	}
}

func TestLoadPartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Only provider set; model and max_rounds should default to zero values.
	partial := `{"provider": "openai"}`
	if err := os.WriteFile(path, []byte(partial), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if c.Provider != "openai" {
		t.Errorf("expected provider %q, got %q", "openai", c.Provider)
	}
	if c.Model != "" {
		t.Errorf("expected empty model, got %q", c.Model)
	}
	if c.MaxRounds != 0 {
		t.Errorf("expected max_rounds 0, got %d", c.MaxRounds)
	}
}

func TestLoadUnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	raw := `{"provider": "openai", "unknown_field": 42, "extra": "data"}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if c.Provider != "openai" {
		t.Errorf("expected provider %q, got %q", "openai", c.Provider)
	}
}

func TestSaveEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	var c Config
	if err := c.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Provider != "" || loaded.Model != "" || loaded.MaxRounds != 0 {
		t.Errorf("expected zero-value config, got %+v", loaded)
	}
}

func TestConfigJSONRoundTripCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "subdir", "config.json")

	original := Config{
		Provider:  "anthropic",
		Model:     "claude-opus",
		MaxRounds: 10,
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
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

func TestBudgetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	tb := 1000000
	original := Config{
		Provider:  "commandcode",
		Model:     "laguna-free",
		MaxRounds: 4,
		Budget: &adapter.Budget{
			TimeSeconds: 300,
			MaxTurns:    20,
			TokenBudget: &tb,
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Budget == nil {
		t.Fatal("expected budget to be loaded, got nil")
	}
	if loaded.Budget.TimeSeconds != 300 {
		t.Errorf("expected time_seconds 300, got %d", loaded.Budget.TimeSeconds)
	}
	if loaded.Budget.MaxTurns != 20 {
		t.Errorf("expected max_turns 20, got %d", loaded.Budget.MaxTurns)
	}
	if loaded.Budget.TokenBudget == nil {
		t.Error("expected token_budget to be set, got nil")
	} else if *loaded.Budget.TokenBudget != 1000000 {
		t.Errorf("expected token_budget 1000000, got %d", *loaded.Budget.TokenBudget)
	}
}

func TestBudgetJSONStructure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	tb := 500000
	c := Config{
		Budget: &adapter.Budget{
			TimeSeconds: 60,
			MaxTurns:    10,
			TokenBudget: &tb,
		},
	}
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

	if _, ok := fields["budget"]; !ok {
		t.Error("expected budget field in config JSON")
	}

	var budget map[string]json.RawMessage
	if err := json.Unmarshal(fields["budget"], &budget); err != nil {
		t.Fatalf("unmarshal budget failed: %v", err)
	}

	expectedBudget := []string{"time_seconds", "max_turns", "token_budget"}
	for _, f := range expectedBudget {
		if _, ok := budget[f]; !ok {
			t.Errorf("expected budget field %q", f)
		}
	}
}

func TestBudgetOptionalTokenBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Config{
		Budget: &adapter.Budget{
			TimeSeconds: 120,
			MaxTurns:    15,
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Budget == nil {
		t.Fatal("expected budget to be loaded, got nil")
	}
	if loaded.Budget.TokenBudget != nil {
		t.Errorf("expected token_budget to be nil (omitted), got %d", *loaded.Budget.TokenBudget)
	}
}
