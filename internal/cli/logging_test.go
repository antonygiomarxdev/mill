package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/adapter"
)

func TestLogPath(t *testing.T) {
	app := &App{MillDir: "/tmp/milltest"}
	got := app.logPath(42)
	if got != "/tmp/milltest/logs/42.jsonl" {
		t.Errorf("logPath(42) = %q, want %q", got, "/tmp/milltest/logs/42.jsonl")
	}
}

func TestSlogLevelFromString(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"garbage", slog.LevelInfo},
		{"", slog.LevelInfo},
	}
	for _, tt := range tests {
		if got := slogLevelFromString(tt.input); got != tt.want {
			t.Errorf("slogLevelFromString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLogLevelFromEnv(t *testing.T) {
	t.Setenv("MILL_LOG_LEVEL", "debug")
	if got := logLevelFromEnv(); got != slog.LevelDebug {
		t.Errorf("with env debug, got %v, want %v", got, slog.LevelDebug)
	}

	t.Setenv("MILL_LOG_LEVEL", "")
	if got := logLevelFromEnv(); got != slog.LevelInfo {
		t.Errorf("with env empty, got %v, want %v", got, slog.LevelInfo)
	}

	t.Setenv("MILL_LOG_LEVEL", "error")
	if got := logLevelFromEnv(); got != slog.LevelError {
		t.Errorf("with env error, got %v, want %v", got, slog.LevelError)
	}
}

func TestPromptHashDeterministic(t *testing.T) {
	prompt := "You are a helpful assistant. Work on issue #42."
	h1 := promptHash(prompt)
	h2 := promptHash(prompt)
	if h1 != h2 {
		t.Fatal("promptHash is not deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d chars: %s", len(h1), h1)
	}
	if h1 == "" {
		t.Error("expected non-empty hash")
	}
}

func TestPromptHashDifferent(t *testing.T) {
	h1 := promptHash("prompt A")
	h2 := promptHash("prompt B")
	if h1 == h2 {
		t.Error("different prompts should produce different hashes")
	}
}

func TestTruncateLogNoTruncation(t *testing.T) {
	got, truncated := truncateLog("short", 10)
	if got != "short" {
		t.Errorf("expected 'short', got %q", got)
	}
	if truncated {
		t.Error("expected truncated=false")
	}
}

func TestTruncateLogTruncation(t *testing.T) {
	long := strings.Repeat("x", 20)
	got, truncated := truncateLog(long, 10)
	if got != long[:10] {
		t.Errorf("expected first 10 chars, got %q", got)
	}
	if !truncated {
		t.Error("expected truncated=true")
	}
}

func TestTruncateLogEmpty(t *testing.T) {
	got, truncated := truncateLog("", 10)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if truncated {
		t.Error("expected truncated=false for empty string")
	}
}

func TestBinaryProvenance(t *testing.T) {
	bp := binaryProvenance()
	if bp == "" {
		t.Error("expected non-empty binary provenance")
	}
}

func TestNewIssueLoggerCreatesFile(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	logger, f, err := app.newIssueLogger(42, slog.LevelInfo)
	if err != nil {
		t.Fatalf("newIssueLogger failed: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if f == nil {
		t.Fatal("expected non-nil file handle")
	}
	f.Close()

	// Verify the file was created
	logFilePath := app.logPath(42)
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Fatal("expected log file to exist")
	}
}

func TestNewIssueLoggerWritesJSON(t *testing.T) {
	dir := t.TempDir()
	var stderrBuf bytes.Buffer
	app := &App{MillDir: dir, Err: &stderrBuf}

	logger, f, err := app.newIssueLogger(42, slog.LevelDebug)
	if err != nil {
		t.Fatalf("newIssueLogger failed: %v", err)
	}

	logger.Info("test message",
		slog.String("key", "value"),
		slog.Int("num", 42),
	)
	f.Close()

	// Verify JSON in the file
	data, err := os.ReadFile(app.logPath(42))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\n%s", err, lines[0])
	}
	if entry["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got %v", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got %v", entry["level"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key 'value', got %v", entry["key"])
	}

	// Verify text appeared in stderr
	if !strings.Contains(stderrBuf.String(), "test message") {
		t.Error("expected text output in stderr")
	}
}

func TestNewIssueLoggerAppends(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir, Err: new(bytes.Buffer)}

	logger1, f1, err := app.newIssueLogger(7, slog.LevelInfo)
	if err != nil {
		t.Fatalf("newIssueLogger failed: %v", err)
	}
	logger1.Info("first")
	f1.Close()

	logger2, f2, err := app.newIssueLogger(7, slog.LevelInfo)
	if err != nil {
		t.Fatalf("newIssueLogger failed: %v", err)
	}
	logger2.Info("second")
	f2.Close()

	data, _ := os.ReadFile(app.logPath(7))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}
}

