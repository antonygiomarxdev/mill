// Package ledger provides append-only event logging for mill sessions.
// Entries are written as JSON lines to .mill/ledger/<issue>.jsonl.
package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry is a single append-only event in the session ledger.
type Entry struct {
	Timestamp      time.Time `json:"timestamp"`
	Issue          int       `json:"issue"`
	Event          string    `json:"event"`
	Status         string    `json:"status"`
	Verdict        string    `json:"verdict,omitempty"`
	Classification string    `json:"classification,omitempty"`
}

// Append writes a single JSON line entry to the ledger file at path.
// Creates parent directories if they do not exist.
func Append(path string, entry Entry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}
