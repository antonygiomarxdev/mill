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
