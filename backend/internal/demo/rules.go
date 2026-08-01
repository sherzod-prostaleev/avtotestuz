// Package demo serves the public, unauthenticated demo and diagnostic
// surfaces. It never becomes a general-purpose grading oracle: grading is
// scoped to a small, deterministic prefix of variant number 1 and writes
// nothing to user or learning tables.
package demo

import "github.com/google/uuid"

// demoQuestionCount is the size of the public demo whitelist. It is a code
// constant, not configuration: deterministic, no DB round-trip, and works
// identically against the [NAMUNA] fixture and real content.
const (
	demoQuestionCount       = 5
	diagnosticQuestionCount = 10
)

// Whitelist returns the demo-eligible prefix of a variant's ordered question
// IDs — the first demoQuestionCount entries, or fewer if the variant itself
// has fewer questions than that.
func Whitelist(orderedIDs []uuid.UUID) []uuid.UUID {
	return prefix(orderedIDs, demoQuestionCount)
}

// DiagnosticWhitelist returns the ordered questions used by the public
// placement diagnostic. Keeping this separate from Whitelist lets the landing
// demo remain a small random sample while the diagnostic is a stable 10-item
// assessment.
func DiagnosticWhitelist(orderedIDs []uuid.UUID) []uuid.UUID {
	return prefix(orderedIDs, diagnosticQuestionCount)
}

func prefix(orderedIDs []uuid.UUID, count int) []uuid.UUID {
	n := count
	if len(orderedIDs) < n {
		n = len(orderedIDs)
	}
	return orderedIDs[:n]
}

// IsWhitelisted reports whether id is one of the demo-eligible questions.
func IsWhitelisted(whitelist []uuid.UUID, id uuid.UUID) bool {
	for _, w := range whitelist {
		if w == id {
			return true
		}
	}
	return false
}
