package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/antonygiomarxdev/mill/internal/adapter"
)

// maxLogFieldLen bounds stderr and output fields captured in log records.
// The record states whether truncation occurred so nothing is silently dropped.
const maxLogFieldLen = 4096

// multiHandler fans slog records out to multiple underlying handlers.
// It is used to write JSON to a per-issue log file and human-readable text
// to stderr simultaneously.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// slogLevelFromString parses a level string, defaulting to Info.
func slogLevelFromString(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// logLevelFromEnv reads the slog level from the MILL_LOG_LEVEL env var.
// Defaults to slog.LevelInfo when unset or unrecognised.
func logLevelFromEnv() slog.Level {
	return slogLevelFromString(os.Getenv("MILL_LOG_LEVEL"))
}

// errWriter returns the App's error writer, falling back to os.Stderr.
func (a *App) errWriter() io.Writer {
	if a.Err != nil {
		return a.Err
	}
	return os.Stderr
}

// logger returns the App's configured logger, or a discard logger when
// Logger is nil (e.g. in tests that construct App literals without calling
// NewApp).
func (a *App) logger() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// logPath returns the per-issue structured log file path.
func (a *App) logPath(issueNum int) string {
	return fmt.Sprintf("%s/logs/%d.jsonl", a.MillDir, issueNum)
}

// newIssueLogger creates a logger that writes JSON to .mill/logs/<issue>.jsonl
// and human-readable text to the App's stderr writer. The file is opened in
// append mode; the caller is responsible for closing *os.File when done.
func (a *App) newIssueLogger(issueNum int, level slog.Level) (*slog.Logger, *os.File, error) {
	path := a.logPath(issueNum)
	if err := os.MkdirAll(fmt.Sprintf("%s/logs", a.MillDir), 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	opts := &slog.HandlerOptions{Level: level}
	jsonHandler := slog.NewJSONHandler(f, opts)
	textHandler := slog.NewTextHandler(a.errWriter(), opts)

	return slog.New(&multiHandler{handlers: []slog.Handler{jsonHandler, textHandler}}), f, nil
}

// promptHash returns a SHA-256 hex digest of a prompt, used as a compact
// identity for the dispatched instructions so we can distinguish an agent that
// failed from a brief that never arrived.
func promptHash(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(h[:])
}

// truncateLog bounds a string to maxRunes, returning the (possibly truncated)
// text and whether truncation occurred.
func truncateLog(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	return s[:max], true
}

// binaryProvenance returns the version string of the running mill binary,
// recorded once per run so a stale binary is visible in the log.
func binaryProvenance() string {
	return resolveVersion()
}

// logSessionResult records the full SessionResult for a dispatch attempt.
// stderr and output are truncated to maxRunes with a _truncated flag so
// nothing is silently dropped.
func (a *App) logSessionResult(phase, model, role string, round int, result adapter.SessionResult) {
	stderr, stderrTrunc := truncateLog(result.Stderr, maxLogFieldLen)
	output, outputTrunc := truncateLog(result.Output, maxLogFieldLen)

	a.logger().Info("session_result",
		slog.String("phase", phase),
		slog.String("model", model),
		slog.String("role", role),
		slog.Int("round", round),
		slog.Int("exit_code", result.ExitCode),
		slog.Int("commits", result.Commits),
		slog.Duration("heartbeat_staleness", result.HeartbeatStaleness),
		slog.Int("output_length", len(result.Output)),
		slog.String("stderr", stderr),
		slog.Bool("stderr_truncated", stderrTrunc),
		slog.String("output", output),
		slog.Bool("output_truncated", outputTrunc),
	)
}
