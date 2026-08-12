package domain

// Classification is the outcome category of an agent session, derived from
// the session's exit code and stderr output rather than from review text.
type Classification string

const (
	ClassificationOK Classification = "OK"

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

// FailureClass is a coarse-grained bucket that groups Classification values
// by the phase of the pipeline where a failure (if any) originated.
type FailureClass string

const (
	CLASS_OK            FailureClass = "CLASS_OK"
	EXECUTION_FAILURE   FailureClass = "EXECUTION_FAILURE"
	CONTRACT_FAILURE    FailureClass = "CONTRACT_FAILURE"
	GATE_FAILURE        FailureClass = "GATE_FAILURE"
	RESULT_FAILURE      FailureClass = "RESULT_FAILURE"
	ENVIRONMENT_FAILURE FailureClass = "ENVIRONMENT_FAILURE"
	// FATAL indicates an unrecoverable structural failure: a cyclic role
	// delegation graph. It is set directly (never derived from a Classification)
	// and aborts the delegation chain.
	FATAL FailureClass = "FATAL"
)

// FailureClassOf maps a Classification to its coarse-grained FailureClass.
func FailureClassOf(c Classification) FailureClass {
	switch c {
	case ClassificationOK:
		return CLASS_OK
	case ClassificationFatal, ClassificationTransient, ClassificationRateLimited:
		return EXECUTION_FAILURE
	case ClassificationBlocked:
		return ENVIRONMENT_FAILURE
	case ClassificationChangesRequested:
		return RESULT_FAILURE
	default:
		return EXECUTION_FAILURE
	}
}
