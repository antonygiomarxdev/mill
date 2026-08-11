package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/compact"
	"github.com/antonygiomarxdev/mill/internal/config"
)

func TestTierForModel(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"laguna-free", "free"},
		{"gpt-5-free", "free"},
		{"deepseek-v4-pro", "pro"},
		{"claude-ultra", "pro"},
		{"gpt-5", "paid"},
		{"claude-sonnet", "paid"},
		{"unknown-model", "paid"},
	}
	for _, tt := range tests {
		got := tierForModel(tt.model)
		if got != tt.want {
			t.Errorf("tierForModel(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestExtractCompactConfig(t *testing.T) {
	configFlag, remaining := ExtractCompactConfig([]string{"--config", "compact-mode=fast", "extra"})
	if configFlag != "compact-mode=fast" {
		t.Errorf("configFlag = %q, want %q", configFlag, "compact-mode=fast")
	}
	if len(remaining) != 1 || remaining[0] != "extra" {
		t.Errorf("remaining = %v, want [extra]", remaining)
	}

	// Without --config flag
	configFlag2, remaining2 := ExtractCompactConfig([]string{"just", "args"})
	if configFlag2 != "" {
		t.Errorf("configFlag = %q, want empty", configFlag2)
	}
	if len(remaining2) != 2 {
		t.Errorf("remaining = %v, want [just args]", remaining2)
	}
}

func TestResolveCompactMode(t *testing.T) {
	tests := []struct {
		name       string
		configFlag string
		cfg        config.Config
		want       compact.Mode
	}{
		{
			name:       "config flag fast",
			configFlag: "compact-mode=fast",
			cfg:        config.Config{},
			want:       compact.ModeFast,
		},
		{
			name:       "config flag unknown value",
			configFlag: "compact-mode=slow",
			cfg:        config.Config{},
			want:       "",
		},
		{
			name:       "no flag, config disabled",
			configFlag: "",
			cfg:        config.Config{},
			want:       "",
		},
		{
			name:       "no flag, config enabled with mode",
			configFlag: "",
			cfg:        config.Config{Compact: &config.CompactConfig{Enabled: true, Mode: "fast"}},
			want:       compact.ModeFast,
		},
		{
			name:       "flag without key",
			configFlag: "other=value",
			cfg:        config.Config{},
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCompactMode(tt.configFlag, tt.cfg)
			if got != tt.want {
				t.Errorf("resolveCompactMode(%q, ...) = %q, want %q", tt.configFlag, got, tt.want)
			}
		})
	}
}

func TestRunCompactNoSession(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runCompact(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No active session to compact") {
		t.Errorf("expected 'No active session to compact', got: %s", buf.String())
	}
}

func TestRunCompactHelp(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runCompact([]string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCompactExtraArgs(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runCompact([]string{"extra-arg"})
	if err == nil {
		t.Fatal("expected error for extra positional args")
	}
	if !strings.Contains(err.Error(), "compact takes no positional arguments") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCompactBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.ndjson")
	// Short session — well below any context window threshold
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.json")
	cfg := config.Config{Provider: "test", Model: "gpt-5"}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runCompact(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "compaction not needed") {
		t.Errorf("expected 'compaction not needed', got: %s", buf.String())
	}
}

func TestRunCompactDryRun(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.ndjson")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.json")
	cfg := config.Config{Provider: "test", Model: "gpt-5-free"}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runCompact([]string{"--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Estimated tokens:") {
		t.Error("expected 'Estimated tokens:' in dry-run output")
	}
	if !strings.Contains(output, "Context window:") {
		t.Error("expected 'Context window:' in dry-run output")
	}
}

func TestPrintDryRun(t *testing.T) {
	buf := new(bytes.Buffer)
	printDryRun(buf, "test context", "free", 100, 64000, 50.0, false)
	output := buf.String()
	if !strings.Contains(output, "Estimated tokens: 100") {
		t.Errorf("expected token estimate, got: %s", output)
	}
	if !strings.Contains(output, "Compaction would trigger: no") {
		t.Errorf("expected 'would trigger: no', got: %s", output)
	}
}

func TestPrintDryRunWouldTrigger(t *testing.T) {
	buf := new(bytes.Buffer)
	printDryRun(buf, "test context", "free", 60000, 64000, 93.75, true)
	output := buf.String()
	if !strings.Contains(output, "Compaction would trigger: yes") {
		t.Errorf("expected 'would trigger: yes', got: %s", output)
	}
	if !strings.Contains(output, "Would save:") {
		t.Errorf("expected 'Would save:', got: %s", output)
	}
}

func TestMaybeAutoCompactSessionDisabled(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	cfg := config.Config{Compact: nil}

	session := &fakeSession{}
	app.maybeAutoCompactSession(session, "gpt-5", 1, dir, cfg)
	// Should be a no-op when compact is disabled
}

func TestMaybeAutoCompactSessionEnabled(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	cfg := config.Config{Compact: &config.CompactConfig{Enabled: true, Mode: "fast"}}

	// Session with empty context — compactSession should return early
	session := &fakeSession{}
	app.maybeAutoCompactSession(session, "gpt-5", 1, dir, cfg)
	// Should not panic
}

func TestCompactSessionEmptyContext(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	session := &fakeSession{}
	app.compactSession(session, "gpt-5", 1, compact.ModeFast, dir)
	// Empty context — should return early
}

func TestCompactSessionWithContext(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	// Create a session with context text above threshold to trigger compaction
	session := &fakeSession{ctxText: strings.Repeat("x", 500000)}
	app.compactSession(session, "laguna-free", 1, compact.ModeFast, dir)
	// Should compact and write session file
}

func TestRunCompactActualCompact(t *testing.T) {
	dir := t.TempDir()

	// Create a large session to trigger compaction
	largeContent := strings.Repeat("x", 500000)
	sessionPath := filepath.Join(dir, "session.ndjson")
	if err := os.WriteFile(sessionPath, []byte(largeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.json")
	cfg := config.Config{Provider: "test", Model: "laguna-free"}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runCompact(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Compacted:") {
		t.Errorf("expected 'Compacted:', got: %s", output)
	}
}
