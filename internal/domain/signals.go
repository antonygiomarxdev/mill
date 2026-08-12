package domain

import (
	"strings"
	"time"
)

// SessionResult captures the observable outcome of a session run, used to
// determine its failure classification.
type SessionResult struct {
	ExitCode           int
	Stderr             string
	Output             string
	HeartbeatStaleness time.Duration
	ProcessActive      bool
	EnvError           error
}

// Signal is a single declarative failure rule: a predicate over a SessionResult
// paired with the FailureClass it maps to and a human-readable description.
type Signal struct {
	Predicate    func(SessionResult) bool
	FailureClass FailureClass
	Description  string
}

// SignalRegistry holds an ordered list of Signals. Resolve iterates the table
// in order and returns the FailureClass of the first matching Signal.
type SignalRegistry struct {
	signals []Signal
}

// NewSignalRegistry returns a registry initialised with the default
// priority-ordered signal table.
func NewSignalRegistry() *SignalRegistry {
	return &SignalRegistry{signals: defaultSignals()}
}

// Signals returns the ordered list of signals in this registry.
func (r *SignalRegistry) Signals() []Signal {
	return r.signals
}

// Resolve returns the FailureClass of the first matching Signal, or CLASS_OK
// when none match.
func (r *SignalRegistry) Resolve(result SessionResult) FailureClass {
	for _, s := range r.signals {
		if s.Predicate(result) {
			return s.FailureClass
		}
	}
	return CLASS_OK
}

// HeartbeatStaleThreshold is the duration after which a still-active process
// with no recent heartbeat is considered stale.
var HeartbeatStaleThreshold = 30 * time.Second

// defaultSignals returns the priority-ordered failure-signal table.
//
// Order reflects priority: stderr-derived signals first, then exit-code-based
// signals, then the heartbeat guard, and finally the environment guard.
func defaultSignals() []Signal {
	return []Signal{
		{
			Predicate: func(r SessionResult) bool {
				lower := strings.ToLower(r.Stderr)
				return strings.Contains(lower, "connection refused") ||
					strings.Contains(lower, "network timeout")
			},
			FailureClass: EXECUTION_FAILURE,
			Description:  "stderr indicates connection refused or network timeout",
		},
		{
			Predicate: func(r SessionResult) bool {
				if r.ExitCode != 1 {
					return false
				}
				lower := strings.ToLower(r.Stderr)
				return strings.Contains(lower, "gate-frd") ||
					strings.Contains(lower, "gate-spec") ||
					strings.Contains(lower, "gate-tasks")
			},
			FailureClass: GATE_FAILURE,
			Description:  "exit code 1 with a gate failure (gate-frd/gate-spec/gate-tasks) in stderr",
		},
		{
			Predicate: func(r SessionResult) bool {
				lower := strings.ToLower(r.Stderr)
				return strings.Contains(lower, "changes_requested:") &&
					strings.Contains(lower, "criterion")
			},
			FailureClass: RESULT_FAILURE,
			Description:  "stderr indicates changes requested with a criterion",
		},
		{
			Predicate: func(r SessionResult) bool {
				if r.ExitCode != -1 && r.ExitCode != -2 {
					return false
				}
				return strings.Contains(strings.ToLower(r.Stderr), "blocked:")
			},
			FailureClass: EXECUTION_FAILURE,
			Description:  "exit code -1 or -2 with a blocked signal in stderr",
		},
		{
			Predicate: func(r SessionResult) bool {
				switch r.ExitCode {
				case 4, 9, 130, 137, 143:
					return true
				}
				return false
			},
			FailureClass: EXECUTION_FAILURE,
			Description:  "exit code indicates process killed or aborted (4, 9, 130, 137, 143)",
		},
		{
			Predicate: func(r SessionResult) bool {
				if r.ExitCode != 0 {
					return false
				}
				if r.Output != "" && strings.TrimSpace(r.Output) == "" {
					return true
				}
				lower := strings.ToLower(r.Output)
				return strings.Contains(lower, "todo") ||
					strings.Contains(lower, "tbd") ||
					strings.Contains(lower, "placeholder")
			},
			FailureClass: CONTRACT_FAILURE,
			Description:  "exit code 0 but output is a placeholder (whitespace-only or contains todo/tbd/placeholder)",
		},
		{
			Predicate: func(r SessionResult) bool {
				return r.ProcessActive && r.HeartbeatStaleness > HeartbeatStaleThreshold
			},
			FailureClass: EXECUTION_FAILURE,
			Description:  "process still active but heartbeat is stale",
		},
		{
			Predicate: func(r SessionResult) bool {
				return r.EnvError != nil
			},
			FailureClass: ENVIRONMENT_FAILURE,
			Description:  "environment error during session",
		},
	}
}
