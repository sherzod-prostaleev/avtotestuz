// Package demo serves the ONE public, unauthenticated question-and-answer
// surface the landing page needs (AVTOTEST-MASTER-PROMPT.txt §6.1 / §0
// item d). It never becomes a general-purpose grading oracle: both
// endpoints are scoped to a small, deterministic whitelist — the first
// demoQuestionCount questions (by position) of variant number 1, the free
// bilet — and it writes nothing to any user/learning table.
package demo

import "github.com/google/uuid"

// demoQuestionCount is the size of the public demo whitelist. It is a code
// constant, not configuration: deterministic, no DB round-trip, and works
// identically against the [NAMUNA] fixture and real content.
const demoQuestionCount = 2

// Whitelist returns the demo-eligible prefix of a variant's ordered question
// IDs — the first demoQuestionCount entries, or fewer if the variant itself
// has fewer questions than that.
func Whitelist(orderedIDs []uuid.UUID) []uuid.UUID {
	n := demoQuestionCount
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
