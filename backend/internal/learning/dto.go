package learning

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
