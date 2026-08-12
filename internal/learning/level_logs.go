// Package learning provides per-level logging and per-role lesson recording
// for the recursive delegation engine.
package learning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// logFileName is the JSONL file where per-level recursion logs are appended.
const logFileName = "recursion.jsonl"

// LevelLog is a single per-level record in the recursion log. Each line of
// recursion.jsonl marshals to one LevelLog.
type LevelLog struct {
	Depth          int                   `json:"depth"`
	Role           string                `json:"role"`
	Model          string                `json:"model"`
	SessionID      string                `json:"session_id"`
	Classification domain.Classification `json:"classification"`
	Duration       time.Duration         `json:"duration"`
	Verdict        domain.Verdict        `json:"verdict"`
}

// LevelLogger writes per-level delegation logs as JSONL to
// <millDir>/logs/recursion.jsonl. The file is opened append-only on every
// write so concurrent writers never clobber each other's records.
type LevelLogger struct {
	path string
}

// NewLevelLogger returns a logger that writes to millDir/logs/recursion.jsonl.
// millDir is the .mill directory (e.g. ".mill" or an absolute equivalent).
func NewLevelLogger(millDir string) *LevelLogger {
	return &LevelLogger{path: filepath.Join(millDir, "logs", logFileName)}
}

// Path returns the log file path. Exposed for tests and callers that need to
// inspect or tail the output.
func (l *LevelLogger) Path() string {
	return l.path
}

// Log appends a single LevelLog entry as one JSON line. The file is created
// (with parent directories) if it does not yet exist.
func (l *LevelLogger) Log(entry LevelLog) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("learning: creating log dir: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("learning: opening log file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("learning: marshalling log entry: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("learning: writing log entry: %w", err)
	}
	return nil
}
