package learning

import "time"

// CategoryStat summarizes a profile's progress in a single content
// category, for display in exam-readiness stats.
type CategoryStat struct {
	CategoryCode  string
	Mastery       float64
	Seen, Correct int
}

// Stats is the exam-readiness snapshot for a profile: per-category
// mastery, an overall weighted readiness percentage, and the count of
// questions currently due for review.
type Stats struct {
	Categories   []CategoryStat
	ReadinessPct int
	DueCount     int
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
