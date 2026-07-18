package session

import (
	"time"

	"github.com/google/uuid"
)

type StartRequest struct {
	Mode       string
	VariantID  uuid.UUID
	CategoryID uuid.UUID
	SignID     uuid.UUID
	Locale     string
	Count      int
}

type SessionView struct {
	ID           uuid.UUID
	Mode         string
	QuestionIDs  []uuid.UUID
	TimeLimitSec *int
	Total        int
	StartedAt    time.Time
}

// AnswerResult is the outcome of a single SubmitAnswer call. In exam mode,
// Correct and CorrectAnswerID are withheld (left nil) until the exam
// finishes; other modes populate both immediately.
type AnswerResult struct {
	Recorded        bool
	Correct         *bool      // nil for exam mode (feedback withheld)
	CorrectAnswerID *uuid.UUID // nil for exam mode
	Stopped         bool       // true if this answer ended an exam (3rd mistake)
	StopReason      string     // "too_many_errors" when Stopped
}

// FinishResult is the full outcome of finishing an exam session: its final
// pass/fail/abandoned status, why it stopped, and the score achieved.
type FinishResult struct {
	Status        string // "passed" | "failed" | "abandoned"
	StoppedReason string
	Score         int
	Total         int
}

// AnsweredQuestion reports the recorded outcome of one answered question
// within a session. Correct is nil while an exam-mode session is still
// in_progress (anti-cheat redaction) and populated for every mode once the
// session is no longer in_progress.
type AnsweredQuestion struct {
	QuestionID uuid.UUID
	Position   int
	Answered   bool
	Correct    *bool // nil while an exam session is still in_progress
}

// SessionDetail is the resume/history view of a single session: its
// original parameters (via the embedded SessionView), its current
// status/score, and the per-question answers recorded so far.
type SessionDetail struct {
	SessionView
	Status        string
	StoppedReason string
	Score         *int
	FinishedAt    *time.Time
	Answers       []AnsweredQuestion
}

// SessionSummary is a condensed history-list entry for one past or
// in-progress session.
type SessionSummary struct {
	ID         uuid.UUID
	Mode       string
	Status     string
	Score      *int
	Total      int
	StartedAt  time.Time
	FinishedAt *time.Time
}

// VariantStatus is the bilet-unlock/progress view of a single variant for a
// given profile: whether it's unlocked yet, and the profile's best result.
type VariantStatus struct {
	Number        int32
	QuestionCount int
	Unlocked      bool
	BestCorrect   int
	Attempts      int
	CompletedAt   *time.Time
}
