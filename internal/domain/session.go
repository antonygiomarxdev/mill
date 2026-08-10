package domain

import "time"

// Session represents a single agent run dispatched by mill.
// Each task may have multiple sessions (e.g. dispatch, review, rework).
type Session struct {
	ID        string         `json:"id"`
	TaskID    string         `json:"task_id,omitempty"`
	Issue     int            `json:"issue"`
	Status    SessionStatus  `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   time.Time      `json:"ended_at,omitempty"`
	ExitCode  int            `json:"exit_code"`
	Commits   int            `json:"commits"`
	Verdict   Verdict        `json:"verdict,omitempty"`
	Output    string         `json:"output,omitempty"`
}

// NewSession creates a Session with pending status and a unique ID.
func NewSession(id, taskID string, issue int) Session {
	return Session{
		ID:        generateSessionID(id),
		TaskID:    taskID,
		Issue:     issue,
		Status:    SessionPending,
		StartedAt: time.Now().UTC(),
	}
}

// End finalizes the session with a status, exit code, commit count,
// verdict, and output text. Sets EndedAt to the current time.
func (s *Session) End(status SessionStatus, exitCode, commits int, verdict Verdict, output string) {
	s.Status = status
	s.ExitCode = exitCode
	s.Commits = commits
	s.Verdict = verdict
	s.Output = output
	s.EndedAt = time.Now().UTC()
}

// generateSessionID prefixes the given id with a unique suffix.
func generateSessionID(id string) string {
	return id + "-" + randomSuffix()
}
