package session

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

// MemorizeItem is one question's position and correct answer within a
// topic's memorize view. Handler.categoryMemorize composes this with
// content.Handler's question detail the same way listSessionQuestions
// composes SessionQuestionAccess with it (see decorateSessionQuestion).
type MemorizeItem struct {
	QuestionID      uuid.UUID
	Position        int
	CorrectAnswerID uuid.UUID
}

// CategoryMemorize returns every valid question in categoryID, in the
// topic's fixed source order (see OrderedQuestionIDsByCategory), each
// already carrying its correct answer.
//
// Unlike ordered practice (orderedCategoryDraw), this reads the whole topic
// in one call and touches no practice_cursor, session, or answer history:
// the same topic returns the same list from question 1, request after
// request. It exists for the "Yodlash" memorize view, which is deliberately
// not a session — nothing here is answered, scored, or FSRS-scheduled.
func (s *Service) CategoryMemorize(ctx context.Context, categoryID uuid.UUID) ([]MemorizeItem, error) {
	total, err := s.Q.CountValidQuestionsInCategory(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return []MemorizeItem{}, nil
	}

	ids, err := s.Q.OrderedQuestionIDsByCategory(ctx, sqlc.OrderedQuestionIDsByCategoryParams{
		CategoryID: categoryID, Skip: 0, LimitCount: total,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.Q.ListCorrectAnswerIDsForQuestions(ctx, ids)
	if err != nil {
		return nil, err
	}
	correctByID := make(map[uuid.UUID]uuid.UUID, len(rows))
	for _, row := range rows {
		correctByID[row.QuestionID] = row.AnswerID
	}

	out := make([]MemorizeItem, 0, len(ids))
	for i, id := range ids {
		correctAnswerID, ok := correctByID[id]
		if !ok {
			return nil, fmt.Errorf("category %s question %s has no correct answer", categoryID, id)
		}
		out = append(out, MemorizeItem{QuestionID: id, Position: i + 1, CorrectAnswerID: correctAnswerID})
	}
	return out, nil
}
