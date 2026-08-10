package cli

import (
	"fmt"
	"path/filepath"
)

// millDir is the directory for mill's persisted state.
// It is a var (not const) so tests can override it with a temp directory.
var millDir = ".mill"

// statePath returns the path to the state file.
func statePath() string {
	return filepath.Join(millDir, "state.json")
}

// ledgerPath returns the path to the ledger file for the given issue.
func ledgerPath(issue int) string {
	return filepath.Join(millDir, "ledger", fmt.Sprintf("%d.jsonl", issue))
}
