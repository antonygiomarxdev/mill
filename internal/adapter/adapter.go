// Package adapter defines the provider-agnostic interface for dispatching
// AI agent sessions. Each provider (CommandCode, OpenCode, Claude) implements Adapter.
package adapter

import "github.com/antonygiomarxdev/mill/internal/domain"

// Budget holds per-target resource constraints for agent delegation.
type Budget struct {
	TimeSeconds int  `json:"time_seconds"`
	MaxTurns    int  `json:"max_turns"`
	TokenBudget *int `json:"token_budget,omitempty"`
}

// DispatchOpts contains the options for dispatching a new agent session.
type DispatchOpts struct {
	// Worktree is the directory where the agent should operate.
	Worktree string
	// Prompt is the query passed to the agent via the CLI.
	Prompt string
	// Model is the provider model identifier (e.g. "laguna-free").
	Model string
	// MaxTurns caps the conversation turns. Zero means no cap.
	MaxTurns int
	// Budget is the per-target resource budget (nil = unbounded).
	Budget *Budget
}

// ReadToolCapabilities describes the forwarder harness's read tool.
type ReadToolCapabilities struct {
	// LineCeiling is the maximum number of lines the read tool returns
	// in a single call. 0 means unlimited.
	LineCeiling int `json:"line_ceiling"`

	// ByteCeiling is the maximum total bytes the read tool returns
	// in a single call. 0 means unlimited.
	ByteCeiling int `json:"byte_ceiling"`

	// CharCeiling is the maximum characters per displayed line before
	// the tool truncates a line mid-display. 0 means unlimited.
	CharCeiling int `json:"char_ceiling"`

	// HasSelectorSupport is true when the read tool accepts line-range
	// selectors (e.g. :50-200, :raw, :50+150, :conflicts).
	HasSelectorSupport bool `json:"has_selector_support"`

	// HasRecoveryNotes is true when the read tool emits truncation
	// indicators (e.g. "[TRUNCATED: 1200 lines omitted]") instead of
	// silently dropping content.
	HasRecoveryNotes bool `json:"has_recovery_notes"`
}

// Capabilities describes what an adapter can do.
type Capabilities struct {
	Models   []string            `json:"models"`
	ReadTool ReadToolCapabilities `json:"read_tool"`
}

// SessionResult is the outcome of a completed session.
type SessionResult struct {
	ExitCode int    `json:"exit_code"`
	Commits  int    `json:"commits"`
	Output   string `json:"output"`
	Stderr   string `json:"stderr"`
}

// Session represents an in-flight or completed agent session.
type Session interface {
	ID() string
	Status() string
	Wait() (SessionResult, error)
	// ContextText returns the full NDJSON session context for compaction.
	ContextText() (string, error)
}

// Adapter dispatches agent sessions for a provider.
// Dispatch starts work on a worktree with a prompt and model.
// Resume reconnects to an existing session by ID.
// Capabilities returns the provider's supported models.
type Adapter interface {
	Dispatch(opts DispatchOpts) (Session, error)
	Resume(sessionID string) (Session, error)
	Capabilities() Capabilities
}

// sessionStatus returns the domain session status as a string.
func sessionStatus(s domain.SessionStatus) string {
	return string(s)
}
