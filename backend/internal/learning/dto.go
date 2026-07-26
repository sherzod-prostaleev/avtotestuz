package learning

import "time"

// CategoryStat summarizes a profile's progress in a single content
// category, for display in exam-readiness stats.
//
// Mastery is bank-honest: (studied/total) × (correct/seen). Unseen
// questions in the category pull the percentage toward zero, so 100%
// requires covering the full category with correct answers — not re-
// drilling a handful of easy items.
type CategoryStat struct {
	CategoryCode   string
	Mastery        float64
	Seen, Correct  int
	Studied, Total int
}

// PassEstimate is a calibrated estimate of mock/exam pass chance given the
// profile's current bank-honest readiness. Source is "empirical" when enough
// finished sessions with readiness snapshots exist, otherwise "model".
type PassEstimate struct {
	EstimatedPassPct int    `json:"estimated_pass_pct"`
	Source           string `json:"source"` // empirical | model
	SampleSize       int    `json:"sample_size"`
	BucketLo         int    `json:"bucket_lo"`
}

// Stats is the exam-readiness snapshot for a profile: per-category
// mastery, an overall weighted readiness percentage, and the count of
// questions currently due for review.
type Stats struct {
	Categories   []CategoryStat
	ReadinessPct int
	DueCount     int
	PassEstimate PassEstimate
}

// MistakeBankSummary separates valid questions due right now from every valid
// question the profile has ever lapsed on. NextDueAt lets clients explain the
// normal FSRS state where a newly missed question returns tomorrow rather
// than appearing in the due queue immediately.
type MistakeBankSummary struct {
	DueCount       int
	TotalBankCount int
	NextDueAt      *time.Time
}
