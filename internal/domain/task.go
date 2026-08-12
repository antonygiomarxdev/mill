package domain

import "time"

// Task represents a unit of work dispatched to an AI agent.
// A task corresponds to a GitHub issue and tracks its lifecycle
// through dispatch, production, and review.
type Task struct {
	ID           string       `json:"id"`
	Issue        int          `json:"issue"`
	Status       TaskStatus   `json:"status"`
	Phase        TaskPhase    `json:"phase"`
	FailureClass FailureClass `json:"failure_class"`
	Commits      int          `json:"commits"`
	Verdict      Verdict      `json:"verdict,omitempty"`
	AbortReason  string       `json:"abort_reason,omitempty"`
	Round        int          `json:"round"`
	StartedAt    time.Time    `json:"started_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// NewTask creates a Task with running status and current timestamps.
func NewTask(id string, issue int) Task {
	now := time.Now().UTC()
	return Task{
		ID:           id,
		Issue:        issue,
		Status:       TaskRunning,
		Phase:        TaskPhaseDispatch,
		FailureClass: CLASS_OK,
		StartedAt:    now,
		UpdatedAt:    now,
	}
}

// Transition atomically sets the task's phase, status, verdict, commit count,
// failure class, and refreshes the UpdatedAt timestamp.
func (t *Task) Transition(phase TaskPhase, status TaskStatus, verdict Verdict, commits int, failureClass FailureClass) {
	t.Phase = phase
	t.Status = status
	t.Verdict = verdict
	t.Commits = commits
	t.FailureClass = failureClass
	t.UpdatedAt = time.Now().UTC()
}
