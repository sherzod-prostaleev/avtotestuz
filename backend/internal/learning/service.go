package learning

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

// defaultNextDueLimit is used by NextDue when the caller passes limit <= 0.
const defaultNextDueLimit = 20

// contentLocale is the locale used to fetch category code/name pairs for
// Stats. Only Code is used from the result; the locale choice does not
// affect correctness (Task 1's ListCategories falls back across locales
// regardless).
const contentLocale = "uz-Latn"

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

// NextDue returns up to limit question IDs that are due for review right
// now, round-robin interleaved across categories in ascending due-urgency
// (the category holding the single most-overdue item goes first in each
// round). This avoids returning a long contiguous run of one category's
// questions when several categories have due items.
func (s *Service) NextDue(ctx context.Context, profileID uuid.UUID, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = defaultNextDueLimit
	}

	rows, err := s.Q.ListDueQuestions(ctx, sqlc.ListDueQuestionsParams{
		ProfileID:  profileID,
		LimitCount: int32(limit * 3),
	})
	if err != nil {
		return nil, err
	}

	// Group by category, preserving first-appearance order (rows arrive
	// ordered by due_at ASC, so the first-seen category is whichever holds
	// the single most-overdue question).
	var order []uuid.UUID
	groups := make(map[uuid.UUID][]uuid.UUID)
	for _, r := range rows {
		if _, ok := groups[r.CategoryID]; !ok {
			order = append(order, r.CategoryID)
		}
		groups[r.CategoryID] = append(groups[r.CategoryID], r.QuestionID)
	}

	result := make([]uuid.UUID, 0, limit)
	for len(result) < limit {
		progressedThisRound := false
		for _, catID := range order {
			if len(result) >= limit {
				break
			}
			pending := groups[catID]
			if len(pending) == 0 {
				continue
			}
			result = append(result, pending[0])
			groups[catID] = pending[1:]
			progressedThisRound = true
		}
		if !progressedThisRound {
			break // every category's group is exhausted
		}
	}

	return result, nil
}

// Stats returns a profile's exam-readiness snapshot: per-category mastery
// (including categories the profile has never touched, at mastery=0), an
// overall readiness percentage weighted by each category's question count,
// and the count of questions currently due for review.
func (s *Service) Stats(ctx context.Context, profileID uuid.UUID) (Stats, error) {
	masteryRows, err := s.Q.ListCategoryMasteryForProfile(ctx, profileID)
	if err != nil {
		return Stats{}, err
	}
	masteryByCategory := make(map[uuid.UUID]sqlc.CategoryMastery, len(masteryRows))
	for _, m := range masteryRows {
		masteryByCategory[m.CategoryID] = m
	}

	countRows, err := s.Q.CountValidQuestionsByCategory(ctx)
	if err != nil {
		return Stats{}, err
	}
	countByCategory := make(map[uuid.UUID]int32, len(countRows))
	for _, c := range countRows {
		countByCategory[c.CategoryID] = c.QuestionCount
	}

	catInfo, err := s.Q.ListCategories(ctx, contentLocale)
	if err != nil {
		return Stats{}, err
	}

	categories := make([]CategoryStat, 0, len(catInfo))
	var weightedMasterySum float64
	var totalQuestionCount int64
	for _, ci := range catInfo {
		var mastery float64
		var seen, correct int32
		if m, ok := masteryByCategory[ci.ID]; ok {
			mastery = float64(m.Mastery)
			seen = m.Seen
			correct = m.Correct
		}
		categories = append(categories, CategoryStat{
			CategoryCode: ci.Code,
			Mastery:      mastery,
			Seen:         int(seen),
			Correct:      int(correct),
		})

		if count := int64(countByCategory[ci.ID]); count > 0 {
			weightedMasterySum += mastery * float64(count)
			totalQuestionCount += count
		}
	}

	readiness := 0
	if totalQuestionCount > 0 {
		readiness = int(math.Round(100 * weightedMasterySum / float64(totalQuestionCount)))
	}

	dueCount, err := s.Q.CountDueQuestions(ctx, profileID)
	if err != nil {
		return Stats{}, err
	}

	return Stats{
		Categories:   categories,
		ReadinessPct: readiness,
		DueCount:     int(dueCount),
	}, nil
}
