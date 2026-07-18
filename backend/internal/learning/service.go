package learning

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

// ErrInvalidRating is returned by RecordReview when the supplied rating is
// not one of Again/Hard/Good/Easy.
var ErrInvalidRating = errors.New("invalid rating")

// Service integrates the pure FSRS math in fsrs.go with the
// question_memory/category_mastery persistence layer.
type Service struct {
	Q *sqlc.Queries
}

// NewService constructs a Service backed by the given sqlc queries.
func NewService(q *sqlc.Queries) *Service {
	return &Service{Q: q}
}

func validRating(r Rating) bool {
	switch r {
	case Again, Hard, Good, Easy:
		return true
	default:
		return false
	}
}

// RecordReview grades a review of questionID by profileID, updates the
// FSRS memory state for that question, and rolls the result up into the
// question's category_mastery row. It returns the updated Card.
func (s *Service) RecordReview(ctx context.Context, profileID, questionID uuid.UUID, rating Rating) (Card, error) {
	if !validRating(rating) {
		return Card{}, ErrInvalidRating
	}

	var card Card
	row, err := s.Q.GetQuestionMemory(ctx, sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: questionID})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return Card{}, err
		}
		// New question — zero-value Card.
	} else {
		card = Card{
			Stability:      float64(row.Stability),
			Difficulty:     float64(row.Difficulty),
			DueAt:          row.DueAt.Time,
			LastReviewedAt: row.LastReviewedAt.Time,
			Reps:           int(row.Reps),
			Lapses:         int(row.Lapses),
			State:          row.State,
		}
	}

	updated := Review(card, rating, time.Now(), DefaultDesiredRetention)

	if _, err := s.Q.UpsertQuestionMemory(ctx, sqlc.UpsertQuestionMemoryParams{
		ProfileID:      profileID,
		QuestionID:     questionID,
		Stability:      float32(updated.Stability),
		Difficulty:     float32(updated.Difficulty),
		DueAt:          pgtype.Timestamptz{Time: updated.DueAt, Valid: true},
		LastReviewedAt: pgtype.Timestamptz{Time: updated.LastReviewedAt, Valid: true},
		Reps:           int32(updated.Reps),
		Lapses:         int32(updated.Lapses),
		State:          updated.State,
	}); err != nil {
		return Card{}, err
	}

	catID, err := s.Q.GetQuestionCategoryID(ctx, questionID)
	if err != nil {
		return Card{}, err
	}

	var seen, correct int32
	m, err := s.Q.GetCategoryMastery(ctx, sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return Card{}, err
		}
		// No existing mastery row — start from zero.
	} else {
		seen = m.Seen
		correct = m.Correct
	}
	seen++
	if rating != Again {
		correct++
	}
	mastery := float32(correct) / float32(seen)

	if _, err := s.Q.UpsertCategoryMastery(ctx, sqlc.UpsertCategoryMasteryParams{
		ProfileID:  profileID,
		CategoryID: catID,
		Mastery:    mastery,
		Seen:       seen,
		Correct:    correct,
	}); err != nil {
		return Card{}, err
	}

	return updated, nil
}
