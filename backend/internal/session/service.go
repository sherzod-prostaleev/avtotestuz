package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/i18n"
	"avtotest.uz/backend/internal/leaderboard"
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
	ErrMockNotEligible     = errors.New("grand mock study requirements not met")
	ErrNothingDue          = errors.New("no due reviews")
)

type Service struct {
	Q           *sqlc.Queries
	Pool        *pgxpool.Pool
	Billing     billing.Service
	Learning    *learning.Service
	Progress    *progress.Service
	Leaderboard *leaderboard.Service // optional; nil-safe, see SubmitAnswer
	Now         func() time.Time
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, b billing.Service, l *learning.Service, p *progress.Service) *Service {
	return &Service{Q: q, Pool: pool, Billing: b, Learning: l, Progress: p}
}

// transactional returns a service whose related persistence components all
// share one pgx transaction. Optional collaborators keep their original
// enablement semantics (notably Billing in tests).
func (s *Service) transactional(q *sqlc.Queries) *Service {
	l := learning.NewService(q)
	p := progress.NewService(q)
	p.Learning = l
	if s.Progress != nil && s.Progress.Billing.Q != nil {
		p.Billing = s.Progress.Billing
		p.Billing.Q = q
	}
	b := s.Billing
	b.Q = q
	return &Service{
		Q: q, Pool: s.Pool, Billing: b, Learning: l, Progress: p,
		Leaderboard: s.Leaderboard, Now: s.Now,
	}
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
// must parse as the variant's integer number, resolved to a UUID via
// GetVariantByNumber. Anything that is neither a valid
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
			// QA/ops profiles with bypass_variant_progress skip sequential
			// unlock; VIP entitlement is still required above.
			bypass, bypassErr := s.bypassVariantProgress(ctx, profileID)
			if bypassErr != nil {
				return SessionView{}, bypassErr
			}
			if !bypass {
				prev, prevErr := s.Q.GetVariantByNumber(ctx, v.Number-1)
				if prevErr != nil {
					return SessionView{}, prevErr
				}
				prevProgress, progErr := s.Q.GetVariantProgress(ctx, sqlc.GetVariantProgressParams{
					ProfileID: profileID,
					VariantID: prev.ID,
				})
				if progErr != nil {
					if errors.Is(progErr, pgx.ErrNoRows) {
						return SessionView{}, ErrVariantLocked
					}
					return SessionView{}, progErr
				}
				if !prevProgress.CompletedAt.Valid {
					return SessionView{}, ErrVariantLocked
				}
			}
		}
		ids, err = s.Q.ListVariantQuestionIDsOrdered(ctx, req.VariantID)
		variantID = uuid.NullUUID{UUID: req.VariantID, Valid: true}

	case "exam":
		// The size decides the whole rule set (questions, minutes, mistake
		// budget) and is whitelisted here, before any work: an open-ended
		// count would let a client request a 3-question "exam" and pass it.
		cfg, ok := ExamConfigFor(req.Count)
		if !ok {
			return SessionView{}, ErrInvalidRequest
		}
		active, _, statusErr := s.Billing.Status(ctx, profileID)
		if statusErr != nil {
			return SessionView{}, statusErr
		}
		if !active {
			return SessionView{}, ErrRequiresVIP
		}
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(cfg.QuestionCount))
		if err == nil && len(ids) < cfg.QuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: int32(cfg.TimeLimitSec), Valid: true}
		errorsAllowed = pgtype.Int4{Int32: int32(cfg.ErrorsAllowed), Valid: true}

	case "grand_mock":
		// Delegating to MockEligibility rather than repeating its checks is
		// what guarantees the card's displayed reason and the server's refusal
		// can never disagree — they are now the same code path, not two
		// switches that have to be kept in the same order by hand.
		elig, eligErr := s.MockEligibility(ctx, profileID)
		if eligErr != nil {
			return SessionView{}, eligErr
		}
		if !elig.Eligible {
			// A missing subscription is reported as ErrRequiresVIP (402
			// vip_required) rather than ErrMockNotEligible, because the client
			// already routes vip_required to the paywall. Collapsing it into
			// the generic mock error sent a non-VIP user who reached
			// /session/start?mode=grand_mock — via a stale card, a
			// subscription that lapsed mid-session, or a shared link — to an
			// unexplained error screen and AWAY from /premium.
			if elig.Reason == MockReasonVIPRequired {
				return SessionView{}, ErrRequiresVIP
			}
			return SessionView{}, ErrMockNotEligible
		}
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(ExamQuestionCount))
		if err == nil && len(ids) < ExamQuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: ExamTimeLimitSec, Valid: true}
		errorsAllowed = pgtype.Int4{Int32: ExamErrorsAllowed, Valid: true}

	case "placement":
		// Free diagnostic: no VIP gate. Records FSRS so placement seeds memory.
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(PlacementQuestionCount))
		if err == nil && len(ids) < PlacementQuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: PlacementTimeLimitSec, Valid: true}
		errorsAllowed = pgtype.Int4{Int32: PlacementErrorsAllowed, Valid: true}

	case "practice":
		// Exactly one selector: category, sign, variant range, or image presence.
		// "Hammasi" is a count choice (large LIMIT), not a missing selector.
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

	case "review":
		// FSRS due queue (learning.NextDue) — not VIP-gated; distinct from the
		// mistakes bank which requires lapses > 0.
		if s.Learning == nil {
			return SessionView{}, ErrInvalidRequest
		}
		count := req.Count
		if count <= 0 {
			count = 20
		}
		ids, err = s.Learning.NextDue(ctx, profileID, count)
		if err == nil && len(ids) == 0 {
			return SessionView{}, ErrNothingDue
		}

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
	if errorsAllowed.Valid {
		v := int(errorsAllowed.Int32)
		view.ErrorsAllowed = &v
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

// SubmitAnswer records a single answer against an in-progress session and
// applies mistake-bank side effects. Exam-like modes return per-answer
// correct/wrong feedback immediately (official Avtotest green/red UI) and
// may stop the session on the 3rd mistake.
//
// FSRS side effects (unless opts.SkipFSRS): incorrect → Again; exam-like
// correct → Good; practice-style correct → FSRSRatingForCorrect(rating, latency).
func (s *Service) SubmitAnswer(ctx context.Context, profileID, sessionID, questionID, answerID uuid.UUID, opts SubmitAnswerOpts) (AnswerResult, error) {
	if s.Pool == nil {
		return AnswerResult{}, errors.New("session transaction pool is not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return AnswerResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize every answer/finish mutation for one session. This prevents an
	// answer from being inserted after a concurrent finish and makes the
	// duplicate-answer check reliable before any FSRS/streak side effect.
	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM exam_session WHERE id = $1 FOR UPDATE`, sessionID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, ErrNotFound
		}
		return AnswerResult{}, err
	}
	q := sqlc.New(tx)
	txSvc := s.transactional(q)
	row, err := q.GetExamSession(ctx, sessionID)
	if err != nil {
		return AnswerResult{}, err
	}
	if row.ProfileID != profileID {
		return AnswerResult{}, ErrNotFound
	}
	if row.Status != "in_progress" {
		return AnswerResult{}, ErrSessionFinished
	}
	if IsExamLike(row.Mode) && row.TimeLimitSec.Valid &&
		!s.now().Before(row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32)*time.Second)) {
		finished, finishErr := txSvc.finishInternal(ctx, row, false, true)
		if finishErr != nil {
			return AnswerResult{}, finishErr
		}
		if err := tx.Commit(ctx); err != nil {
			return AnswerResult{}, err
		}
		return AnswerResult{Stopped: true, StopReason: finished.StoppedReason}, nil
	}

	assigned, err := q.GetSessionQuestion(ctx, sqlc.GetSessionQuestionParams{
		SessionID: sessionID, QuestionID: questionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, ErrQuestionNotAssigned
		}
		return AnswerResult{}, err
	}
	if _, err = q.GetSessionAnswer(ctx, sqlc.GetSessionAnswerParams{
		SessionID: sessionID, QuestionID: questionID,
	}); err == nil {
		return AnswerResult{}, ErrAlreadyAnswered
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AnswerResult{}, err
	}

	ans, err := q.GetAnswerForScoring(ctx, sqlc.GetAnswerForScoringParams{
		ID: answerID, QuestionID: questionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AnswerResult{}, ErrInvalidAnswer
		}
		return AnswerResult{}, err
	}

	var explanation *ExplanationPayload
	if !IsExamLike(row.Mode) {
		expl, explErr := q.GetVerifiedExplanation(ctx, sqlc.GetVerifiedExplanationParams{
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

	if _, err := q.InsertSessionAnswer(ctx, sqlc.InsertSessionAnswerParams{
		SessionID: sessionID, QuestionID: questionID, AnswerID: answerID,
		IsCorrect: ans.IsCorrect, Position: assigned.Position,
	}); err != nil {
		return AnswerResult{}, err
	}
	if !opts.SkipFSRS {
		var rating learning.Rating
		switch {
		case !ans.IsCorrect:
			rating = learning.Again
		case IsExamLike(row.Mode):
			rating = learning.Good
		default:
			latencyMs := 0
			if opts.LatencyMs != nil {
				latencyMs = *opts.LatencyMs
			}
			rating = FSRSRatingForCorrect(opts.Rating, latencyMs)
		}
		if _, err := txSvc.Learning.RecordReview(ctx, profileID, questionID, rating); err != nil {
			return AnswerResult{}, err
		}
	}
	if _, err := txSvc.Progress.RecordActivity(ctx, profileID); err != nil {
		return AnswerResult{}, err
	}

	var result AnswerResult
	if IsExamLike(row.Mode) {
		examCorrectID := answerID
		if !ans.IsCorrect {
			examCorrectID, err = q.GetCorrectAnswerID(ctx, questionID)
			if err != nil {
				return AnswerResult{}, err
			}
		}
		examCorrect := ans.IsCorrect
		result = AnswerResult{Recorded: true, Correct: &examCorrect, CorrectAnswerID: &examCorrectID}

		after, err := q.CountSessionAnswers(ctx, sessionID)
		if err != nil {
			return AnswerResult{}, err
		}
		wrongSoFar := int(after.TotalAnswered - after.CorrectCount)
		errorsAllowed := ExamErrorsAllowed
		if row.ErrorsAllowed.Valid {
			errorsAllowed = int(row.ErrorsAllowed.Int32)
		}
		if ShouldStopForErrors(wrongSoFar, errorsAllowed) {
			if _, err := txSvc.finishInternal(ctx, row, true, false); err != nil {
				return AnswerResult{}, err
			}
			result.Stopped = true
			result.StopReason = "too_many_errors"
		}
	} else {
		correctID := answerID
		if !ans.IsCorrect {
			correctID, err = q.GetCorrectAnswerID(ctx, questionID)
			if err != nil {
				return AnswerResult{}, err
			}
		}
		correct := ans.IsCorrect
		result = AnswerResult{
			Recorded: true, Correct: &correct, CorrectAnswerID: &correctID, Explanation: explanation,
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return AnswerResult{}, err
	}
	// Leaderboard is intentionally outside the source-of-truth transaction:
	// it is a reconstructable Redis projection, while answer/FSRS/streak are
	// committed together above.
	if ans.IsCorrect && s.Leaderboard != nil {
		_ = s.Leaderboard.RecordPoint(ctx, profileID)
	}
	return result, nil
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

	finished := row.Status != "in_progress"
	if IsExamLike(row.Mode) {
		// Green/red grades for answered questions during the exam; explanations
		// and unanswered answer keys stay sealed until finish.
		access.ExplanationAllowed = finished
	} else {
		access.ExplanationAllowed = access.Answered
	}

	gradeAllowed := access.Answered || finished
	if !gradeAllowed {
		return access, nil
	}

	correctAnswerID, err := s.Q.GetCorrectAnswerID(ctx, questionID)
	if err != nil {
		return SessionQuestionAccess{}, err
	}
	if access.Answered || finished {
		access.CorrectAnswerID = &correctAnswerID
	}
	if access.Answered {
		correct := answer.IsCorrect
		access.Correct = &correct
	}
	return access, nil
}

// ListSessionQuestionAccesses authorizes a session-scoped batch read and
// returns every assigned question's disclosure decision in position order.
// Ownership and missing sessions surface as ErrNotFound, matching
// GetSessionQuestionAccess so the list endpoint cannot probe membership.
func (s *Service) ListSessionQuestionAccesses(ctx context.Context, profileID, sessionID uuid.UUID) ([]SessionQuestionAccessItem, error) {
	row, err := s.Q.GetExamSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.ProfileID != profileID {
		return nil, ErrNotFound
	}

	assigned, err := s.Q.ListSessionQuestionsWithAnswers(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	finished := row.Status != "in_progress"
	out := make([]SessionQuestionAccessItem, 0, len(assigned))
	for _, a := range assigned {
		item := SessionQuestionAccessItem{QuestionID: a.QuestionID}
		item.Position = int(a.Position)
		if a.UserAnswerID.Valid {
			item.Answered = true
			userAnswerID := a.UserAnswerID.UUID
			item.UserAnswerID = &userAnswerID
		}
		if IsExamLike(row.Mode) {
			item.ExplanationAllowed = finished
		} else {
			item.ExplanationAllowed = item.Answered
		}

		gradeAllowed := item.Answered || finished
		if !gradeAllowed {
			out = append(out, item)
			continue
		}
		if !a.CorrectAnswerID.Valid {
			return nil, fmt.Errorf("session %s question %s missing correct_answer_id", sessionID, a.QuestionID)
		}
		correctAnswerID := a.CorrectAnswerID.UUID
		item.CorrectAnswerID = &correctAnswerID
		if item.Answered && a.IsCorrect.Valid {
			correct := a.IsCorrect.Bool
			item.Correct = &correct
		}
		out = append(out, item)
	}
	return out, nil
}

// unlockThresholdConfigKey is the limit_config key holding the minimum
// correct-answer count a variant-mode session must reach to mark the bilet
// completed (completed_at). Completing a bilet unlocks the next one for VIP
// profiles (free users stay on #1 only) — see IsVariantUnlocked / StartSession.
// Free and VIP tiers share the same completed threshold (10), so FinishSession
// reads FreeValue without an extra billing lookup.
const unlockThresholdConfigKey = "unlock_threshold_correct"

// FinishSession finishes an in-progress session — computing its final
// score/status, persisting it, and (for variant mode) upserting bilet-unlock
// progress. It is idempotent: finishing an already-finished session returns
// the stored result with no additional writes.
func (s *Service) FinishSession(ctx context.Context, profileID, sessionID uuid.UUID) (FinishResult, error) {
	if s.Pool == nil {
		return FinishResult{}, errors.New("session transaction pool is not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return FinishResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM exam_session WHERE id = $1 FOR UPDATE`, sessionID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FinishResult{}, ErrNotFound
		}
		return FinishResult{}, err
	}
	q := sqlc.New(tx)
	row, err := q.GetExamSession(ctx, sessionID)
	if err != nil {
		return FinishResult{}, err
	}
	if row.ProfileID != profileID {
		return FinishResult{}, ErrNotFound
	}

	timedOut := false
	if IsExamLike(row.Mode) && row.TimeLimitSec.Valid {
		timedOut = !s.now().Before(row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32) * time.Second))
	}
	result, err := s.transactional(q).finishInternal(ctx, row, false, timedOut)
	if err != nil {
		return FinishResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return FinishResult{}, err
	}
	return result, nil
}

