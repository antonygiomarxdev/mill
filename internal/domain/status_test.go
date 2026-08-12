package domain

import "testing"

var allPhases = []TaskPhase{
	TaskPhaseDispatch,
	TaskPhaseProduce,
	TaskPhaseReview,
	TaskPhaseRework,
	TaskPhaseRejected,
	TaskPhaseGateFailed,
	TaskPhaseAborted,
}

func TestTaskPhaseValues(t *testing.T) {
	tests := []struct {
		phase    TaskPhase
		expected string
	}{
		{TaskPhaseDispatch, "dispatch"},
		{TaskPhaseProduce, "produce"},
		{TaskPhaseReview, "review"},
		{TaskPhaseRework, "rework"},
		{TaskPhaseRejected, "rejected"},
		{TaskPhaseGateFailed, "gate_failed"},
		{TaskPhaseAborted, "aborted"},
	}

	for _, tc := range tests {
		if string(tc.phase) != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, string(tc.phase))
		}
	}
}

func TestTaskAbortedStatusValue(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskAborted, "aborted"},
	}

	for _, tc := range tests {
		if string(tc.status) != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, string(tc.status))
		}
	}
}

func TestTaskPhaseCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		current  TaskPhase
		next     TaskPhase
		expected bool
	}{
		// Allowed transitions
		{"dispatch -> produce", TaskPhaseDispatch, TaskPhaseProduce, true},
		{"dispatch -> aborted", TaskPhaseDispatch, TaskPhaseAborted, true},
		{"produce -> review", TaskPhaseProduce, TaskPhaseReview, true},
		{"produce -> rejected", TaskPhaseProduce, TaskPhaseRejected, true},
		{"produce -> gate_failed", TaskPhaseProduce, TaskPhaseGateFailed, true},
		{"produce -> aborted", TaskPhaseProduce, TaskPhaseAborted, true},
		{"review -> rework", TaskPhaseReview, TaskPhaseRework, true},
		{"review -> aborted", TaskPhaseReview, TaskPhaseAborted, true},
		{"rework -> review", TaskPhaseRework, TaskPhaseReview, true},
		{"rework -> aborted", TaskPhaseRework, TaskPhaseAborted, true},
		{"rejected -> produce", TaskPhaseRejected, TaskPhaseProduce, true},
		{"rejected -> aborted", TaskPhaseRejected, TaskPhaseAborted, true},
		{"gate_failed -> rework", TaskPhaseGateFailed, TaskPhaseRework, true},
		{"gate_failed -> aborted", TaskPhaseGateFailed, TaskPhaseAborted, true},

		// Representative disallowed transitions
		{"dispatch -> review", TaskPhaseDispatch, TaskPhaseReview, false},
		{"review -> dispatch", TaskPhaseReview, TaskPhaseDispatch, false},
		{"produce -> dispatch", TaskPhaseProduce, TaskPhaseDispatch, false},
		{"review -> produce", TaskPhaseReview, TaskPhaseProduce, false},
		{"rework -> dispatch", TaskPhaseRework, TaskPhaseDispatch, false},
		{"rejected -> review", TaskPhaseRejected, TaskPhaseReview, false},
		{"gate_failed -> produce", TaskPhaseGateFailed, TaskPhaseProduce, false},
		{"dispatch -> rework", TaskPhaseDispatch, TaskPhaseRework, false},
		{"produce -> rework", TaskPhaseProduce, TaskPhaseRework, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.current.CanTransitionTo(tc.next)
			if got != tc.expected {
				t.Errorf("CanTransitionTo(%q -> %q): expected %v, got %v",
					tc.current, tc.next, tc.expected, got)
			}
		})
	}
}

func TestTaskPhaseAbortedHasNoOutgoingTransitions(t *testing.T) {
	for _, next := range []TaskPhase{
		TaskPhaseDispatch,
		TaskPhaseProduce,
		TaskPhaseReview,
		TaskPhaseRework,
		TaskPhaseRejected,
		TaskPhaseGateFailed,
	} {
		if TaskPhaseAborted.CanTransitionTo(next) {
			t.Errorf("expected no outgoing transition from aborted -> %q", next)
		}
	}
}

func TestTaskPhaseExhaustiveAllowedTransitions(t *testing.T) {
	allowed := map[TaskPhase]map[TaskPhase]bool{
		TaskPhaseDispatch: {
			TaskPhaseProduce: true,
			TaskPhaseAborted: true,
		},
		TaskPhaseProduce: {
			TaskPhaseReview:     true,
			TaskPhaseRejected:   true,
			TaskPhaseGateFailed: true,
			TaskPhaseAborted:    true,
		},
		TaskPhaseReview: {
			TaskPhaseRework:  true,
			TaskPhaseAborted: true,
		},
		TaskPhaseRework: {
			TaskPhaseReview:  true,
			TaskPhaseAborted: true,
		},
		TaskPhaseRejected: {
			TaskPhaseProduce: true,
			TaskPhaseAborted: true,
		},
		TaskPhaseGateFailed: {
			TaskPhaseRework:  true,
			TaskPhaseAborted: true,
		},
		TaskPhaseAborted: {},
	}

	allPhasesSet := make(map[TaskPhase]bool, len(allPhases))
	for _, p := range allPhases {
		allPhasesSet[p] = true
	}

	for src, targets := range allowed {
		for dst := range allPhasesSet {
			expected := targets[dst]
			if got := src.CanTransitionTo(dst); got != expected {
				t.Errorf("CanTransitionTo(%q -> %q): expected %v, got %v",
					src, dst, expected, got)
			}
		}
	}
}
