package progress

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/learning"
)

const maxDemoMigrateAnswers = 100

// DemoMigrateAnswer is one guest-demo grade carried into an authenticated account.
type DemoMigrateAnswer struct {
	QuestionID uuid.UUID
	AnswerID   uuid.UUID
	Correct    bool
	AnsweredAt time.Time // accepted for honesty; server clock owns FSRS
}

// DemoMigrateResult counts how many guest answers became mistake-bank reviews
// versus how many were intentionally skipped (correct, invalid, or empty).
type DemoMigrateResult struct {
	Migrated int `json:"migrated"`
	Skipped  int `json:"skipped"`
}

// ErrLearningUnavailable is returned when MigrateDemoProgress is called without
// a Learning service wired (programming error in server setup).
var ErrLearningUnavailable = errors.New("learning service unavailable")

// MigrateDemoProgress applies guest demo answers to the profile.
//
// Incorrect + valid Q/A → FSRS Again (mistake bank / due reviews).
// Correct → skipped (no mastery / Grand Mock inflation).
// Invalid UUIDs or answers that do not belong to the question → skipped.
// Idempotent: re-applying Again on the same question is safe.
func (s *Service) MigrateDemoProgress(ctx context.Context, profileID uuid.UUID, answers []DemoMigrateAnswer) (DemoMigrateResult, error) {
	if s.Learning == nil {
		return DemoMigrateResult{}, ErrLearningUnavailable
	}
	if len(answers) > maxDemoMigrateAnswers {
		answers = answers[:maxDemoMigrateAnswers]
	}

	var out DemoMigrateResult
	seenQ := make(map[uuid.UUID]struct{}, len(answers))

	for _, a := range answers {
		if a.QuestionID == uuid.Nil || a.AnswerID == uuid.Nil {
			out.Skipped++
			continue
		}
		// Last write wins per question within one payload.
		if _, dup := seenQ[a.QuestionID]; dup {
			out.Skipped++
			continue
		}
		seenQ[a.QuestionID] = struct{}{}

		if a.Correct {
			out.Skipped++
			continue
		}

		_, err := s.Q.GetAnswerForScoring(ctx, sqlc.GetAnswerForScoringParams{
			ID:         a.AnswerID,
			QuestionID: a.QuestionID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				out.Skipped++
				continue
			}
			return out, err
		}

		if _, err := s.Learning.RecordReview(ctx, profileID, a.QuestionID, learning.Again); err != nil {
			return out, err
		}
		out.Migrated++
	}
	return out, nil
}
