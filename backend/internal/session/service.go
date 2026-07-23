package session

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/i18n"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
)

var (
	ErrNotFound            = errors.New("session not found")
	ErrInvalidRequest      = errors.New("invalid session request")
	ErrDailyLimitReached   = errors.New("daily practice limit reached")
	ErrAlreadyAnswered     = errors.New("question already answered in this session")
	ErrInvalidAnswer       = errors.New("answer does not belong to question")
	ErrQuestionNotAssigned = errors.New("question is not assigned to this session")
	ErrSessionFinished     = errors.New("session already finished")
	ErrRequiresVIP         = errors.New("active entitlement required")
	ErrVariantLocked       = errors.New("variant is locked")
)

type Service struct {
	Q        *sqlc.Queries
	Billing  billing.Service
	Learning *learning.Service
	Progress *progress.Service
	Now      func() time.Time
}

func NewService(q *sqlc.Queries, b billing.Service, l *learning.Service, p *progress.Service) *Service {
	return &Service{Q: q, Billing: b, Learning: l, Progress: p}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ResolveCategoryID accepts either a category UUID or its human-readable
// `code` — the only identifier content.CategoryDTO (GET /categories) ever
// exposes, since categories are never given a UUID over the wire — and
// resolves it to the category's UUID for use in a practice-mode
// StartRequest. A raw string that already parses as a UUID is trusted
// as-is (matching StartSession's prior behavior of not re-validating a
// caller-supplied UUID against the category table); anything else is
// looked up by code, and a code with no matching row surfaces as
// ErrNotFound (mirrored to the standard "not_found" response by
// writeSessionError, same as every other not-found path in this package).
func (s *Service) ResolveCategoryID(ctx context.Context, raw string) (uuid.UUID, error) {
	if id, err := uuid.Parse(raw); err == nil {
		return id, nil
	}
	id, err := s.Q.GetCategoryIDByCode(ctx, raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, ErrNotFound
		}
		return uuid.UUID{}, err
	}
	return id, nil
}

// ResolveSignID is ResolveCategoryID's counterpart for signs
// (content.SignDTOs / GET /signs), which likewise only ever expose a
// `code`.
func (s *Service) ResolveSignID(ctx context.Context, raw string) (uuid.UUID, error) {
	if id, err := uuid.Parse(raw); err == nil {
		return id, nil
	}
	id, err := s.Q.GetSignIDByCode(ctx, raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, ErrNotFound
		}
		return uuid.UUID{}, err
	}
	return id, nil
}

// ResolveVariantID is ResolveCategoryID/ResolveSignID's counterpart for
// variant-mode session starts. GET /variants (content.VariantListItemDTO)
// never exposes a bilet's UUID — only its human-readable `number` — so a
// real client can only send `variant_id: "12"`, not a UUID it doesn't have.
// A raw string that already parses as a UUID is trusted as-is (matching the
// prior behavior of not re-validating a caller-supplied UUID); otherwise it
// must parse as the variant's integer number, resolved to a UUID via the
// existing GetVariantByNumber query (already used by ListVariantStatuses,
// so no new sqlc query is needed here). Anything that is neither a valid
// UUID nor a valid integer, or a well-formed number with no matching
// variant, surfaces as ErrNotFound (mirrored to "not_found" by
// writeSessionError, same as ResolveCategoryID/ResolveSignID).
func (s *Service) ResolveVariantID(ctx context.Context, raw string) (uuid.UUID, error) {
	if id, err := uuid.Parse(raw); err == nil {
		return id, nil
	}
	num, err := strconv.Atoi(raw)
	if err != nil {
		return uuid.UUID{}, ErrNotFound
	}
	v, err := s.Q.GetVariantByNumber(ctx, int32(num))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, ErrNotFound
		}
		return uuid.UUID{}, err
	}
	return v.ID, nil
}

