package session

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type StartRequest struct {
	Mode       string
	VariantID  uuid.UUID
	CategoryID uuid.UUID
	SignID     uuid.UUID
	// HasImage narrows practice to illustrated (true) or text-only (false)
	// questions. Nil means the selector is unused, which is how it stays
	// mutually exclusive with CategoryID and SignID.
	HasImage *bool
	// VariantFrom/VariantTo draw across a contiguous bilet span. Both are set
	// together or neither is; zero means the selector is unused.
	VariantFrom int
	VariantTo   int
	Locale      string
	Count       int
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
	Explanation     *ExplanationPayload
	Stopped         bool   // true if this answer ended an exam (3rd mistake)
	StopReason      string // "too_many_errors" when Stopped
}

// ExplanationPayload is localized, verified answer feedback. It is returned
// only after disclosure is allowed for the session mode.
type ExplanationPayload struct {
	LegalRefs json.RawMessage `json:"legal_refs"`
	Blocks    json.RawMessage `json:"blocks"`
}

// SessionQuestionAccess is the authorization and anti-cheat decision for one
// assigned question. The content handler uses FeedbackAllowed to decide
// whether explanation prose may be serialized.
type SessionQuestionAccess struct {
	Position        int
	Answered        bool
	UserAnswerID    *uuid.UUID
	Correct         *bool
	CorrectAnswerID *uuid.UUID
	FeedbackAllowed bool
}

// FinishResult is the full outcome of finishing an exam session: its final
// pass/fail/abandoned status, why it stopped, and the score achieved.
// CertificateShareCode is set when a Grand Mock pass persists a shareable credential.
type FinishResult struct {
	Status               string // "passed" | "failed" | "abandoned"
	StoppedReason        string
	Score                int
	Total                int
	CertificateShareCode string `json:"certificate_share_code,omitempty"`
}

// AnsweredQuestion reports the recorded outcome of one answered question
// within a session. Correct is nil while an exam-mode session is still
// in_progress (anti-cheat redaction) and populated for every mode once the
// session is no longer in_progress.
type AnsweredQuestion struct {
	QuestionID      uuid.UUID
	Position        int
	Answered        bool
	UserAnswerID    *uuid.UUID // nil until the profile answers this question
	Correct         *bool      // nil when unanswered or anti-cheat redaction applies
	CorrectAnswerID *uuid.UUID // nil when the answer key must remain hidden
}

// SessionDetail is the resume/history view of a single session: its
// original parameters (via the embedded SessionView), its current
// status/score, and the per-question answers recorded so far.
type SessionDetail struct {
	SessionView
	Status               string
	StoppedReason        string
	Score                *int
	FinishedAt           *time.Time
	Answers              []AnsweredQuestion
	CertificateShareCode string
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
