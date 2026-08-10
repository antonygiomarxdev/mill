package domain

import "time"

// Task represents a unit of work dispatched to an AI agent.
// A task corresponds to a GitHub issue and tracks its lifecycle
// through dispatch, production, and review.
type Task struct {
	ID        string     `json:"id"`
	Issue     int        `json:"issue"`
	Status    TaskStatus `json:"status"`
	Commits   int        `json:"commits"`
	Verdict   Verdict    `json:"verdict,omitempty"`
	Round     int        `json:"round"`
	StartedAt time.Time  `json:"started_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NewTask creates a Task with running status and current timestamps.
func NewTask(id string, issue int) Task {
	now := time.Now().UTC()
	return Task{
		ID:        id,
		Issue:     issue,
		Status:    TaskRunning,
		StartedAt: now,
		UpdatedAt: now,
	}
}

// UpdateStatus sets the task's status, verdict, and commit count,
// refreshing the UpdatedAt timestamp.
func (t *Task) UpdateStatus(status TaskStatus, verdict Verdict, commits int) {
	t.Status = status
	t.Commits = commits
	t.Verdict = verdict
	t.UpdatedAt = time.Now().UTC()
}
