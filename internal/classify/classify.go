// Package classify provides session outcome classification for agent runs.
// It inspects an agent's exit code and stderr output to determine the
// category of the session outcome (e.g. OK, FATAL, AUTH, ...), rather than
// parsing review verdicts from text output.
package classify

import (
	"strings"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// Classify examines an agent session's exit code and stderr output and
// returns the corresponding Classification.
//
// Stderr signals are checked first (priority over exit code), in the order:
//  1. "blocked:"           -> BLOCKED
//  2. auth signals         -> AUTH
//  3. no-credit signals    -> NO_CREDIT
//  4. rate-limit signals   -> RATE_LIMITED
//  5. transient signals    -> TRANSIENT
//
// If no stderr signal matches, the exit code is mapped:
// 0 -> OK, 3 -> AUTH, 4/9/130/137/143 -> FATAL, 5 -> RATE_LIMITED,
// 6/7 -> TRANSIENT, 8 -> MAX_TURNS, 10 -> NO_CREDIT, default -> FATAL.
func Classify(exitCode int, stderr string) domain.Classification {
	lower := strings.ToLower(stderr)
	// Check stderr signals first (priority over exit code)
	if strings.Contains(lower, "blocked:") {
		return domain.ClassificationBlocked
	}
	if strings.Contains(lower, "not authenticated") || strings.Contains(lower, "no api key") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "401") || strings.Contains(lower, "403") {
		return domain.ClassificationAuth
	}
	if strings.Contains(lower, "insufficient credits") || strings.Contains(lower, "no credits") || strings.Contains(lower, "credit limit") {
		return domain.ClassificationNoCredit
	}
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") {
		return domain.ClassificationRateLimited
	}
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "econnrefused") || strings.Contains(lower, "timeout") {
		return domain.ClassificationTransient
	}
	// Fall back to exit code
	switch exitCode {
	case 0:
		return domain.ClassificationOK
	case 3:
		return domain.ClassificationAuth
	case 4, 9, 130, 137, 143:
		return domain.ClassificationFatal
	case 5:
		return domain.ClassificationRateLimited
	case 6, 7:
		return domain.ClassificationTransient
	case 8:
		return domain.ClassificationMaxTurns
	case 10:
		return domain.ClassificationNoCredit
	default:
		return domain.ClassificationFatal
	}
}
