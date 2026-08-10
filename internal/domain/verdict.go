// Package domain contains the core business types for mill.
// These types have no infrastructure imports — they are pure domain models.
package domain

// Verdict represents the outcome of an agent session review.
type Verdict string

const (
	VerdictApproved Verdict = "approved"
	VerdictChanges  Verdict = "changes"

	// VerdictChangesRequested means the reviewer requested modifications
	// but the review cycle was exhausted without approval.
	VerdictChangesRequested Verdict = "changes_requested"

	VerdictRejected Verdict = "rejected"
)
