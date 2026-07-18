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
