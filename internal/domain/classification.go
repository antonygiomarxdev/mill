package domain

// Classification is the outcome category of an agent session, derived from
// the session's exit code and stderr output rather than from review text.
type Classification string

const (
	ClassificationOK          Classification = "OK"

	// ClassificationChangesRequested is returned when the agent's stderr
	// contains "changes_requested:" — the reviewer requested modifications.
	ClassificationChangesRequested Classification = "CHANGES_REQUESTED"

	ClassificationFatal       Classification = "FATAL"
	ClassificationMaxTurns    Classification = "MAX_TURNS"
	ClassificationAuth        Classification = "AUTH"
	ClassificationNoCredit    Classification = "NO_CREDIT"
	ClassificationRateLimited Classification = "RATE_LIMITED"
	ClassificationTransient   Classification = "TRANSIENT"
	ClassificationBlocked     Classification = "BLOCKED"
)
