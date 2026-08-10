// Package state manages persistent task state for mill.
// Task states are persisted to .mill/state.json as domain.Task records.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"

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

// Load reads state from path. If the file does not exist,
// an empty State is returned with no error.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return State{}, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}

	if s.Tasks == nil {
		s.Tasks = make(map[string]domain.Task)
	}

	return s, nil
}

// Save writes the state to path as JSON, creating parent directories.
func (s State) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
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
