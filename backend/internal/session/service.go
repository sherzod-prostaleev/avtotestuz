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
	ErrNotFound          = errors.New("session not found")
	ErrInvalidRequest    = errors.New("invalid session request")
	ErrDailyLimitReached = errors.New("daily practice limit reached")
	ErrAlreadyAnswered   = errors.New("question already answered in this session")
	ErrInvalidAnswer     = errors.New("answer does not belong to question")
	ErrSessionFinished   = errors.New("session already finished")
	ErrRequiresVIP       = errors.New("active entitlement required")
)

type Service struct {
	Q        *sqlc.Queries
	Billing  billing.Service
	Learning *learning.Service
	Progress *progress.Service
}

func NewService(q *sqlc.Queries, b billing.Service, l *learning.Service, p *progress.Service) *Service {
	return &Service{Q: q, Billing: b, Learning: l, Progress: p}
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
	return AnswerResult{Recorded: true, Correct: &correct, CorrectAnswerID: &correctID}, nil
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
		timedOut = time.Now().After(row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32) * time.Second))
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

	rows, err := s.Q.ListSessionAnswers(ctx, sessionID)
	if err != nil {
		return SessionDetail{}, err
	}

	redact := row.Mode == "exam" && row.Status == "in_progress"

	answers := make([]AnsweredQuestion, 0, len(rows))
	for _, a := range rows {
		aq := AnsweredQuestion{
			QuestionID: a.QuestionID,
			Position:   int(a.Position),
			Answered:   true,
		}
		if !redact {
			correct := a.IsCorrect
			aq.Correct = &correct
		}
		answers = append(answers, aq)
	}

	detail := SessionDetail{
		SessionView: SessionView{
			ID:        row.ID,
			Mode:      row.Mode,
			Total:     int(row.Total),
			StartedAt: row.StartedAt.Time,
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