func TestMultiHandlerFansOut(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, nil)
	h2 := slog.NewTextHandler(&buf2, nil)
	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}
	logger := slog.New(mh)

	logger.Info("fan test", slog.String("k", "v"))

	if !strings.Contains(buf1.String(), "fan test") {
		t.Error("first handler did not receive record")
	}
	if !strings.Contains(buf2.String(), "fan test") {
		t.Error("second handler did not receive record")
	}
}

func TestMultiHandlerWithAttrs(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewJSONHandler(&buf1, nil)
	h2 := slog.NewJSONHandler(&buf2, nil)
	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}
	logger := slog.New(mh)

	logger = logger.With(slog.String("request_id", "abc"))
	logger.Info("with attrs", slog.String("key", "value"))

	for i, buf := range []bytes.Buffer{buf1, buf2} {
		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("handler %d: invalid JSON: %v", i, err)
		}
		if entry["request_id"] != "abc" {
			t.Errorf("handler %d: expected request_id 'abc', got %v", i, entry["request_id"])
		}
		if entry["key"] != "value" {
			t.Errorf("handler %d: expected key 'value', got %v", i, entry["key"])
		}
	}
}

func TestMultiHandlerEnabled(t *testing.T) {
	h := &multiHandler{handlers: []slog.Handler{
		slog.NewTextHandler(io.Discard, nil),
	}}
	if !h.Enabled(nil, slog.LevelInfo) {
		t.Error("expected Enabled to return true for Info level")
	}
}

func TestLoggerNilSafe(t *testing.T) {
	app := &App{MillDir: t.TempDir()}
	// Logger is nil — logger() should return a discard logger, not panic
	l := app.logger()
	if l == nil {
		t.Fatal("expected non-nil logger from nil Logger field")
	}
	// Should not panic
	l.Info("test", slog.String("key", "val"))
}

func TestLogSessionResult(t *testing.T) {
	dir := t.TempDir()
	var stderrBuf bytes.Buffer
	app := &App{MillDir: dir, Err: &stderrBuf}

	logger, f, err := app.newIssueLogger(55, slog.LevelInfo)
	if err != nil {
		t.Fatalf("newIssueLogger failed: %v", err)
	}
	app.Logger = logger
	defer f.Close()

	result := adapter.SessionResult{
		ExitCode:           137,
		Commits:            0,
		Stderr:             "blocked: time budget exceeded",
		Output:             "",
		HeartbeatStaleness: 0,
	}
	app.logSessionResult("produce", "deepseek-v4-pro", "sr-dev-be", 0, result)

	data, _ := os.ReadFile(app.logPath(55))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, lines[0])
	}
	if entry["phase"] != "produce" {
		t.Errorf("expected phase 'produce', got %v", entry["phase"])
	}
	if entry["model"] != "deepseek-v4-pro" {
		t.Errorf("expected model 'deepseek-v4-pro', got %v", entry["model"])
	}
	if entry["role"] != "sr-dev-be" {
		t.Errorf("expected role 'sr-dev-be', got %v", entry["role"])
	}
	if entry["exit_code"].(float64) != 137 {
		t.Errorf("expected exit_code 137, got %v", entry["exit_code"])
	}
	if entry["commits"].(float64) != 0 {
		t.Errorf("expected commits 0, got %v", entry["commits"])
	}
	if entry["stderr"] != "blocked: time budget exceeded" {
		t.Errorf("expected stderr, got %v", entry["stderr"])
	}
	if entry["stderr_truncated"].(bool) != false {
		t.Error("expected stderr_truncated=false")
	}
	if entry["output_length"].(float64) != 0 {
		t.Errorf("expected output_length 0, got %v", entry["output_length"])
	}
}

func TestLogSessionResultTruncatesStderr(t *testing.T) {
	dir := t.TempDir()
	var stderrBuf bytes.Buffer
	app := &App{MillDir: dir, Err: &stderrBuf}

	logger, f, err := app.newIssueLogger(55, slog.LevelInfo)
	if err != nil {
		t.Fatalf("newIssueLogger failed: %v", err)
	}
	app.Logger = logger
	defer f.Close()

	longStderr := strings.Repeat("x", maxLogFieldLen+100)
	result := adapter.SessionResult{
		ExitCode: 1,
		Stderr:   longStderr,
	}
	app.logSessionResult("produce", "gpt-5", "staff", 0, result)

	data, _ := os.ReadFile(app.logPath(55))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["stderr_truncated"].(bool) != true {
		t.Error("expected stderr_truncated=true")
	}
	// output_length reflects result.Output, which is empty here
	if entry["output_length"].(float64) != 0 {
		t.Errorf("expected output_length 0, got %v", entry["output_length"])
	}
}

func TestLogPathMethod(t *testing.T) {
	app := &App{MillDir: filepath.Join(os.TempDir(), "milltest")}
	want := filepath.Join(os.TempDir(), "milltest", "logs", "155.jsonl")
	if got := app.logPath(155); got != want {
		t.Errorf("logPath(155) = %q, want %q", got, want)
	}
}
