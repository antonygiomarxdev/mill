package domain

import "time"

// Session represents a single agent run dispatched by mill.
// Each task may have multiple sessions (e.g. dispatch, review, rework).
type Session struct {
	ID                 string        `json:"id"`
	TaskID             string        `json:"task_id,omitempty"`
	Issue              int           `json:"issue"`
	Status             SessionStatus `json:"status"`
	StartedAt          time.Time     `json:"started_at"`
	EndedAt            time.Time     `json:"ended_at,omitempty"`
	ExitCode           int           `json:"exit_code"`
	Commits            int           `json:"commits"`
	Verdict            Verdict       `json:"verdict,omitempty"`
	Output             string        `json:"output,omitempty"`
	Duration           time.Duration `json:"duration"`
	HeartbeatStaleness time.Duration `json:"heartbeat_staleness"`
	ArtifactPath       string        `json:"artifact_path,omitempty"`
	lastHeartbeat      time.Time     `json:"-"`
}

// NewSession creates a Session with pending status and a unique ID.
func NewSession(id, taskID string, issue int) Session {
	now := time.Now().UTC()
	return Session{
		ID:            generateSessionID(id),
		TaskID:        taskID,
		Issue:         issue,
		Status:        SessionPending,
		StartedAt:     now,
		lastHeartbeat: now,
	}
}

// RegisterHeartbeat records the current time as the most recent heartbeat.
func (s *Session) RegisterHeartbeat() {
	s.lastHeartbeat = time.Now().UTC()
}

// End finalizes the session with a status, exit code, commit count,
// verdict, output text, and artifact path. Sets EndedAt to the current time.
func (s *Session) End(status SessionStatus, exitCode, commits int, verdict Verdict, output, artifactPath string) {
	endedAt := time.Now().UTC()
	s.Status = status
	s.ExitCode = exitCode
	s.Commits = commits
	s.Verdict = verdict
	s.Output = output
	s.ArtifactPath = artifactPath
	s.EndedAt = endedAt
	s.Duration = endedAt.Sub(s.StartedAt)
	s.HeartbeatStaleness = endedAt.Sub(s.lastHeartbeat)
	s.lastHeartbeat = endedAt
}

// generateSessionID prefixes the given id with a unique suffix.
func generateSessionID(id string) string {
	return id + "-" + randomSuffix()
}