func (s *Service) StartSession(ctx context.Context, profileID uuid.UUID, req StartRequest) (SessionView, error) {
	if req.Locale == "" {
		req.Locale = i18n.Default
	} else if !i18n.Supported[req.Locale] {
		return SessionView{}, ErrInvalidRequest
	}

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
		v, verr := s.Q.GetVariantByID(ctx, req.VariantID)
		if verr != nil {
			return SessionView{}, verr
		}
		if v.Number > 1 {
			active, _, statusErr := s.Billing.Status(ctx, profileID)
			if statusErr != nil {
				return SessionView{}, statusErr
			}
			if !active {
				return SessionView{}, ErrRequiresVIP
			}
			previous, previousErr := s.Q.GetVariantByNumber(ctx, v.Number-1)
			if previousErr != nil {
				if errors.Is(previousErr, pgx.ErrNoRows) {
					return SessionView{}, ErrVariantLocked
				}
				return SessionView{}, previousErr
			}
			previousProgress, progressErr := s.Q.GetVariantProgress(ctx, sqlc.GetVariantProgressParams{
				ProfileID: profileID, VariantID: previous.ID,
			})
			if progressErr != nil {
				if errors.Is(progressErr, pgx.ErrNoRows) {
					return SessionView{}, ErrVariantLocked
				}
				return SessionView{}, progressErr
			}
			cfg, cfgErr := s.Q.GetLimitConfig(ctx, unlockThresholdConfigKey)
			if cfgErr != nil {
				return SessionView{}, cfgErr
			}
			if !IsVariantUnlocked(false, int(previousProgress.BestCorrect), int(cfg.FreeValue)) {
				return SessionView{}, ErrVariantLocked
			}
		}
		ids, err = s.Q.ListVariantQuestionIDsOrdered(ctx, req.VariantID)
		variantID = uuid.NullUUID{UUID: req.VariantID, Valid: true}

	case "exam":
		active, _, statusErr := s.Billing.Status(ctx, profileID)
		if statusErr != nil {
			return SessionView{}, statusErr
		}
		if !active {
			return SessionView{}, ErrRequiresVIP
		}
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(ExamQuestionCount))
		if err == nil && len(ids) < ExamQuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: ExamTimeLimitSec, Valid: true}
		errorsAllowed = pgtype.Int4{Int32: ExamErrorsAllowed, Valid: true}

	case "practice":
		// Exactly one selector: category, sign, or image presence.
		selectors := 0
		if req.CategoryID != uuid.Nil {
			selectors++
		}
		if req.SignID != uuid.Nil {
			selectors++
		}
		if req.HasImage != nil {
			selectors++
		}
		hasRange := req.VariantFrom > 0 || req.VariantTo > 0
		if hasRange {
			// A half-open or inverted span would silently widen or empty the
			// draw, so reject it rather than guessing what was meant.
			if req.VariantFrom <= 0 || req.VariantTo <= 0 || req.VariantFrom > req.VariantTo {
				return SessionView{}, ErrInvalidRequest
			}
			selectors++
		}
		if selectors != 1 {
			return SessionView{}, ErrInvalidRequest
		}
		count, dailyErr := s.clampToDailyAllowance(ctx, profileID, req.Count)
		if dailyErr != nil {
			return SessionView{}, dailyErr
		}
		switch {
		case req.CategoryID != uuid.Nil:
			categoryID = uuid.NullUUID{UUID: req.CategoryID, Valid: true}
			ids, err = s.Q.RandomQuestionIDsByCategory(ctx, sqlc.RandomQuestionIDsByCategoryParams{
				CategoryID: req.CategoryID, LimitCount: int32(count),
			})
		case req.SignID != uuid.Nil:
			signID = uuid.NullUUID{UUID: req.SignID, Valid: true}
			ids, err = s.Q.RandomQuestionIDsBySign(ctx, sqlc.RandomQuestionIDsBySignParams{
				SignID: req.SignID, LimitCount: int32(count),
			})
		case hasRange:
			ids, err = s.Q.RandomQuestionIDsByVariantRange(ctx, sqlc.RandomQuestionIDsByVariantRangeParams{
				FromNumber: int32(req.VariantFrom), ToNumber: int32(req.VariantTo), LimitCount: int32(count),
			})
		default:
			ids, err = s.Q.RandomQuestionIDsByImagePresence(ctx, sqlc.RandomQuestionIDsByImagePresenceParams{
				HasImage: *req.HasImage, LimitCount: int32(count),
			})
		}

	case "mistakes":
		active, _, statusErr := s.Billing.Status(ctx, profileID)
		if statusErr != nil {
			return SessionView{}, statusErr
		}
		if !active {
			return SessionView{}, ErrRequiresVIP
		}
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
		QuestionIds:   ids,
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

