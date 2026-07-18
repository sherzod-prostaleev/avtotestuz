package explanation

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

// ErrNotFound indicates the question has no explanation (or no verified
// uz-Latn translation, depending on the operation) yet.
var ErrNotFound = errors.New("explanation not found")

// Service wires the pure draft-generation logic in draft.go to the
// explanation/explanation_translation/explanation_feedback tables.
type Service struct {
	Q         *sqlc.Queries
	Generator AIDraftGenerator
}

func NewService(q *sqlc.Queries, g AIDraftGenerator) *Service {
	return &Service{Q: q, Generator: g}
}

// CreateDraft generates and stores a uz-Latn draft for questionID (status='draft').
func (s *Service) CreateDraft(ctx context.Context, questionID uuid.UUID) error {
	q, err := s.Q.GetQuestionForDraft(ctx, questionID)
	if err != nil {
		return err
	}
	correct, err := s.Q.GetCorrectAnswerTextForDraft(ctx, questionID)
	if err != nil {
		return err
	}

	blocks := s.Generator.Generate(DraftInput{
		QuestionText:      q.QuestionText,
		CategoryCode:      q.CategoryCode,
		CorrectAnswerText: correct,
	})

	blocksJSON, err := json.Marshal(blocks)
	if err != nil {
		return err
	}

	explID, err := s.Q.InsertDraftExplanation(ctx, questionID)
	if err != nil {
		return err
	}

	return s.Q.InsertDraftTranslation(ctx, sqlc.InsertDraftTranslationParams{
		ExplanationID: explID,
		Locale:        "uz-Latn",
		Blocks:        blocksJSON,
	})
}

// Verify marks the uz-Latn translation for questionID as verified.
func (s *Service) Verify(ctx context.Context, questionID uuid.UUID, verifiedBy uuid.UUID) error {
	explID, err := s.Q.GetExplanationIDByQuestionID(ctx, questionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return s.Q.VerifyExplanationTranslation(ctx, sqlc.VerifyExplanationTranslationParams{
		ExplanationID: explID,
		Locale:        "uz-Latn",
		VerifiedBy:    uuid.NullUUID{UUID: verifiedBy, Valid: true},
	})
}

// RecordFeedback upserts a helpful/not-helpful vote; ErrNotFound if the
// question has no explanation yet.
func (s *Service) RecordFeedback(ctx context.Context, profileID, questionID uuid.UUID, helpful bool) error {
	explID, err := s.Q.GetExplanationTranslationForFeedback(ctx, questionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return s.Q.UpsertExplanationFeedback(ctx, sqlc.UpsertExplanationFeedbackParams{
		ProfileID:     profileID,
		ExplanationID: explID,
		Helpful:       helpful,
	})
}
