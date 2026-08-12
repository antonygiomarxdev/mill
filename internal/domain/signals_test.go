package domain

import (
	"errors"
	"testing"
	"time"
)

func TestSignalResolve(t *testing.T) {
	tests := []struct {
		name     string
		result   SessionResult
		expected FailureClass
	}{
		{
			name:     "connection refused in stderr",
			result:   SessionResult{Stderr: "dial tcp: connection refused"},
			expected: EXECUTION_FAILURE,
		},
		{
			name:     "network timeout in stderr",
			result:   SessionResult{Stderr: "i/o timeout: network timeout"},
			expected: EXECUTION_FAILURE,
		},
		{
			name:     "gate-frd at exit 1",
			result:   SessionResult{ExitCode: 1, Stderr: "gate-frd: validation failed"},
			expected: GATE_FAILURE,
		},
		{
			name:     "changes requested with criterion",
			result:   SessionResult{Stderr: "CHANGES_REQUESTED: [criterion: tests]"},
			expected: RESULT_FAILURE,
		},
		{
			name:     "exit -1 with blocked",
			result:   SessionResult{ExitCode: -1, Stderr: "blocked: waiting on dependency"},
			expected: EXECUTION_FAILURE,
		},
		{
			name:     "exit 9",
			result:   SessionResult{ExitCode: 9},
			expected: EXECUTION_FAILURE,
		},
		{
			name:     "exit 0 placeholder output",
			result:   SessionResult{ExitCode: 0, Output: "TODO: implement this"},
			expected: CONTRACT_FAILURE,
		},
		{
			name:     "heartbeat stale while active",
			result:   SessionResult{ProcessActive: true, HeartbeatStaleness: 60 * time.Second},
			expected: EXECUTION_FAILURE,
		},
		{
			name:     "env error",
			result:   SessionResult{EnvError: errors.New("missing env var")},
			expected: ENVIRONMENT_FAILURE,
		},
	}

	reg := NewSignalRegistry()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reg.Resolve(tc.result)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestSignalResolvePriorityDeterminism(t *testing.T) {
	reg := NewSignalRegistry()

	result := SessionResult{
		ExitCode: 0,
		Output:   "TODO placeholder",
		Stderr:   "CHANGES_REQUESTED: [criterion: tests]",
	}

	got := reg.Resolve(result)

	if got != RESULT_FAILURE {
		t.Errorf("expected %q, got %q", RESULT_FAILURE, got)
	}
}

func TestSignalResolveCleanResult(t *testing.T) {
	reg := NewSignalRegistry()

	result := SessionResult{ExitCode: 0}

	got := reg.Resolve(result)

	if got != CLASS_OK {
		t.Errorf("expected %q, got %q", CLASS_OK, got)
	}
}
