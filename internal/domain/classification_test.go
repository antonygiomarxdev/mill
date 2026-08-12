package domain

import "testing"

func TestClassificationValues(t *testing.T) {
	tests := []struct {
		classification Classification
		expected       string
	}{
		{ClassificationOK, "OK"},
		{ClassificationFatal, "FATAL"},
		{ClassificationMaxTurns, "MAX_TURNS"},
		{ClassificationAuth, "AUTH"},
		{ClassificationNoCredit, "NO_CREDIT"},
		{ClassificationRateLimited, "RATE_LIMITED"},
		{ClassificationTransient, "TRANSIENT"},
		{ClassificationBlocked, "BLOCKED"},
	}

	for _, tc := range tests {
		if string(tc.classification) != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, string(tc.classification))
		}
	}
}

func TestFailureClassOf(t *testing.T) {
	tests := []struct {
		classification Classification
		expected       FailureClass
	}{
		{ClassificationOK, CLASS_OK},
		{ClassificationChangesRequested, RESULT_FAILURE},
		{ClassificationFatal, EXECUTION_FAILURE},
		{ClassificationMaxTurns, EXECUTION_FAILURE},
		{ClassificationAuth, EXECUTION_FAILURE},
		{ClassificationNoCredit, EXECUTION_FAILURE},
		{ClassificationRateLimited, EXECUTION_FAILURE},
		{ClassificationTransient, EXECUTION_FAILURE},
		{ClassificationBlocked, ENVIRONMENT_FAILURE},
		{Classification("BOGUS"), EXECUTION_FAILURE},
	}

	for _, tc := range tests {
		if got := FailureClassOf(tc.classification); got != tc.expected {
			t.Errorf("expected %q, got %q", tc.expected, got)
		}
	}
}