// Allowance is today's practice budget for one profile.
type Allowance struct {
	Unlimited bool
	Limit     int
	Used      int
	Remaining int
}

// PracticeAllowance reports the budget without consuming it, so the client can
// show what a chosen session size will really deliver before starting one.
func (s *Service) PracticeAllowance(ctx context.Context, profileID uuid.UUID) (Allowance, error) {
	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return Allowance{}, err
	}
	cfg, err := s.Q.GetLimitConfig(ctx, "daily_practice_questions")
	if err != nil {
		return Allowance{}, err
	}
	limit := int(cfg.FreeValue)
	if active {
		limit = int(cfg.VipValue)
	}
	if limit == -1 {
		return Allowance{Unlimited: true}, nil
	}
	used, err := s.practiceAnswersToday(ctx, profileID)
	if err != nil {
		return Allowance{}, err
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return Allowance{Limit: limit, Used: used, Remaining: remaining}, nil
}

func (s *Service) practiceAnswersToday(ctx context.Context, profileID uuid.UUID) (int, error) {
	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)
	used, err := s.Q.CountPracticeAnswersToday(ctx, sqlc.CountPracticeAnswersTodayParams{
		ProfileID: profileID,
		Since:     pgtype.Timestamptz{Time: startOfDay, Valid: true},
	})
	return int(used), err
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
	used, err := s.practiceAnswersToday(ctx, profileID)
	if err != nil {
		return 0, err
	}
	remaining := limit - used
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
	if row.Mode == "exam" && row.TimeLimitSec.Valid &&
		!s.now().Before(row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32)*time.Second)) {
		finished, finishErr := s.finishInternal(ctx, row, false, true)
		if finishErr != nil {
			return AnswerResult{}, finishErr
		}
		return AnswerResult{Stopped: true, StopReason: finished.StoppedReason}, nil
	}

	assigned, err := s.Q.GetSessionQuestion(ctx, sqlc.GetSessionQuestionParams{
		SessionID: sessionID, QuestionID: questionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, ErrQuestionNotAssigned
		}
		return AnswerResult{}, err
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

	var explanation *ExplanationPayload
	if row.Mode != "exam" {
		expl, explErr := s.Q.GetVerifiedExplanation(ctx, sqlc.GetVerifiedExplanationParams{
			QuestionID: questionID, Locale: row.Locale,
		})
		switch {
		case explErr == nil:
			explanation = &ExplanationPayload{LegalRefs: expl.LegalRefs, Blocks: expl.Blocks}
		case errors.Is(explErr, pgx.ErrNoRows):
			// Explanations are optional; accepted answers remain successful.
		default:
			return AnswerResult{}, explErr
		}
	}

	if _, err := s.Q.InsertSessionAnswer(ctx, sqlc.InsertSessionAnswerParams{
		SessionID:  sessionID,
		QuestionID: questionID,
		AnswerID:   answerID,
		IsCorrect:  ans.IsCorrect,
		Position:   assigned.Position,
	}); err != nil {
		return AnswerResult{}, err
	}

	rating := learning.Good
	if !ans.IsCorrect {
		rating = learning.Again
	}
	if _, err := s.Learning.RecordReview(ctx, profileID, questionID, rating); err != nil {
		return AnswerResult{}, err
	}
	if _, err := s.Progress.RecordActivity(ctx, profileID); err != nil {
		return AnswerResult{}, err
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
	return AnswerResult{
		Recorded: true, Correct: &correct, CorrectAnswerID: &correctID, Explanation: explanation,
	}, nil
}

// GetSessionQuestionAccess authorizes a session-scoped question read and
// computes what grading feedback can be exposed. A caller outside the owning
// profile, or a question outside the persisted assignment, receives
// ErrNotFound so the endpoint does not reveal session membership.
func (s *Service) GetSessionQuestionAccess(ctx context.Context, profileID, sessionID, questionID uuid.UUID) (SessionQuestionAccess, error) {
	row, err := s.Q.GetExamSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionQuestionAccess{}, ErrNotFound
		}
		return SessionQuestionAccess{}, err
	}
	if row.ProfileID != profileID {
		return SessionQuestionAccess{}, ErrNotFound
	}

	assigned, err := s.Q.GetSessionQuestion(ctx, sqlc.GetSessionQuestionParams{
		SessionID: sessionID, QuestionID: questionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionQuestionAccess{}, ErrNotFound
		}
		return SessionQuestionAccess{}, err
	}

	access := SessionQuestionAccess{Position: int(assigned.Position)}
	answer, answerErr := s.Q.GetSessionAnswer(ctx, sqlc.GetSessionAnswerParams{
		SessionID: sessionID, QuestionID: questionID,
	})
	switch {
	case answerErr == nil:
		access.Answered = true
		userAnswerID := answer.AnswerID
		access.UserAnswerID = &userAnswerID
	case errors.Is(answerErr, pgx.ErrNoRows):
		// The assignment exists but is unanswered.
	default:
		return SessionQuestionAccess{}, answerErr
	}

	if row.Mode == "exam" {
		access.FeedbackAllowed = row.Status != "in_progress"
	} else {
		access.FeedbackAllowed = access.Answered
	}
	if !access.FeedbackAllowed {
		return access, nil
	}

	correctAnswerID, err := s.Q.GetCorrectAnswerID(ctx, questionID)
	if err != nil {
		return SessionQuestionAccess{}, err
	}
	access.CorrectAnswerID = &correctAnswerID
	if access.Answered {
		correct := answer.IsCorrect
		access.Correct = &correct
	}
	return access, nil
}

