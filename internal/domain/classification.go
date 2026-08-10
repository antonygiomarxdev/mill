package domain

// Classification is the outcome category of an agent session, derived from
// the session's exit code and stderr output rather than from review text.
type Classification string

const (
	OK           Classification = "OK"
	FATAL        Classification = "FATAL"
	MAX_TURNS    Classification = "MAX_TURNS"
	AUTH         Classification = "AUTH"
	NO_CREDIT    Classification = "NO_CREDIT"
	RATE_LIMITED Classification = "RATE_LIMITED"
	TRANSIENT    Classification = "TRANSIENT"
	BLOCKED      Classification = "BLOCKED"
)
