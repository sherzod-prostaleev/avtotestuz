package session

// Ordered category practice: the "Hammasi" button on one of the 13 topics.
//
// Every other practice draw is random, and for a learner picking 20 questions
// that is right. It is wrong for a teacher taking a class through a topic. The
// road-signs topic alone is 337 questions; drawn at random each lesson, a class
// sees some questions five times and others never, and there is no answer to
// "where did we get to last time".
//
// So this one combination -- one topic, all of its questions -- walks the topic
// in a fixed order and remembers how far the profile got. Nothing else changes:
// the 20/50/100 presets, a typed count, signs, ticket ranges and the with-image
// selector all still draw at random.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

// PracticeProgressItem is how far one profile has walked one topic.
//
// Carries the category's code as well as its id. The practice screen is the
// only consumer and it holds codes, not uuids: GET /categories answers with
// code, name, sort_order and question_count (content.CategoryDTO). Sending the
// uuid alone would make this list unmatchable by the screen it exists for.
type PracticeProgressItem struct {
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryCode string    `json:"category_code"`
	// NextIndex is 0-based and counts questions already worked through, so 123
	// means the next question served is the 124th.
	NextIndex int `json:"next_index"`
	Total     int `json:"total"`
}

// orderedCategoryDraw returns the next stretch of a topic in source order,
// along with the index it starts at, which the session stores so recording an
// answer can move the cursor to the right absolute position.
func (s *Service) orderedCategoryDraw(
	ctx context.Context, profileID, categoryID uuid.UUID, count int,
) ([]uuid.UUID, pgtype.Int4, error) {
	var none pgtype.Int4

	total, err := s.Q.CountValidQuestionsInCategory(ctx, categoryID)
	if err != nil {
		return nil, none, err
	}
	if total == 0 {
		// An empty topic has no walk to resume and no position to store, and
		// dividing by its size below would decide the wrap on nothing. Refused
		// outright rather than served as an empty session -- which is what the
		// random path does with an empty topic, since StartSession only guards
		// len(ids) for the exam-like modes.
		return nil, none, ErrInvalidRequest
	}

	from, err := s.Q.GetPracticeCursor(ctx, sqlc.GetPracticeCursorParams{
		ProfileID: profileID, CategoryID: categoryID,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		from = 0 // never started this topic
	case err != nil:
		return nil, none, err
	}

	// Wrap. Reaching the end of a topic starts it again, which is what a
	// classroom wants: the next group begins at question 1 without anyone
	// having to reset anything. This also repairs a cursor left past the end by
	// a shrinking bank -- a quarantined question lowers the count, and a stored
	// index beyond it would otherwise draw nothing forever.
	//
	// The wrap is WRITTEN DOWN, not just applied to the local copy, and that is
	// load-bearing. The cursor only ever moves forward (AdvancePracticeCursor
	// uses GREATEST, which is what stops an out-of-order answer rewinding a
	// class). A wrap that lived only in this function would leave 337 stored
	// while the draw began at 0, so answering the first question would write
	// GREATEST(337, 1) = 337 and the cursor would never move again: every draw
	// from then on would wrap to 0 and the class would repeat the same
	// questions for the life of the topic.
	if from >= total {
		if err := s.Q.ResetPracticeCursor(ctx, sqlc.ResetPracticeCursorParams{
			ProfileID: profileID, CategoryID: categoryID,
		}); err != nil {
			return nil, none, err
		}
		from = 0
	}

	ids, err := s.Q.OrderedQuestionIDsByCategory(ctx, sqlc.OrderedQuestionIDsByCategoryParams{
		CategoryID: categoryID, Skip: int32(from), LimitCount: int32(count),
	})
	if err != nil {
		return nil, none, err
	}
	return ids, pgtype.Int4{Int32: from, Valid: true}, nil
}

// advanceOrderedCursor moves a profile's position in a topic to the end of what
// it has actually worked through. q is the caller's transaction, so the cursor
// moves with the answer or not at all.
//
// It runs on answer, never on session creation, and that is the whole point of
// "continue from 123": a class that opens a 337-question session, answers 123
// and switches the PC off must resume at 124, not skip to the end.
//
// How far "worked through" reaches is decided by the query, not here -- see
// AdvancePracticeCursor for why it is the contiguous answered run rather than
// the furthest question touched, and for the guard that ignores answers
// belonging to an earlier walk.
func advanceOrderedCursor(
	ctx context.Context, q *sqlc.Queries, profileID uuid.UUID, row sqlc.ExamSession,
) error {
	if !row.OrderedFrom.Valid || !row.CategoryID.Valid {
		return nil
	}
	return q.AdvancePracticeCursor(ctx, sqlc.AdvancePracticeCursorParams{
		ProfileID:   profileID,
		CategoryID:  row.CategoryID.UUID,
		SessionID:   row.ID,
		OrderedFrom: row.OrderedFrom.Int32,
	})
}

// PracticeProgress reports where this profile stands in every topic it has
// started. Topics never started are absent rather than zero: one call answers
// for all 13 at once, and a caller reads a missing entry as "from the start".
func (s *Service) PracticeProgress(ctx context.Context, profileID uuid.UUID) ([]PracticeProgressItem, error) {
	rows, err := s.Q.ListPracticeProgress(ctx, profileID)
	if err != nil {
		return nil, err
	}
	out := make([]PracticeProgressItem, 0, len(rows))
	for _, r := range rows {
		next := int(r.NextIndex)
		total := int(r.Total)
		// Report the wrap the next draw will perform, rather than a position
		// past the end that no screen could render sensibly.
		if next >= total {
			next = 0
		}
		out = append(out, PracticeProgressItem{
			CategoryID: r.CategoryID, CategoryCode: r.CategoryCode,
			NextIndex: next, Total: total,
		})
	}
	return out, nil
}

// ResetPracticeProgress puts a topic back to its first question.
//
// A classroom needs this because the cursor belongs to the PC, not to a
// student: when a new group starts the topic, the previous group's position is
// still stored and only a person can say that it should not be. Resetting a
// topic that was never started succeeds -- there is nothing to fail, and a
// caller should not have to check first.
func (s *Service) ResetPracticeProgress(ctx context.Context, profileID, categoryID uuid.UUID) error {
	return s.Q.ResetPracticeCursor(ctx, sqlc.ResetPracticeCursorParams{
		ProfileID: profileID, CategoryID: categoryID,
	})
}