// unlockThresholdConfigKey is the limit_config key holding the minimum
// correct-answer count a variant-mode session must reach to unlock the next
// bilet. Free and VIP tiers currently share the same value (10), so
// FinishSession reads FreeValue without an extra billing lookup.
const unlockThresholdConfigKey = "unlock_threshold_correct"

// FinishSession finishes an in-progress session — computing its final
// score/status, persisting it, and (for variant mode) upserting bilet-unlock
// progress. It is idempotent: finishing an already-finished session returns
// the stored result with no additional writes.
func (s *Service) FinishSession(ctx context.Context, profileID, sessionID uuid.UUID) (FinishResult, error) {
	row, err := s.Q.GetExamSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FinishResult{}, ErrNotFound
		}
		return FinishResult{}, err
	}
	if row.ProfileID != profileID {
		return FinishResult{}, ErrNotFound
	}

	timedOut := false
	if row.Mode == "exam" && row.TimeLimitSec.Valid {
		timedOut = !s.now().Before(row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32) * time.Second))
	}
	return s.finishInternal(ctx, row, false, timedOut)
}

// finishInternal marks a session as finished. It is the shared landing point
// for both SubmitAnswer's 3rd-mistake stop (tooManyErrors=true) and the
// public FinishSession (manual finish / time-up, tooManyErrors=false). It is
// idempotent — a session that's no longer "in_progress" is returned as-is,
// with no further writes (in particular, no double bilet-unlock upsert).
func (s *Service) finishInternal(ctx context.Context, row sqlc.ExamSession, tooManyErrors, timedOut bool) (FinishResult, error) {
	if row.Status != "in_progress" {
		return FinishResult{
			Status:        row.Status,
			StoppedReason: row.StoppedReason.String,
			Score:         int(row.Score.Int32),
			Total:         int(row.Total),
		}, nil
	}

	counts, err := s.Q.CountSessionAnswers(ctx, row.ID)
	if err != nil {
		return FinishResult{}, err
	}
	totalAnswered := int(counts.TotalAnswered)
	correctCount := int(counts.CorrectCount)

	var status, reason string
	switch row.Mode {
	case "exam":
		wrong := totalAnswered - correctCount
		outcome := EvaluateExam(correctCount, wrong, int(row.Total), timedOut, tooManyErrors)
		status = outcome.Status
		reason = outcome.StoppedReason
	default: // "variant", "practice", "mistakes"
		status = "passed"
		if totalAnswered < int(row.Total) {
			status = "abandoned"
		}
		reason = "completed"
		if status == "abandoned" {
			reason = ""
		}
	}

	stoppedReason := pgtype.Text{}
	if reason != "" {
		stoppedReason = pgtype.Text{String: reason, Valid: true}
	}

	updated, err := s.Q.FinishExamSession(ctx, sqlc.FinishExamSessionParams{
		ID:            row.ID,
		Status:        status,
		Score:         pgtype.Int4{Int32: int32(correctCount), Valid: true},
		StoppedReason: stoppedReason,
	})
	if err != nil {
		return FinishResult{}, err
	}

	if row.Mode == "variant" {
		cfg, err := s.Q.GetLimitConfig(ctx, unlockThresholdConfigKey)
		if err != nil {
			return FinishResult{}, err
		}
		var completedAt pgtype.Timestamptz
		if correctCount >= int(cfg.FreeValue) {
			completedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}
		if _, err := s.Q.UpsertVariantProgress(ctx, sqlc.UpsertVariantProgressParams{
			ProfileID:   row.ProfileID,
			VariantID:   row.VariantID.UUID,
			BestCorrect: int32(correctCount),
			CompletedAt: completedAt,
		}); err != nil {
			return FinishResult{}, err
		}
	}

	return FinishResult{
		Status:        updated.Status,
		StoppedReason: reason,
		Score:         correctCount,
		Total:         int(row.Total),
	}, nil
}

