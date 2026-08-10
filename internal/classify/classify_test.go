package classify

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		stderr   string
		want     Classification
	}{
		// --- Exit-code driven classifications ---
		{name: "exit 0 -> OK", exitCode: 0, stderr: "", want: OK},
		{name: "exit 3 -> AUTH", exitCode: 3, stderr: "", want: AUTH},
		{name: "exit 4 -> FATAL", exitCode: 4, stderr: "", want: FATAL},
		{name: "exit 9 -> FATAL", exitCode: 9, stderr: "", want: FATAL},
		{name: "exit 130 -> FATAL", exitCode: 130, stderr: "", want: FATAL},
		{name: "exit 137 -> FATAL", exitCode: 137, stderr: "", want: FATAL},
		{name: "exit 143 -> FATAL", exitCode: 143, stderr: "", want: FATAL},
		{name: "exit 5 -> RATE_LIMITED", exitCode: 5, stderr: "", want: RATE_LIMITED},
		{name: "exit 6 -> TRANSIENT", exitCode: 6, stderr: "", want: TRANSIENT},
		{name: "exit 7 -> TRANSIENT", exitCode: 7, stderr: "", want: TRANSIENT},
		{name: "exit 8 -> MAX_TURNS", exitCode: 8, stderr: "", want: MAX_TURNS},
		{name: "exit 10 -> NO_CREDIT", exitCode: 10, stderr: "", want: NO_CREDIT},
		{name: "exit 999 -> FATAL default", exitCode: 999, stderr: "", want: FATAL},

		// --- Stderr-driven classifications (all 5 signal groups) ---
		{name: "stderr blocked -> BLOCKED", exitCode: 0, stderr: "session blocked: access denied", want: BLOCKED},
		{name: "stderr not authenticated -> AUTH", exitCode: 0, stderr: "not authenticated", want: AUTH},
		{name: "stderr no api key -> AUTH", exitCode: 0, stderr: "error: no api key found", want: AUTH},
		{name: "stderr unauthorized -> AUTH", exitCode: 0, stderr: "unauthorized request", want: AUTH},
		{name: "stderr 401 -> AUTH", exitCode: 0, stderr: "HTTP 401 failed", want: AUTH},
		{name: "stderr 403 -> AUTH", exitCode: 0, stderr: "HTTP 403 forbidden", want: AUTH},
		{name: "stderr insufficient credits -> NO_CREDIT", exitCode: 0, stderr: "insufficient credits on account", want: NO_CREDIT},
		{name: "stderr no credits -> NO_CREDIT", exitCode: 0, stderr: "no credits remaining", want: NO_CREDIT},
		{name: "stderr credit limit -> NO_CREDIT", exitCode: 0, stderr: "credit limit exceeded", want: NO_CREDIT},
		{name: "stderr rate limit -> RATE_LIMITED", exitCode: 0, stderr: "rate limit exceeded", want: RATE_LIMITED},
		{name: "stderr 429 -> RATE_LIMITED", exitCode: 0, stderr: "HTTP 429 too many requests", want: RATE_LIMITED},
		{name: "stderr too many requests -> RATE_LIMITED", exitCode: 0, stderr: "too many requests", want: RATE_LIMITED},
		{name: "stderr connection refused -> TRANSIENT", exitCode: 0, stderr: "connection refused", want: TRANSIENT},
		{name: "stderr econnrefused -> TRANSIENT", exitCode: 0, stderr: "getaddrinfo: econnrefused", want: TRANSIENT},
		{name: "stderr timeout -> TRANSIENT", exitCode: 0, stderr: "request timeout", want: TRANSIENT},

		// --- Stderr priority over exit code ---
		{name: "blocked priority over exit 0", exitCode: 0, stderr: "blocked: by guardrail", want: BLOCKED},
		{name: "blocked priority over exit 4", exitCode: 4, stderr: "blocked: by guardrail", want: BLOCKED},
		{name: "auth priority over exit 8", exitCode: 8, stderr: "not authenticated", want: AUTH},
		{name: "no credit priority over exit 0", exitCode: 0, stderr: "insufficient credits", want: NO_CREDIT},
		{name: "rate limit priority over exit 0", exitCode: 0, stderr: "rate limit", want: RATE_LIMITED},
		{name: "transient priority over exit 0", exitCode: 0, stderr: "connection refused", want: TRANSIENT},
		{name: "case-insensitive auth", exitCode: 0, stderr: "NOT AUTHENTICATED", want: AUTH},
		{name: "case-insensitive blocked", exitCode: 0, stderr: "BLOCKED: guardrail", want: BLOCKED},
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
	seen := map[Classification]bool{}
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
	for _, c := range []Classification{OK, FATAL, MAX_TURNS, AUTH, NO_CREDIT, RATE_LIMITED, TRANSIENT, BLOCKED} {
		if !seen[c] {
			t.Errorf("classification %q is never produced by Classify", c)
		}
	}
}
