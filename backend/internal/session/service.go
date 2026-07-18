package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// SubmitAnswer records a single answer against an in-progress session,
// applies mistake-bank side effects, and — for exam mode — withholds
// correctness feedback until the exam finishes (either by hitting the 3rd
// mistake here, or later via FinishSession).
func (s *Service) SubmitAnswer(ctx context.Context, profileID, sessionID, questionID, answerID uuid.UUID) (AnswerResult, error) {
	row, err := s.Q.GetExamSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, ErrNotFound
		}
		return AnswerResult{}, err
	}
	if row.ProfileID != profileID {
		return AnswerResult{}, ErrNotFound
	}
	if row.Status != "in_progress" {
		return AnswerResult{}, ErrSessionFinished
	}

	_, err = s.Q.GetSessionAnswer(ctx, sqlc.GetSessionAnswerParams{SessionID: sessionID, QuestionID: questionID})
	if err == nil {
		return AnswerResult{}, ErrAlreadyAnswered
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AnswerResult{}, err
	}

	ans, err := s.Q.GetAnswerForScoring(ctx, sqlc.GetAnswerForScoringParams{ID: answerID, QuestionID: questionID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, ErrInvalidAnswer
		}
		return AnswerResult{}, err
	}

	before, err := s.Q.CountSessionAnswers(ctx, sessionID)
	if err != nil {
		return AnswerResult{}, err
	}
	position := int16(before.TotalAnswered + 1)

	if _, err := s.Q.InsertSessionAnswer(ctx, sqlc.InsertSessionAnswerParams{
		SessionID:  sessionID,
		QuestionID: questionID,
		AnswerID:   answerID,
		IsCorrect:  ans.IsCorrect,
		Position:   position,
	}); err != nil {
		return AnswerResult{}, err
	}

	if row.Mode == "mistakes" {
		if ans.IsCorrect {
			if _, err := s.Q.MarkQuestionCorrectInMistakesMode(ctx, sqlc.MarkQuestionCorrectInMistakesModeParams{
				ClearAfter: int32(MistakeClearAfter), ProfileID: profileID, QuestionID: questionID,
			}); err != nil {
				return AnswerResult{}, err
			}
		} else {
			if _, err := s.Q.MarkQuestionWrong(ctx, sqlc.MarkQuestionWrongParams{ProfileID: profileID, QuestionID: questionID}); err != nil {
				return AnswerResult{}, err
			}
		}
	} else if !ans.IsCorrect {
		// Any wrong answer, in any mode, feeds the mistake bank.
		if _, err := s.Q.MarkQuestionWrong(ctx, sqlc.MarkQuestionWrongParams{ProfileID: profileID, QuestionID: questionID}); err != nil {
			return AnswerResult{}, err
		}
	}

	if row.Mode == "exam" {
		after, err := s.Q.CountSessionAnswers(ctx, sessionID)
		if err != nil {
			return AnswerResult{}, err
		}
		wrongSoFar := int(after.TotalAnswered - after.CorrectCount)
		if ShouldStopExam(wrongSoFar) {
			if _, err := s.finishInternal(ctx, row, true, false); err != nil {
				return AnswerResult{}, err
			}
			return AnswerResult{Recorded: true, Stopped: true, StopReason: "too_many_errors"}, nil
		}
		return AnswerResult{Recorded: true}, nil
	}

	correctID := answerID
	if !ans.IsCorrect {
		correctID, err = s.Q.GetCorrectAnswerID(ctx, questionID)
		if err != nil {
			return AnswerResult{}, err
		}
	}
	correct := ans.IsCorrect
	return AnswerResult{Recorded: true, Correct: &correct, CorrectAnswerID: &correctID}, nil
}

// FinishResult is the minimal outcome shape finishInternal produces today.
// Task 5's FinishSession will flesh this out with the full scoring payload
// (correct/wrong counts, bilet unlock, variant/mistake-bank progress); Task 4
// only needs enough to know the session was marked finished.
type FinishResult struct {
	Status        string
	StoppedReason string
}

// finishInternal marks an exam-mode session as finished. It is the shared
// landing point for both SubmitAnswer's 3rd-mistake stop (this task) and
// Task 5's public FinishSession (manual finish / time-up). Task 4 only
// wires the tooManyErrors=true path; Task 5 extends this function with the
// full pass/fail computation (EvaluateExam), idempotency guard for a
// session that's already finished, and the variant/practice/mistakes
// finish branches (variant progress upsert, bilet unlock, etc.) — the
// timedOut parameter is already threaded through for that purpose even
// though this task never sets it true.
func (s *Service) finishInternal(ctx context.Context, row sqlc.ExamSession, tooManyErrors, timedOut bool) (FinishResult, error) {
	_ = timedOut // reserved for Task 5's EvaluateExam branching

	status := "failed"
	reason := "too_many_errors"

	updated, err := s.Q.FinishExamSession(ctx, sqlc.FinishExamSessionParams{
		ID:            row.ID,
		Status:        status,
		StoppedReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		return FinishResult{}, err
	}
	return FinishResult{Status: updated.Status, StoppedReason: reason}, nil
}