// GetSession returns the resume/history view of a single session, scoped to
// its owning profile (ErrNotFound otherwise, matching SubmitAnswer/
// FinishSession). Per-answer correctness is redacted (Correct left nil) for
// every answer while an exam-mode session is still in_progress; once the
// session is no longer in_progress (any other status, any mode), full
// correctness is reported for every answer.
func (s *Service) GetSession(ctx context.Context, profileID, sessionID uuid.UUID) (SessionDetail, error) {
	row, err := s.Q.GetExamSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionDetail{}, ErrNotFound
		}
		return SessionDetail{}, err
	}
	if row.ProfileID != profileID {
		return SessionDetail{}, ErrNotFound
	}

	rows, err := s.Q.ListSessionQuestionsWithAnswers(ctx, sessionID)
	if err != nil {
		return SessionDetail{}, err
	}

	redact := row.Mode == "exam" && row.Status == "in_progress"

	answers := make([]AnsweredQuestion, 0, len(rows))
	questionIDs := make([]uuid.UUID, 0, len(rows))
	for _, a := range rows {
		aq := AnsweredQuestion{
			QuestionID: a.QuestionID,
			Position:   int(a.Position),
			Answered:   a.UserAnswerID.Valid,
		}
		questionIDs = append(questionIDs, a.QuestionID)
		if a.UserAnswerID.Valid {
			userAnswerID := a.UserAnswerID.UUID
			aq.UserAnswerID = &userAnswerID
		}
		if !redact && a.IsCorrect.Valid {
			correct := a.IsCorrect.Bool
			aq.Correct = &correct
		}
		// Feedback modes reveal the answer key only after this particular
		// question is answered. A completed session may reveal it for every
		// assigned question, including questions skipped before an early stop.
		if !redact && a.CorrectAnswerID.Valid && (aq.Answered || row.Status != "in_progress") {
			correctAnswerID := a.CorrectAnswerID.UUID
			aq.CorrectAnswerID = &correctAnswerID
		}
		answers = append(answers, aq)
	}

	detail := SessionDetail{
		SessionView: SessionView{
			ID:          row.ID,
			Mode:        row.Mode,
			QuestionIDs: questionIDs,
			Total:       int(row.Total),
			StartedAt:   row.StartedAt.Time,
		},
		Status:        row.Status,
		StoppedReason: row.StoppedReason.String,
		Answers:       answers,
	}
	if row.TimeLimitSec.Valid {
		v := int(row.TimeLimitSec.Int32)
		detail.TimeLimitSec = &v
	}
	if row.Score.Valid {
		v := int(row.Score.Int32)
		detail.Score = &v
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		detail.FinishedAt = &t
	}
	return detail, nil
}

