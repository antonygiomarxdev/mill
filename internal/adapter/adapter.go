// Package adapter defines the provider-agnostic interface for dispatching
// AI agent sessions. Each provider (CommandCode, OpenCode, Claude) implements Adapter.
package adapter

// Capabilities describes what an adapter can do.
type Capabilities struct {
	Models []string `json:"models"`
}

// SessionResult is the outcome of a completed session.
type SessionResult struct {
	ExitCode int    `json:"exit_code"`
	Commits  int    `json:"commits"`
	Verdict  string `json:"verdict"`
}

// Session represents an in-flight or completed agent session.
type Session interface {
	ID() string
	Status() string
	Wait() SessionResult
}

// Adapter dispatches agent sessions for a provider.
// Dispatch starts work on a worktree with a prompt and model.
// Resume reconnects to an existing session by ID.
// Capabilities returns the provider's supported models.
type Adapter interface {
	Dispatch(worktree, prompt, model string) (Session, error)
	Resume(sessionID string) (Session, error)
	Capabilities() Capabilities
}
