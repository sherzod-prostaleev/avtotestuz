package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrNotFound          = errors.New("session not found")
	ErrInvalidRequest    = errors.New("invalid session request")
	ErrDailyLimitReached = errors.New("daily practice limit reached")
	ErrAlreadyAnswered   = errors.New("question already answered in this session")
	ErrInvalidAnswer     = errors.New("answer does not belong to question")
	ErrSessionFinished   = errors.New("session already finished")
)

type Service struct {
	Q       *sqlc.Queries
	Billing billing.Service
}

func NewService(q *sqlc.Queries, b billing.Service) *Service {
	return &Service{Q: q, Billing: b}
}

func (s *Service) StartSession(ctx context.Context, profileID uuid.UUID, req StartRequest) (SessionView, error) {
	var (
		ids           []uuid.UUID
		timeLimit     pgtype.Int4
		errorsAllowed pgtype.Int4
		variantID     uuid.NullUUID
		categoryID    uuid.NullUUID
		signID        uuid.NullUUID
		err           error
	)

	switch req.Mode {
	case "variant":
		if req.VariantID == uuid.Nil {
			return SessionView{}, ErrInvalidRequest
		}
		ids, err = s.Q.ListVariantQuestionIDsOrdered(ctx, req.VariantID)
		variantID = uuid.NullUUID{UUID: req.VariantID, Valid: true}

	case "exam":
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(ExamQuestionCount))
		if err == nil && len(ids) < ExamQuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: ExamTimeLimitSec, Valid: true}
		errorsAllowed = pgtype.Int4{Int32: ExamErrorsAllowed, Valid: true}

	case "practice":
		if (req.CategoryID == uuid.Nil) == (req.SignID == uuid.Nil) {
			return SessionView{}, ErrInvalidRequest // exactly one must be set
		}
		count, dailyErr := s.clampToDailyAllowance(ctx, profileID, req.Count)
		if dailyErr != nil {
			return SessionView{}, dailyErr
		}
		if req.CategoryID != uuid.Nil {
			categoryID = uuid.NullUUID{UUID: req.CategoryID, Valid: true}
			ids, err = s.Q.RandomQuestionIDsByCategory(ctx, sqlc.RandomQuestionIDsByCategoryParams{
				CategoryID: req.CategoryID, LimitCount: int32(count),
			})
		} else {
			signID = uuid.NullUUID{UUID: req.SignID, Valid: true}
			ids, err = s.Q.RandomQuestionIDsBySign(ctx, sqlc.RandomQuestionIDsBySignParams{
				SignID: req.SignID, LimitCount: int32(count),
			})
		}

	case "mistakes":
		count := req.Count
		if count <= 0 {
			count = 10
		}
		ids, err = s.Q.ListMistakeBankQuestionIDs(ctx, sqlc.ListMistakeBankQuestionIDsParams{
			ProfileID: profileID, LimitCount: int32(count),
		})

	default:
		return SessionView{}, ErrInvalidRequest
	}
	if err != nil {
		return SessionView{}, err
	}

	row, err := s.Q.CreateExamSession(ctx, sqlc.CreateExamSessionParams{
		ProfileID:     profileID,
		Mode:          req.Mode,
		VariantID:     variantID,
		CategoryID:    categoryID,
		SignID:        signID,
		Locale:        req.Locale,
		TimeLimitSec:  timeLimit,
		ErrorsAllowed: errorsAllowed,
		Total:         int32(len(ids)),
	})
	if err != nil {
		return SessionView{}, err
	}

	view := SessionView{
		ID: row.ID, Mode: row.Mode, QuestionIDs: ids,
		Total: int(row.Total), StartedAt: row.StartedAt.Time,
	}
	if timeLimit.Valid {
		v := int(timeLimit.Int32)
		view.TimeLimitSec = &v
	}
	return view, nil
}

func (s *Service) clampToDailyAllowance(ctx context.Context, profileID uuid.UUID, requested int) (int, error) {
	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return 0, err
	}
	cfg, err := s.Q.GetLimitConfig(ctx, "daily_practice_questions")
	if err != nil {
		return 0, err
	}
	limit := int(cfg.FreeValue)
	if active {
		limit = int(cfg.VipValue)
	}
	if limit == -1 {
		if requested <= 0 {
			return 10, nil
		}
		return requested, nil
	}
	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)
	used, err := s.Q.CountPracticeAnswersToday(ctx, sqlc.CountPracticeAnswersTodayParams{
		ProfileID: profileID,
		Since:     pgtype.Timestamptz{Time: startOfDay, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	remaining := limit - int(used)
	if remaining < 0 {
		remaining = 0
	}
	if remaining == 0 {
		return 0, ErrDailyLimitReached
	}
	count := requested
	if count <= 0 || count > remaining {
		count = remaining
	}
	return count, nil
}
