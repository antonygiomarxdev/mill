// Package ledger provides append-only event logging for mill sessions.
// Entries are written as JSON lines to .mill/ledger/<issue>.jsonl.
package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

type Entry struct {
	Timestamp      time.Time `json:"timestamp"`
	Issue          int       `json:"issue"`
	Event          string    `json:"event"`
	Status         string    `json:"status"`
	Verdict        string    `json:"verdict,omitempty"`
	Classification string    `json:"classification,omitempty"`
	Round          int       `json:"round"`
	// File is the worktree-relative path for file_read/file_write events.
	File string `json:"file,omitempty"`
	// Version is the per-file monotonic counter for file_read/file_write events.
	Version int `json:"version,omitempty"`
	// AgentID is the dispatch phase ("produce" or "review") for file events.
	AgentID string `json:"agent_id,omitempty"`
	// FailureClass is a coarse-grained failure bucket for task lifecycle events.
	FailureClass domain.FailureClass `json:"failure_class,omitempty"`
	// Phase is the task lifecycle phase associated with the event.
	Phase domain.TaskPhase `json:"phase,omitempty"`
	// Role is the mill role (e.g. "sr-dev-be") associated with the event.
	Role string `json:"role,omitempty"`
	// ParentIssue is the parent issue number this entry is nested under.
	ParentIssue int `json:"parent_issue,omitempty"`
	// Depth is the recursion depth of the task producing this entry.
	Depth int `json:"depth,omitempty"`
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

// ReadEntries reads and parses all JSONL entries from a ledger file.
// Malformed lines are skipped with a warning to stderr.
// If the file does not exist, returns an empty slice with no error.
func ReadEntries(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ledger: cannot read version data from %s: %w — conflict detection disabled", path, err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			fmt.Fprintf(os.Stderr, "ledger: skipping malformed line %d in %s: %v\n", lineNum, path, err)
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("ledger: error reading %s: %w", path, err)
	}
	return entries, nil
}
