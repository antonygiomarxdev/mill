// Package session models an AI agent session dispatched by mill.
package session

import "fmt"

// Status represents the lifecycle state of a session.
type Status string

const (
	Pending Status = "pending"
	Running Status = "running"
	Done    Status = "done"
	Error   Status = "error"
)

// Verdict represents the outcome of a review session.
type Verdict string

const (
	Approved Verdict = "approved"
	Changes  Verdict = "changes"
	Rejected Verdict = "rejected"
)

// Session represents a single mill task session.
type Session struct {
	ID      string  `json:"id"`
	Issue   int     `json:"issue"`
	Status  Status  `json:"status"`
	Commits int     `json:"commits"`
	Verdict Verdict `json:"verdict,omitempty"`
}

// NewSession creates a new session for the given issue with Pending status.
// The ID is generated as "session-<issue>-<counter>" for uniqueness.
var sessionCounter int

func NewSession(issue int) Session {
	sessionCounter++
	return Session{
		ID:      fmt.Sprintf("session-%d-%d", issue, sessionCounter),
		Issue:   issue,
		Status:  Pending,
		Commits: 0,
	}
}