// finishInternal marks a session as finished. It is the shared landing point
// for both SubmitAnswer's 3rd-mistake stop (tooManyErrors=true) and the
// public FinishSession (manual finish / time-up, tooManyErrors=false). It is
// idempotent — a session that's no longer "in_progress" is returned as-is,
// with no further writes (in particular, no double bilet-unlock upsert).
func (s *Service) finishInternal(ctx context.Context, row sqlc.ExamSession, tooManyErrors, timedOut bool) (FinishResult, error) {
	if row.Status != "in_progress" {
		res := FinishResult{
			Status:        row.Status,
			StoppedReason: row.StoppedReason.String,
			Score:         int(row.Score.Int32),
			Total:         int(row.Total),
		}
		if row.Mode == "grand_mock" && row.Status == "passed" {
			if cert, err := s.Q.GetGrandMockCertificateBySession(ctx, row.ID); err == nil {
				res.CertificateShareCode = cert.ShareCode
			}
		}
		return res, nil
	}

	counts, err := s.Q.CountSessionAnswers(ctx, row.ID)
	if err != nil {
		return FinishResult{}, err
	}
	totalAnswered := int(counts.TotalAnswered)
	correctCount := int(counts.CorrectCount)

	var status, reason string
	switch row.Mode {
	case "exam", "grand_mock":
		wrong := totalAnswered - correctCount
		// Same fallback SubmitAnswer uses: sessions created before
		// errors_allowed existed carry NULL and are graded as standard exams.
		errorsAllowed := ExamErrorsAllowed
		if row.ErrorsAllowed.Valid {
			errorsAllowed = int(row.ErrorsAllowed.Int32)
		}
		outcome := EvaluateExam(correctCount, wrong, int(row.Total), errorsAllowed, timedOut, tooManyErrors)
		status = outcome.Status
		reason = outcome.StoppedReason
	case "placement":
		wrong := totalAnswered - correctCount
		outcome := EvaluatePlacement(correctCount, wrong, int(row.Total), timedOut, tooManyErrors)
		status = outcome.Status
		reason = outcome.StoppedReason
	default: // "variant", "practice", "mistakes", "review"
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

	var readinessAtFinish pgtype.Int4
	if row.Mode == "exam" || row.Mode == "grand_mock" || row.Mode == "placement" {
		if s.Learning != nil {
			st, statsErr := s.Learning.Stats(ctx, row.ProfileID)
			if statsErr != nil {
				return FinishResult{}, statsErr
			}
			readinessAtFinish = pgtype.Int4{Int32: int32(st.ReadinessPct), Valid: true}
		}
	}

	updated, err := s.Q.FinishExamSession(ctx, sqlc.FinishExamSessionParams{
		ID:                   row.ID,
		Status:               status,
		Score:                pgtype.Int4{Int32: int32(correctCount), Valid: true},
		StoppedReason:        stoppedReason,
		ReadinessPctAtFinish: readinessAtFinish,
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

	res := FinishResult{
		Status:        updated.Status,
		StoppedReason: reason,
		Score:         correctCount,
		Total:         int(row.Total),
	}
	if row.Mode == "grand_mock" && status == "passed" {
		code, err := newCertificateShareCode()
		if err != nil {
			return FinishResult{}, err
		}
		cert, err := s.Q.InsertGrandMockCertificate(ctx, sqlc.InsertGrandMockCertificateParams{
			SessionID: row.ID,
			ProfileID: row.ProfileID,
			ShareCode: code,
			Score:     int32(correctCount),
			Total:     row.Total,
		})
		if err != nil {
			return FinishResult{}, err
		}
		res.CertificateShareCode = cert.ShareCode
	}
	return res, nil
}

// GetSession returns the resume/history view of a single session, scoped to
// its owning profile (ErrNotFound otherwise, matching SubmitAnswer/
// FinishSession). Answered questions always expose Correct / CorrectAnswerID
// (including during an in-progress exam, matching SubmitAnswer's immediate
// green/red feedback). Unanswered questions stay sealed until the session is
// no longer in_progress, when the full answer key may be revealed.
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
		if aq.Answered && a.IsCorrect.Valid {
			correct := a.IsCorrect.Bool
			aq.Correct = &correct
		}
		// Reveal the answer key for answered questions always; for unanswered
		// ones only after the session finishes (review / early-stop key).
		if a.CorrectAnswerID.Valid && (aq.Answered || row.Status != "in_progress") {
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
	if row.ErrorsAllowed.Valid {
		v := int(row.ErrorsAllowed.Int32)
		detail.ErrorsAllowed = &v
	}
	if row.Score.Valid {
		v := int(row.Score.Int32)
		detail.Score = &v
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		detail.FinishedAt = &t
	}
	if row.Mode == "grand_mock" && row.Status == "passed" {
		if cert, err := s.Q.GetGrandMockCertificateBySession(ctx, row.ID); err == nil {
			detail.CertificateShareCode = cert.ShareCode
		}
	}
	return detail, nil
}

// PublicCertificate is the PII-light payload for a shareable Grand Mock pass.
type PublicCertificate struct {
	ShareCode string
	Score     int
	Total     int
	IssuedAt  time.Time
}

// GetPublicCertificate looks up a certificate by share code (no auth).
func (s *Service) GetPublicCertificate(ctx context.Context, shareCode string) (PublicCertificate, error) {
	shareCode = strings.TrimSpace(strings.ToLower(shareCode))
	if shareCode == "" {
		return PublicCertificate{}, ErrNotFound
	}
	cert, err := s.Q.GetGrandMockCertificateByShareCode(ctx, shareCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicCertificate{}, ErrNotFound
		}
		return PublicCertificate{}, err
	}
	return PublicCertificate{
		ShareCode: cert.ShareCode,
		Score:     int(cert.Score),
		Total:     int(cert.Total),
		IssuedAt:  cert.CreatedAt.Time.UTC(),
	}, nil
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

// bypassVariantProgress reports whether the profile may skip sequential
// bilet unlock. Missing profile → false (fail closed for the bypass only).
func (s *Service) bypassVariantProgress(ctx context.Context, profileID uuid.UUID) (bool, error) {
	p, err := s.Q.GetProfileByID(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return p.BypassVariantProgress, nil
}

// ListVariantStatuses returns every bilet variant, in number order, with the
// profile's progress against it and whether it's unlocked. Unlock matches
// StartSession: #1 for everyone; #N+1 for VIP only after #N has completed_at —
// unless profile.bypass_variant_progress (QA/ops) — via IsVariantUnlocked
// (rules.go), never reimplemented here.
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

	active, _, statusErr := s.Billing.Status(ctx, profileID)
	if statusErr != nil {
		return nil, statusErr
	}
	bypass, bypassErr := s.bypassVariantProgress(ctx, profileID)
	if bypassErr != nil {
		return nil, bypassErr
	}

	statuses := make([]VariantStatus, 0, len(variants))
	prevCompleted := true // unused for #1; seeded so the first step is clean
	for _, v := range variants {
		gatePrev := prevCompleted || bypass
		unlocked := IsVariantUnlocked(int(v.Number), active, gatePrev)
		status := VariantStatus{
			Number:        v.Number,
			QuestionCount: int(v.QuestionCount),
			Unlocked:      unlocked,
			LockReason:    VariantLockReason(int(v.Number), active, unlocked),
		}

		completed := false
		if p, ok := progressByVariant[v.ID]; ok {
			status.BestCorrect = int(p.BestCorrect)
			status.Attempts = int(p.Attempts)
			if p.CompletedAt.Valid {
				t := p.CompletedAt.Time
				status.CompletedAt = &t
				completed = true
			}
		}
		prevCompleted = completed

		statuses = append(statuses, status)
	}
	return statuses, nil
}

// Grand Mock gate reasons, as reported by MockEligibility and consumed by the
// client to pick its message and destination.
const (
	MockReasonVIPRequired   = "vip_required"
	MockReasonTooFewStudied = "too_few_studied"
	MockReasonMasteryTooLow = "mastery_too_low"
)

// limit_config keys backing the Grand Mock gate. Both are read (rather than
// hardcoded) so the values stay tunable from one place; see rules.go for the
// seeded defaults.
const (
	mockThresholdPctConfigKey  = "grand_mock_threshold_pct"
	mockMinStudiedPctConfigKey = "grand_mock_min_studied_pct"
)

// MockEligibilityResult is the read-only view GrandMockCard polls before
// offering the "start" button, and — since StartSession delegates to
// MockEligibility — also the decision the server enforces.
type MockEligibilityResult struct {
	Eligible             bool
	MasteryPercent       int
	MinRequiredPercent   int
	QuestionsStudied     int
	MinRequiredQuestions int
	IsVIP                bool
	Reason               string // one of the MockReason* constants, or ""
}

// MockEligibility reports whether the profile currently meets the Grand Mock
// requirements: an active VIP entitlement, enough of the question bank
// actually studied, and a high enough overall readiness percentage.
//
// Readiness is bank-honest (coverage × accuracy), so a handful of correct
// answers cannot inflate to 100%. The volume floor still blocks sparse study
// even when accuracy on those few items is perfect. Distinct question_memory
// rows are counted so replaying one easy question cannot satisfy the floor;
// the threshold is a share of the bank so it scales with content.
//
// Reasons are reported in the order a user should act on them — subscribe,
// then study more, then improve accuracy — and only the first blocking one is
// returned.
func (s *Service) MockEligibility(ctx context.Context, profileID uuid.UUID) (MockEligibilityResult, error) {
	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return MockEligibilityResult{}, err
	}
	thresholdCfg, err := s.Q.GetLimitConfig(ctx, mockThresholdPctConfigKey)
	if err != nil {
		return MockEligibilityResult{}, err
	}
	minStudiedCfg, err := s.Q.GetLimitConfig(ctx, mockMinStudiedPctConfigKey)
	if err != nil {
		return MockEligibilityResult{}, err
	}
	stats, err := s.Learning.Stats(ctx, profileID)
	if err != nil {
		return MockEligibilityResult{}, err
	}
	studied, err := s.Q.CountStudiedQuestions(ctx, profileID)
	if err != nil {
		return MockEligibilityResult{}, err
	}
	bankSize, err := s.Q.CountValidQuestions(ctx)
	if err != nil {
		return MockEligibilityResult{}, err
	}

	// VipValue, not FreeValue: the VIP check below runs first, so a separate
	// free-tier gate would be unreachable.
	res := MockEligibilityResult{
		MasteryPercent:       stats.ReadinessPct,
		MinRequiredPercent:   int(thresholdCfg.VipValue),
		QuestionsStudied:     int(studied),
		MinRequiredQuestions: requiredStudiedQuestions(int(bankSize), int(minStudiedCfg.VipValue)),
		IsVIP:                active,
	}
	switch {
	case !active:
		res.Reason = MockReasonVIPRequired
	case res.QuestionsStudied < res.MinRequiredQuestions:
		res.Reason = MockReasonTooFewStudied
	case res.MasteryPercent < res.MinRequiredPercent:
		res.Reason = MockReasonMasteryTooLow
	default:
		res.Eligible = true
	}
	return res, nil
}

// requiredStudiedQuestions converts the configured share of the question bank
// into an absolute count, rounding up so a non-zero percentage never collapses
// to "no requirement" on a small bank. An empty bank requires nothing — there
// would be nothing to study, and failing every VIP user because content has
// not been imported yet would be worse than letting the accuracy gate decide.
func requiredStudiedQuestions(bankSize, pct int) int {
	if bankSize <= 0 || pct <= 0 {
		return 0
	}
	return (bankSize*pct + 99) / 100
}
