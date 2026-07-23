package demo

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	// ErrNotFound covers every "no demo question here" case: variant number
	// 1 doesn't exist, it has no questions, or (for SubmitAnswer) the given
	// question_id is real but outside the demo whitelist. It is never
	// fabricated — an empty/short variant 1 is reported as-is.
	ErrNotFound = errors.New("demo question not found")
	// ErrInvalidAnswer mirrors internal/session's rule of the same name:
	// answer_id does not belong to question_id.
	ErrInvalidAnswer = errors.New("answer does not belong to question")
	// ErrRateLimited is returned once an IP exceeds demoAnswerRateLimit
	// requests per hour against SubmitAnswer.
	ErrRateLimited = errors.New("rate limited")
)

// demoVariantNumber is the free bilet the whitelist is drawn from.
const demoVariantNumber = 1

// demoAnswerRateLimit is the per-IP hourly cap on POST /demo/answer, using
// the same fixed-window Limiter as the auth/OTP package.
const demoAnswerRateLimit = 60

// Service implements the public, unauthenticated demo surface: one
// real question a visitor can answer without registration. It is
// completely stateless aside from the Redis rate-limit counter — no
// session, FSRS, streak, or event rows are ever written here.
type Service struct {
	Q       *sqlc.Queries
	Content *content.Handler
	Lim     auth.Limiter
}

// NewService builds a demo Service. Content is reused (not duplicated) so
// GetQuestion renders the exact same DTO shape and locale-fallback
// behavior as GET /questions/{id}.
func NewService(q *sqlc.Queries, ch *content.Handler, lim auth.Limiter) *Service {
	return &Service{Q: q, Content: ch, Lim: lim}
}

// whitelistIDs returns the current demo whitelist (see rules.go), or
// ErrNotFound if variant number 1 doesn't exist or has no questions.
func (s *Service) whitelistIDs(ctx context.Context) ([]uuid.UUID, error) {
	v, err := s.Q.GetVariantByNumber(ctx, demoVariantNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	ordered, err := s.Q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	wl := Whitelist(ordered)
	if len(wl) == 0 {
		return nil, ErrNotFound
	}
	return wl, nil
}

// GetQuestion returns one randomly-chosen question from the demo whitelist,
// rendered via content.Handler.LoadQuestionDetail (same DTO/locale-fallback
// path as GET /questions/{id} — no correctness fields, ever).
func (s *Service) GetQuestion(ctx context.Context, locale string) (content.QuestionDetailDTO, bool, error) {
	ids, err := s.whitelistIDs(ctx)
	if err != nil {
		return content.QuestionDetailDTO{}, false, err
	}
	pick := ids[rand.IntN(len(ids))]
	detail, fallback, err := s.Content.LoadQuestionDetail(ctx, pick, locale)
	if err != nil {
		return content.QuestionDetailDTO{}, false, err
	}
	// Explanations frequently name the correct option. The public question
	// response is pre-grade, so it must never expose that prose.
	detail.Explanation = nil
	return detail, fallback, nil
}

// SubmitAnswer grades a single answer against a whitelisted demo question.
// It never grades anything outside the whitelist (ErrNotFound), and never
// accepts an answer_id that doesn't belong to question_id (ErrInvalidAnswer)
// — the same ownership rule internal/session.SubmitAnswer enforces via
// GetAnswerForScoring. ip empty means "don't rate limit" (used only when a
// caller genuinely has no client IP; the HTTP handler always has one).
func (s *Service) SubmitAnswer(ctx context.Context, ip string, questionID, answerID uuid.UUID) (bool, uuid.UUID, error) {
	if ip != "" {
		ok, err := s.Lim.Allow(ctx, "demo:answer:ip:"+ip, demoAnswerRateLimit, time.Hour)
		if err != nil {
			return false, uuid.UUID{}, err
		}
		if !ok {
			return false, uuid.UUID{}, ErrRateLimited
		}
	}

	ids, err := s.whitelistIDs(ctx)
	if err != nil {
		return false, uuid.UUID{}, err
	}
	if !IsWhitelisted(ids, questionID) {
		return false, uuid.UUID{}, ErrNotFound
	}

	ans, err := s.Q.GetAnswerForScoring(ctx, sqlc.GetAnswerForScoringParams{ID: answerID, QuestionID: questionID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, uuid.UUID{}, ErrInvalidAnswer
		}
		return false, uuid.UUID{}, err
	}

	correctID := answerID
	if !ans.IsCorrect {
		correctID, err = s.Q.GetCorrectAnswerID(ctx, questionID)
		if err != nil {
			return false, uuid.UUID{}, err
		}
	}
	return ans.IsCorrect, correctID, nil
}