// ListMySessions returns the profile's session history, most recent first,
// capped at limit (defaulting to 20 when limit <= 0).
func (s *Service) ListMySessions(ctx context.Context, profileID uuid.UUID, limit int) ([]SessionSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.Q.ListMySessions(ctx, sqlc.ListMySessionsParams{
		ProfileID:  profileID,
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	summaries := make([]SessionSummary, 0, len(rows))
	for _, row := range rows {
		summary := SessionSummary{
			ID:        row.ID,
			Mode:      row.Mode,
			Status:    row.Status,
			Total:     int(row.Total),
			StartedAt: row.StartedAt.Time,
		}
		if row.Score.Valid {
			v := int(row.Score.Int32)
			summary.Score = &v
		}
		if row.FinishedAt.Valid {
			t := row.FinishedAt.Time
			summary.FinishedAt = &t
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// ListVariantStatuses returns every bilet variant, in number order, with the
// profile's progress against it and whether it's unlocked. A variant is
// unlocked if it's the first one, or if the *previous* variant's best_correct
// meets the configured unlock threshold — computed via the pure
// IsVariantUnlocked rule (rules.go), never reimplemented here.
func (s *Service) ListVariantStatuses(ctx context.Context, profileID uuid.UUID) ([]VariantStatus, error) {
	variants, err := s.Q.ListVariants(ctx)
	if err != nil {
		return nil, err
	}

	progressRows, err := s.Q.ListVariantProgressForProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	progressByVariant := make(map[uuid.UUID]sqlc.VariantProgress, len(progressRows))
	for _, p := range progressRows {
		progressByVariant[p.VariantID] = p
	}

	cfg, err := s.Q.GetLimitConfig(ctx, unlockThresholdConfigKey)
	if err != nil {
		return nil, err
	}
	threshold := int(cfg.FreeValue)

	statuses := make([]VariantStatus, 0, len(variants))
	prevBestCorrect := 0
	for i, v := range variants {
		variant, err := s.Q.GetVariantByNumber(ctx, v.Number)
		if err != nil {
			return nil, err
		}

		status := VariantStatus{
			Number:        v.Number,
			QuestionCount: int(v.QuestionCount),
			Unlocked:      IsVariantUnlocked(i == 0, prevBestCorrect, threshold),
		}

		bestCorrect := 0
		if p, ok := progressByVariant[variant.ID]; ok {
			status.BestCorrect = int(p.BestCorrect)
			status.Attempts = int(p.Attempts)
			if p.CompletedAt.Valid {
				t := p.CompletedAt.Time
				status.CompletedAt = &t
			}
			bestCorrect = int(p.BestCorrect)
		}
		prevBestCorrect = bestCorrect

		statuses = append(statuses, status)
	}
	return statuses, nil
}
