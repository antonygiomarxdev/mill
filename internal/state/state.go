// Package state manages persistent task state for mill.
// Task states are persisted to .mill/state.json as domain.Task records.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// State holds all current task states, keyed by task ID.
type State struct {
	Tasks map[string]domain.Task `json:"tasks"`
}

// New returns an empty State ready for use.
func New() State {
	return State{
		Tasks: make(map[string]domain.Task),
	}
}

// Load reads state from path with backup fallback. If the primary file parses,
// it is returned directly. If the primary is missing (os.IsNotExist), Load
// falls back to state.json.1, then state.json.2, so a crash in the
// rotate->rename window remains recoverable. If the primary is corrupt, the
// same backup fallback is attempted. An empty State is returned only when no
// primary file exists and no valid (or corrupt) backup is present. An error is
// returned when a corrupt primary or backup exists and every parse attempt
// fails.
func Load(path string) (State, error) {
	// Try primary first.
	s, err := parseStateFile(path)
	if err == nil {
		return s, nil
	}

	// Primary failed (missing or corrupt); fall back to backups in order.
	primaryMissing := os.IsNotExist(err)
	hadCorrupt := false

	for _, n := range []int{1, 2} {
		bp := backupPath(path, n)
		bs, bkErr := parseStateFile(bp)
		if bkErr == nil {
			return bs, nil
		}
		if !os.IsNotExist(bkErr) {
			// Backup exists but is corrupt — remember corruption occurred.
			hadCorrupt = true
		}
	}

	// No valid file was found. If the primary was simply missing and no corrupt
	// backup existed, treat this as a fresh state; otherwise a corrupt
	// primary/backup exists and all parse attempts failed.
	if primaryMissing && !hadCorrupt {
		return New(), nil
	}

	return State{}, fmt.Errorf("state: cannot load %s (corrupt, no valid backup): %w", path, err)
}

// parseStateFile reads and unmarshals a state file, returning the parsed State.
func parseStateFile(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	return parseState(data)
}

// parseState unmarshals JSON data into a State, initializing Tasks map if nil.
func parseState(data []byte) (State, error) {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	if s.Tasks == nil {
		s.Tasks = make(map[string]domain.Task)
	}
	return s, nil
}

// backupPath returns the backup path for a given suffix: 1 → state.json.1, 2 → state.json.2.
func backupPath(path string, n int) string {
	return path + "." + strconv.Itoa(n)
}

// rotateBackups shifts the backup chain: .2 removed, .1 → .2, primary → .1.
// Errors are logged but not propagated — backup rotation is best-effort.
func rotateBackups(path string) {
	_ = os.Remove(backupPath(path, 2))
	_ = os.Rename(backupPath(path, 1), backupPath(path, 2))
	_ = os.Rename(path, backupPath(path, 1))
}

// Save writes the state to path atomically: marshal to temp file, fsync,
// rotate backups, then atomic rename onto the target path.
func (s State) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	rotateBackups(path)

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}

// UpsertTask inserts a new task or updates an existing one by ID.
func (s *State) UpsertTask(t domain.Task) {
	if s.Tasks == nil {
		s.Tasks = make(map[string]domain.Task)
	}
	s.Tasks[t.ID] = t
}

// Task looks up a task by ID.
func (s State) Task(id string) (domain.Task, bool) {
	t, ok := s.Tasks[id]
	return t, ok
}
