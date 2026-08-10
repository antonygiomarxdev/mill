package classify

import (
	"testing"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		stderr   string
		want     domain.Classification
	}{
		// --- Exit-code driven classifications ---
		{name: "exit 0 -> OK", exitCode: 0, stderr: "", want: domain.ClassificationOK},
		{name: "exit 3 -> AUTH", exitCode: 3, stderr: "", want: domain.ClassificationAuth},
		{name: "exit 4 -> FATAL", exitCode: 4, stderr: "", want: domain.ClassificationFatal},
		{name: "exit 9 -> FATAL", exitCode: 9, stderr: "", want: domain.ClassificationFatal},
		{name: "exit 130 -> FATAL", exitCode: 130, stderr: "", want: domain.ClassificationFatal},
		{name: "exit 137 -> FATAL", exitCode: 137, stderr: "", want: domain.ClassificationFatal},
		{name: "exit 143 -> FATAL", exitCode: 143, stderr: "", want: domain.ClassificationFatal},
		{name: "exit 5 -> RATE_LIMITED", exitCode: 5, stderr: "", want: domain.ClassificationRateLimited},
		{name: "exit 6 -> TRANSIENT", exitCode: 6, stderr: "", want: domain.ClassificationTransient},
		{name: "exit 7 -> TRANSIENT", exitCode: 7, stderr: "", want: domain.ClassificationTransient},
		{name: "exit 8 -> MAX_TURNS", exitCode: 8, stderr: "", want: domain.ClassificationMaxTurns},
		{name: "exit 10 -> NO_CREDIT", exitCode: 10, stderr: "", want: domain.ClassificationNoCredit},
		{name: "exit 999 -> FATAL default", exitCode: 999, stderr: "", want: domain.ClassificationFatal},

		// --- Stderr-driven classifications (all 5 signal groups) ---
		{name: "stderr blocked -> BLOCKED", exitCode: 0, stderr: "session blocked: access denied", want: domain.ClassificationBlocked},
		{name: "stderr not authenticated -> AUTH", exitCode: 0, stderr: "not authenticated", want: domain.ClassificationAuth},
		{name: "stderr no api key -> AUTH", exitCode: 0, stderr: "error: no api key found", want: domain.ClassificationAuth},
		{name: "stderr unauthorized -> AUTH", exitCode: 0, stderr: "unauthorized request", want: domain.ClassificationAuth},
		{name: "stderr 401 -> AUTH", exitCode: 0, stderr: "HTTP 401 failed", want: domain.ClassificationAuth},
		{name: "stderr 403 -> AUTH", exitCode: 0, stderr: "HTTP 403 forbidden", want: domain.ClassificationAuth},
		{name: "stderr insufficient credits -> NO_CREDIT", exitCode: 0, stderr: "insufficient credits on account", want: domain.ClassificationNoCredit},
		{name: "stderr no credits -> NO_CREDIT", exitCode: 0, stderr: "no credits remaining", want: domain.ClassificationNoCredit},
		{name: "stderr credit limit -> NO_CREDIT", exitCode: 0, stderr: "credit limit exceeded", want: domain.ClassificationNoCredit},
		{name: "stderr rate limit -> RATE_LIMITED", exitCode: 0, stderr: "rate limit exceeded", want: domain.ClassificationRateLimited},
		{name: "stderr 429 -> RATE_LIMITED", exitCode: 0, stderr: "HTTP 429 too many requests", want: domain.ClassificationRateLimited},
		{name: "stderr connection refused -> TRANSIENT", exitCode: 0, stderr: "connection refused", want: domain.ClassificationTransient},
		{name: "stderr econnrefused -> TRANSIENT", exitCode: 0, stderr: "getaddrinfo: econnrefused", want: domain.ClassificationTransient},
		{name: "stderr timeout -> TRANSIENT", exitCode: 0, stderr: "request timeout", want: domain.ClassificationTransient},

		// --- Stderr priority over exit code ---
		{name: "blocked priority over exit 0", exitCode: 0, stderr: "blocked: by guardrail", want: domain.ClassificationBlocked},
		{name: "blocked priority over exit 4", exitCode: 4, stderr: "blocked: by guardrail", want: domain.ClassificationBlocked},
		{name: "auth priority over exit 8", exitCode: 8, stderr: "not authenticated", want: domain.ClassificationAuth},
		{name: "no credit priority over exit 0", exitCode: 0, stderr: "insufficient credits", want: domain.ClassificationNoCredit},
		{name: "rate limit priority over exit 0", exitCode: 0, stderr: "rate limit", want: domain.ClassificationRateLimited},
		{name: "transient priority over exit 0", exitCode: 0, stderr: "connection refused", want: domain.ClassificationTransient},
		{name: "case-insensitive auth", exitCode: 0, stderr: "NOT AUTHENTICATED", want: domain.ClassificationAuth},
		{name: "case-insensitive blocked", exitCode: 0, stderr: "BLOCKED: guardrail", want: domain.ClassificationBlocked},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.exitCode, tc.stderr)
			if got != tc.want {
				t.Errorf("Classify(%d, %q) = %q, want %q", tc.exitCode, tc.stderr, got, tc.want)
			}
		})
	}
}

// TestClassifyAllClassificationsHit ensures every one of the eight
// Classification values is reachable through the classifier.
func TestClassifyAllClassificationsHit(t *testing.T) {
	seen := map[domain.Classification]bool{}
	for _, tc := range []struct {
		exitCode int
		stderr   string
	}{
		{0, ""},                // OK
		{4, ""},                // FATAL
		{8, ""},                // MAX_TURNS
		{3, ""},                // AUTH
		{10, ""},               // NO_CREDIT
		{5, ""},                // RATE_LIMITED
		{6, ""},                // TRANSIENT
		{0, "blocked: denied"}, // BLOCKED
	} {
		seen[Classify(tc.exitCode, tc.stderr)] = true
	}
	for _, c := range []domain.Classification{domain.ClassificationOK, domain.ClassificationFatal, domain.ClassificationMaxTurns, domain.ClassificationAuth, domain.ClassificationNoCredit, domain.ClassificationRateLimited, domain.ClassificationTransient, domain.ClassificationBlocked} {
		if !seen[c] {
			t.Errorf("classification %q is never produced by Classify", c)
		}
	}
}
