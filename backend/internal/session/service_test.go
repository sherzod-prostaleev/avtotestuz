package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/leaderboard"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *session.Service, uuid.UUID) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	svc := session.NewService(q, billing.Service{Q: q}, learning.NewService(q), progress.NewService(q))
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return q, svc, profile.ID
}

// grantVIP grants profileID an active entitlement so VIP-gated modes (exam,
// mistakes, variant 2+) can be exercised by tests that aren't specifically
// testing the free-tier gate itself.
func grantVIP(t *testing.T, q *sqlc.Queries, profileID uuid.UUID) {
	t.Helper()
	billingSvc := billing.Service{Q: q}
	if _, err := billingSvc.GrantDays(context.Background(), profileID, 7, "admin", "test", uuid.NullUUID{}); err != nil {
		t.Fatalf("grant vip: %v", err)
	}
}

// grantMastery answers every valid fixture question correctly once, which
// drives every category's mastery ratio (correct/seen, see
// learning.Service.RecordReview) to 100% — so the profile's overall
// learning.Stats().ReadinessPct clears MockMasteryThreshold (85) regardless
// of how the fixture's questions are distributed across categories.
func grantMastery(t *testing.T, q *sqlc.Queries, l *learning.Service, profileID uuid.UUID) {
	t.Helper()
	studyQuestions(t, q, l, profileID, 0, learning.Good)
}

// studyQuestions records a review of `limit` fixture questions (0 = all) at the
// given rating. learning.Good drives mastery up; learning.Again drives it to
// zero while still building study volume, which is what separates the Grand
// Mock gate's two failure reasons: too_few_studied (studied too little) versus
// mastery_too_low (studied enough, answered badly).
func studyQuestions(t *testing.T, q *sqlc.Queries, l *learning.Service, profileID uuid.UUID, limit int, rating learning.Rating) {
	t.Helper()
	ctx := context.Background()
	ids, err := q.RandomQuestionIDs(ctx, 1000)
	if err != nil {
		t.Fatalf("studyQuestions: list questions: %v", err)
	}
	if limit > 0 && limit < len(ids) {
		ids = ids[:limit]
	}
	for _, qid := range ids {
		if _, err := l.RecordReview(ctx, profileID, qid, rating); err != nil {
			t.Fatalf("studyQuestions: record review: %v", err)
		}
	}
}

// studyOnePerCategoryCorrectly answers exactly ONE question correctly in every
// category. That drives each category's mastery ratio to 1.0 and therefore the
// weighted overall readiness_pct to 100 — the precise exploit that made the
// Grand Mock accuracy gate hollow — while leaving study volume at one question
// per category.
func studyOnePerCategoryCorrectly(t *testing.T, q *sqlc.Queries, l *learning.Service, profileID uuid.UUID) int {
	t.Helper()
	ctx := context.Background()
	ids, err := q.RandomQuestionIDs(ctx, 1000)
	if err != nil {
		t.Fatalf("studyOnePerCategoryCorrectly: list questions: %v", err)
	}
	done := map[uuid.UUID]bool{}
	for _, qid := range ids {
		catID, err := q.GetQuestionCategoryID(ctx, qid)
		if err != nil {
			t.Fatalf("studyOnePerCategoryCorrectly: category of %s: %v", qid, err)
		}
		if done[catID] {
			continue
		}
		if _, err := l.RecordReview(ctx, profileID, qid, learning.Good); err != nil {
			t.Fatalf("studyOnePerCategoryCorrectly: record review: %v", err)
		}
		done[catID] = true
	}
	return len(done)
}

func TestStartSessionVariantMode(t *testing.T) {
	q, svc, profileID := seed(t)
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v.ID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.Total != 20 || len(view.QuestionIDs) != 20 {
		t.Fatalf("expected 20 questions, got total=%d ids=%d", view.Total, len(view.QuestionIDs))
	}
	if view.TimeLimitSec != nil {
		t.Fatalf("variant mode must have no time limit, got %v", *view.TimeLimitSec)
	}
}

func TestStartSessionExamMode(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.Total != session.ExamQuestionCount || len(view.QuestionIDs) != session.ExamQuestionCount {
		t.Fatalf("expected %d questions, got %d", session.ExamQuestionCount, len(view.QuestionIDs))
	}
	if view.TimeLimitSec == nil || *view.TimeLimitSec != session.ExamTimeLimitSec {
		t.Fatalf("expected time limit %d, got %v", session.ExamTimeLimitSec, view.TimeLimitSec)
	}
}

func TestStartSessionPlacementFree(t *testing.T) {
	_, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "placement", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession placement (free): %v", err)
	}
	if view.Total != session.PlacementQuestionCount || len(view.QuestionIDs) != session.PlacementQuestionCount {
		t.Fatalf("expected %d questions, got %d", session.PlacementQuestionCount, len(view.QuestionIDs))
	}
	if view.TimeLimitSec == nil || *view.TimeLimitSec != session.PlacementTimeLimitSec {
		t.Fatalf("expected time limit %d, got %v", session.PlacementTimeLimitSec, view.TimeLimitSec)
	}
	if view.Mode != "placement" {
		t.Fatalf("mode=%q want placement", view.Mode)
	}
}

func TestStartSessionInvalidMode(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "not-a-mode", Locale: "uz-Latn",
	}); err != session.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestStartSessionInvalidLocale(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "fr-FR",
	}); err != session.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestStartSessionVariantRequiresVariantID(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", Locale: "uz-Latn",
	}); err != session.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestStartSessionPracticeDailyLimitClampsAndBlocks(t *testing.T) {
	q, svc, profileID := seed(t)
	ctx := context.Background()
	// Read the allowance instead of restating it: this test previously hard-coded
	// the seeded value of 10 and broke the moment the free tier was retuned,
	// even though the behaviour under test — clamp, then block — never changed.
	cfg, err := q.GetLimitConfig(ctx, "daily_practice_questions")
	if err != nil {
		t.Fatalf("limit config: %v", err)
	}
	allowance := int(cfg.FreeValue)
	if allowance <= 0 {
		t.Fatalf("free allowance = %d, expected a finite positive limit to test against", allowance)
	}

	// fixture.Sample() assigns 40 questions round-robin across 4 categories
	// (10 each), so one session can never exceed 10 regardless of allowance.
	catID, err := q.GetCategoryIDByCode(ctx, "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}

	// Record answers directly via InsertSessionAnswer (rather than
	// svc.SubmitAnswer) so CountPracticeAnswersToday sees today's
	// practice-mode answers exactly as a real submit would record them.
	answered := 0
	for answered < allowance {
		view, err := svc.StartSession(ctx, profileID, session.StartRequest{
			Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 100,
		})
		if err != nil {
			t.Fatalf("StartSession after %d answers: %v", answered, err)
		}
		if len(view.QuestionIDs) == 0 {
			t.Fatalf("empty session after %d answers, allowance %d", answered, allowance)
		}
		if remaining := allowance - answered; len(view.QuestionIDs) > remaining {
			t.Fatalf("session of %d exceeds remaining allowance %d", len(view.QuestionIDs), remaining)
		}
		for i, qid := range view.QuestionIDs {
			correctID, err := q.GetCorrectAnswerID(ctx, qid)
			if err != nil {
				t.Fatalf("correct answer: %v", err)
			}
			if _, err := q.InsertSessionAnswer(ctx, sqlc.InsertSessionAnswerParams{
				SessionID: view.ID, QuestionID: qid, AnswerID: correctID, IsCorrect: true, Position: int16(i + 1),
			}); err != nil {
				t.Fatalf("insert session answer: %v", err)
			}
		}
		answered += len(view.QuestionIDs)
	}

	if _, err := svc.StartSession(ctx, profileID, session.StartRequest{
		Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 5,
	}); err != session.ErrDailyLimitReached {
		t.Fatalf("err=%v want ErrDailyLimitReached once today's allowance is used up", err)
	}
}

func startVariantSession(t *testing.T, q *sqlc.Queries, svc *session.Service, profileID uuid.UUID) session.SessionView {
	t.Helper()
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v.ID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	return view
}

func correctAnswerID(t *testing.T, q *sqlc.Queries, questionID uuid.UUID) uuid.UUID {
	t.Helper()
	ans, err := q.ListAnswersByQuestionIDs(context.Background(),
		sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{questionID}, Locale: "uz-Latn"})
	if err != nil || len(ans) == 0 {
		t.Fatalf("answers: %v", err)
	}
	// fixture guarantees exactly one correct answer per question; find it
	// via a direct query since ListAnswersByQuestionIDs never exposes it.
	for _, a := range ans {
		full, err := q.GetAnswerForScoring(context.Background(), sqlc.GetAnswerForScoringParams{ID: a.ID, QuestionID: questionID})
		if err == nil && full.IsCorrect {
			return a.ID
		}
	}
	t.Fatal("no correct answer found")
	return uuid.Nil
}

// wrongAnswerID returns any answer for questionID other than the correct
// one, for tests that need to deliberately submit an incorrect answer.
func wrongAnswerID(t *testing.T, q *sqlc.Queries, questionID uuid.UUID) uuid.UUID {
	t.Helper()
	ans, err := q.ListAnswersByQuestionIDs(context.Background(),
		sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{questionID}, Locale: "uz-Latn"})
	if err != nil || len(ans) == 0 {
		t.Fatalf("answers: %v", err)
	}
	correctID := correctAnswerID(t, q, questionID)
	for _, a := range ans {
		if a.ID != correctID {
			return a.ID
		}
	}
	t.Fatal("no wrong answer found")
	return uuid.Nil
}

func TestSubmitAnswerVariantModeImmediateFeedback(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{})
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if res.Correct == nil || !*res.Correct {
		t.Fatalf("expected correct=true, got %+v", res)
	}
	if res.CorrectAnswerID == nil || *res.CorrectAnswerID != correctID {
		t.Fatalf("expected correct_answer_id=%v, got %+v", correctID, res.CorrectAnswerID)
	}
}

func TestSubmitAnswerSkipFSRSDoesNotRecordReview(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	qid := view.QuestionIDs[0]
	correctID := correctAnswerID(t, q, qid)

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID, session.SubmitAnswerOpts{
		SkipFSRS: true,
	}); err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	counts, err := q.CountSessionAnswers(context.Background(), view.ID)
	if err != nil || counts.TotalAnswered != 1 {
		t.Fatalf("session answer must still be saved: %+v err=%v", counts, err)
	}
	if _, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{
		ProfileID: profileID, QuestionID: qid,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("skip_fsrs must leave question_memory empty, got err=%v", err)
	}
}

func TestSubmitAnswerLatencyMapsToFSRSRating(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	qid := view.QuestionIDs[0]
	correctID := correctAnswerID(t, q, qid)
	fast := 2000

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID, session.SubmitAnswerOpts{
		LatencyMs: &fast,
	}); err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	mem, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{
		ProfileID: profileID, QuestionID: qid,
	})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	want := learning.Review(learning.Card{}, learning.Easy, time.Now(), learning.DefaultDesiredRetention)
	// First-review Easy has distinctly higher stability than Good/Hard.
	if float64(mem.Stability) < want.Stability*0.9 {
		t.Fatalf("fast latency should grade Easy: stability=%v want≈%v", mem.Stability, want.Stability)
	}
}

func TestSubmitAnswerRejectsInvalidAnswerID(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], uuid.New(), session.SubmitAnswerOpts{})
	if err == nil {
		t.Fatal("random answer id must be rejected as invalid")
	}
	if err != session.ErrInvalidAnswer {
		t.Fatalf("err=%v want ErrInvalidAnswer", err)
	}
	_ = res
}

func TestSubmitAnswerRejectsQuestionOutsideSession(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)

	secondVariant, err := q.GetVariantByNumber(context.Background(), 2)
	if err != nil {
		t.Fatalf("get second variant: %v", err)
	}
	outsideIDs, err := q.ListVariantQuestionIDsOrdered(context.Background(), secondVariant.ID)
	if err != nil || len(outsideIDs) == 0 {
		t.Fatalf("outside questions: len=%d err=%v", len(outsideIDs), err)
	}
	outsideQuestionID := outsideIDs[0]
	outsideAnswerID := correctAnswerID(t, q, outsideQuestionID)

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, outsideQuestionID, outsideAnswerID, session.SubmitAnswerOpts{}); !errors.Is(err, session.ErrQuestionNotAssigned) {
		t.Fatalf("a valid question/answer pair outside the assigned session must return ErrQuestionNotAssigned, got %v", err)
	}
	counts, err := q.CountSessionAnswers(context.Background(), view.ID)
	if err != nil {
		t.Fatalf("count session answers: %v", err)
	}
	if counts.TotalAnswered != 0 || counts.CorrectCount != 0 {
		t.Fatalf("rejected injection must not be recorded: %+v", counts)
	}
	if _, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{
		ProfileID: profileID, QuestionID: outsideQuestionID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("rejected injection must not update FSRS memory, got err=%v", err)
	}
	if _, err := q.GetStreak(context.Background(), profileID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("rejected injection must not update streak, got err=%v", err)
	}
}

func TestSubmitAnswerExamModeRealSubmissionProvidesFeedback(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{})
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if !res.Recorded {
		t.Fatalf("expected Recorded=true, got %+v", res)
	}
	// Exam mode now returns per-answer feedback (matching official Avtotest desktop app).
	if res.Correct == nil || *res.Correct != true {
		t.Fatalf("exam mode should report correct=true for right answer, got %+v", res)
	}
	if res.CorrectAnswerID == nil {
		t.Fatalf("exam mode should report correct_answer_id, got %+v", res)
	}
	if *res.CorrectAnswerID != correctID {
		t.Fatalf("expected correct_answer_id=%v, got %+v", correctID, res.CorrectAnswerID)
	}
}

func TestSubmitAnswerExpiresExamBeforeRecordingSideEffects(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	answerID := correctAnswerID(t, q, view.QuestionIDs[0])
	svc.Now = func() time.Time {
		return view.StartedAt.Add(time.Duration(session.ExamTimeLimitSec+1) * time.Second)
	}

	result, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], answerID, session.SubmitAnswerOpts{})
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if result.Recorded || !result.Stopped || result.StopReason != "time_up" {
		t.Fatalf("expired exam must stop without recording: %+v", result)
	}
	counts, err := q.CountSessionAnswers(context.Background(), view.ID)
	if err != nil || counts.TotalAnswered != 0 {
		t.Fatalf("expired answer count=%+v err=%v, want zero", counts, err)
	}
	if _, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{
		ProfileID: profileID, QuestionID: view.QuestionIDs[0],
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired answer must not update FSRS memory, got err=%v", err)
	}
	if _, err := q.GetStreak(context.Background(), profileID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired answer must not update streak, got err=%v", err)
	}
	stored, err := q.GetExamSession(context.Background(), view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || !stored.StoppedReason.Valid || stored.StoppedReason.String != "time_up" || !stored.Score.Valid || stored.Score.Int32 != 0 {
		t.Fatalf("expired exam was not finalized as time_up: %+v", stored)
	}
}

func TestSubmitAnswerRejectsDuplicateAndWrongQuestionPair(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{}); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{}); err != session.ErrAlreadyAnswered {
		t.Fatalf("err=%v want ErrAlreadyAnswered", err)
	}

	otherCorrect := correctAnswerID(t, q, view.QuestionIDs[1])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], otherCorrect, session.SubmitAnswerOpts{}); err != session.ErrAlreadyAnswered {
		t.Fatalf("already-answered check must run before mismatch check: err=%v", err)
	}
}

func TestSubmitAnswerExamStopsOnThirdMistake(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	// answer the first 3 questions wrong by submitting a non-correct answer id each time
	for i := 0; i < 3; i++ {
		ans, err := q.ListAnswersByQuestionIDs(context.Background(),
			sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{view.QuestionIDs[i]}, Locale: "uz-Latn"})
		if err != nil || len(ans) != 4 {
			t.Fatalf("answers: %v", err)
		}
		correctID := correctAnswerID(t, q, view.QuestionIDs[i])
		var wrongID uuid.UUID
		for _, a := range ans {
			if a.ID != correctID {
				wrongID = a.ID
				break
			}
		}
		res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[i], wrongID, session.SubmitAnswerOpts{})
		if err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		// Exam mode now returns per-answer feedback.
		if res.Correct == nil || *res.Correct != false {
			t.Fatalf("exam mode should report correct=false for wrong answer, i=%d got %+v", i, res)
		}
		if res.CorrectAnswerID == nil {
			t.Fatalf("exam mode should report correct_answer_id, i=%d got %+v", i, res)
		}
		if i < 2 {
			if res.Stopped {
				t.Fatalf("must not stop before the 3rd mistake, i=%d", i)
			}
		} else {
			if !res.Stopped || res.StopReason != "too_many_errors" {
				t.Fatalf("expected stop on 3rd mistake, got %+v", res)
			}
		}
	}
}

func TestSubmitAnswerOwnershipIsEnforced(t *testing.T) {
	q, svc, profileAID := seed(t)
	view := startVariantSession(t, q, svc, profileAID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	profileB, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998907654321",
	})
	if err != nil {
		t.Fatalf("create second profile: %v", err)
	}

	if _, err := svc.SubmitAnswer(context.Background(), profileB.ID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{}); err != session.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestFinishSessionVariantModeUnlocksNextBilet(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID, session.SubmitAnswerOpts{}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	res, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("FinishSession: %v", err)
	}
	if res.Status != "passed" || res.Score != 20 {
		t.Fatalf("expected passed 20/20, got %+v", res)
	}

	v1, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := q.GetVariantProgress(context.Background(), sqlc.GetVariantProgressParams{ProfileID: profileID, VariantID: v1.ID})
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if progress.BestCorrect != 20 || !progress.CompletedAt.Valid {
		t.Fatalf("expected best_correct=20 and completed_at set, got %+v", progress)
	}
}

func TestFinishSessionIsIdempotent(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs[:5] {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID, session.SubmitAnswerOpts{}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	first, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("first finish: %v", err)
	}
	second, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("second finish: %v", err)
	}
	if first != second {
		t.Fatalf("finish must be idempotent: first=%+v second=%+v", first, second)
	}
}

func TestFinishSessionAbandonedWhenIncomplete(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	res, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("FinishSession: %v", err)
	}
	if res.Status != "abandoned" {
		t.Fatalf("expected abandoned with 1/20 answered, got %+v", res)
	}
}

func TestGetSessionRevealsAnsweredFeedbackDuringInProgressExam(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	detail, err := svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Answers) != len(view.QuestionIDs) {
		t.Fatalf("resume must return all %d assigned questions, got %d", len(view.QuestionIDs), len(detail.Answers))
	}
	if !detail.Answers[0].Answered || detail.Answers[0].UserAnswerID == nil || *detail.Answers[0].UserAnswerID != correctID ||
		detail.Answers[0].Correct == nil || !*detail.Answers[0].Correct ||
		detail.Answers[0].CorrectAnswerID == nil || *detail.Answers[0].CorrectAnswerID != correctID {
		t.Fatalf("in-progress exam must keep green/red feedback for answered questions: %+v", detail.Answers[0])
	}
	for i, answer := range detail.Answers[1:] {
		if answer.Answered || answer.UserAnswerID != nil || answer.Correct != nil || answer.CorrectAnswerID != nil {
			t.Fatalf("in-progress exam question %d leaks answer data: %+v", i+1, answer)
		}
	}

	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("finish: %v", err)
	}
	detail, err = svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession after finish: %v", err)
	}
	if detail.Answers[0].Correct == nil || !*detail.Answers[0].Correct ||
		detail.Answers[0].CorrectAnswerID == nil || *detail.Answers[0].CorrectAnswerID != correctID {
		t.Fatalf("finished exam must reveal submitted correctness and answer key: %+v", detail.Answers[0])
	}
	for i, answer := range detail.Answers {
		if answer.CorrectAnswerID == nil {
			t.Fatalf("finished exam question %d must disclose its answer key: %+v", i, answer)
		}
	}
}

func TestGetSessionReturnsExactOrderedQuestionSetBeforeAnyAnswers(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)

	detail, err := svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Answers) != len(view.QuestionIDs) {
		t.Fatalf("new session must resume with all %d questions, got %d", len(view.QuestionIDs), len(detail.Answers))
	}
	for i, got := range detail.Answers {
		if got.QuestionID != view.QuestionIDs[i] || got.Position != i+1 {
			t.Fatalf("question %d = (%s, position %d), want (%s, position %d)",
				i, got.QuestionID, got.Position, view.QuestionIDs[i], i+1)
		}
		if got.Answered || got.UserAnswerID != nil || got.Correct != nil || got.CorrectAnswerID != nil {
			t.Fatalf("fresh question %d must be unanswered with no correctness: %+v", i, got)
		}
	}
}

func TestGetSessionKeepsAssignedOrderWhenAnsweredOutOfOrder(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)

	answerIDs := make(map[int]uuid.UUID)
	for _, index := range []int{7, 2} {
		answerID := correctAnswerID(t, q, view.QuestionIDs[index])
		answerIDs[index] = answerID
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[index], answerID, session.SubmitAnswerOpts{}); err != nil {
			t.Fatalf("submit question %d: %v", index, err)
		}
	}

	detail, err := svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Answers) != len(view.QuestionIDs) {
		t.Fatalf("got %d questions, want %d", len(detail.Answers), len(view.QuestionIDs))
	}
	for i, got := range detail.Answers {
		if got.QuestionID != view.QuestionIDs[i] || got.Position != i+1 {
			t.Fatalf("question order changed at %d: got (%s, %d), want (%s, %d)",
				i, got.QuestionID, got.Position, view.QuestionIDs[i], i+1)
		}
		wantAnswered := i == 2 || i == 7
		if got.Answered != wantAnswered {
			t.Fatalf("question %d answered=%v, want %v", i, got.Answered, wantAnswered)
		}
		if wantAnswered {
			if got.UserAnswerID == nil || *got.UserAnswerID != answerIDs[i] ||
				got.Correct == nil || !*got.Correct ||
				got.CorrectAnswerID == nil || *got.CorrectAnswerID != answerIDs[i] {
				t.Fatalf("feedback-mode answer %d must disclose the recorded result: %+v", i, got)
			}
		} else if got.UserAnswerID != nil || got.Correct != nil || got.CorrectAnswerID != nil {
			t.Fatalf("unanswered feedback-mode question %d leaks answer data: %+v", i, got)
		}
	}
}

func TestGetSessionOwnershipIsEnforced(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	other, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998907654321"})
	if err != nil {
		t.Fatalf("create other profile: %v", err)
	}
	if _, err := svc.GetSession(context.Background(), other.ID, view.ID); err != session.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound for another profile's session", err)
	}
}

func TestListVariantStatusesVIPNeedsPreviousComplete(t *testing.T) {
	q, svc, profileID := seed(t)
	statuses, err := svc.ListVariantStatuses(context.Background(), profileID)
	if err != nil {
		t.Fatalf("ListVariantStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("fixture has 2 variants, got %d", len(statuses))
	}
	if !statuses[0].Unlocked {
		t.Fatal("variant 1 must always be unlocked")
	}
	if statuses[1].Unlocked || statuses[1].LockReason != session.LockReasonVIPRequired {
		t.Fatalf("variant 2 must be vip_required for free profiles: %+v", statuses[1])
	}

	grantVIP(t, q, profileID)

	statuses, err = svc.ListVariantStatuses(context.Background(), profileID)
	if err != nil {
		t.Fatalf("ListVariantStatuses: %v", err)
	}
	if statuses[1].Unlocked || statuses[1].LockReason != session.LockReasonPrevRequired {
		t.Fatalf("VIP without #1 completed must keep #2 prev_required: %+v", statuses[1])
	}
}

func TestListVariantStatusesVIPUnlocksNextAfterThreshold(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)

	view := startVariantSession(t, q, svc, profileID)
	// Threshold is 10; answer 10 correctly then finish (remaining unanswered → abandoned
	// still upserts progress with completed_at when correctCount >= threshold).
	for _, qid := range view.QuestionIDs[:10] {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID, session.SubmitAnswerOpts{}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("FinishSession: %v", err)
	}

	statuses, err := svc.ListVariantStatuses(context.Background(), profileID)
	if err != nil {
		t.Fatalf("ListVariantStatuses: %v", err)
	}
	if !statuses[0].Unlocked || statuses[0].CompletedAt == nil {
		t.Fatalf("variant 1 must be completed: %+v", statuses[0])
	}
	if !statuses[1].Unlocked || statuses[1].LockReason != "" {
		t.Fatalf("variant 2 must unlock after #1 completed: %+v", statuses[1])
	}
}

func TestStartSessionVariantTwoRequiresVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	v2, err := q.GetVariantByNumber(context.Background(), 2)
	if err != nil {
		t.Fatalf("get variant 2: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v2.ID, Locale: "uz-Latn",
	}); err != session.ErrRequiresVIP {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionVariantVIPRequiresPreviousComplete(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	v2, err := q.GetVariantByNumber(context.Background(), 2)
	if err != nil {
		t.Fatalf("get variant 2: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v2.ID, Locale: "uz-Latn",
	}); err != session.ErrVariantLocked {
		t.Fatalf("err=%v want ErrVariantLocked before #1 completed", err)
	}

	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs[:10] {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID, session.SubmitAnswerOpts{}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("FinishSession: %v", err)
	}

	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v2.ID, Locale: "uz-Latn",
	}); err != nil {
		t.Fatalf("VIP must start variant 2 after completing #1: %v", err)
	}
}

func TestStartSessionVariantOneNeverRequiresVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	v1, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant 1: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v1.ID, Locale: "uz-Latn",
	}); err != nil {
		t.Fatalf("variant 1 should always be accessible: %v", err)
	}
}

func TestStartSessionExamRequiresVIP(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	}); err != session.ErrRequiresVIP {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionMistakesRequiresVIP(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "mistakes", Locale: "uz-Latn",
	}); err != session.ErrRequiresVIP {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionReviewNothingDue(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "review", Locale: "uz-Latn", Count: 10,
	}); !errors.Is(err, session.ErrNothingDue) {
		t.Fatalf("err=%v want ErrNothingDue", err)
	}
}

func TestStartSessionReviewUsesDueQueueWithoutVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	ctx := context.Background()
	ids, err := q.RandomQuestionIDs(ctx, 5)
	if err != nil || len(ids) < 3 {
		t.Fatalf("need fixture questions: %v len=%d", err, len(ids))
	}
	l := learning.NewService(q)
	for _, qid := range ids[:3] {
		if _, err := l.RecordReview(ctx, profileID, qid, learning.Again); err != nil {
			t.Fatalf("RecordReview: %v", err)
		}
		// Force due now so the session start does not depend on FSRS timing.
		if _, err := q.UpsertQuestionMemory(ctx, sqlc.UpsertQuestionMemoryParams{
			ProfileID:      profileID,
			QuestionID:     qid,
			Stability:      0.1,
			Difficulty:     5,
			DueAt:          pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
			LastReviewedAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			Reps:           1,
			Lapses:         1,
			State:          1,
		}); err != nil {
			t.Fatalf("backdate due: %v", err)
		}
	}

	view, err := svc.StartSession(ctx, profileID, session.StartRequest{
		Mode: "review", Locale: "uz-Latn", Count: 10,
	})
	if err != nil {
		t.Fatalf("StartSession review: %v", err)
	}
	if view.Mode != "review" {
		t.Fatalf("mode=%q want review", view.Mode)
	}
	if view.Total < 1 || len(view.QuestionIDs) < 1 {
		t.Fatalf("want due questions in review session, got %+v", view)
	}
}

func TestStartSessionEmptyMistakesPersistsAnEmptyQuestionSet(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "mistakes", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.Total != 0 || len(view.QuestionIDs) != 0 {
		t.Fatalf("empty mistake bank session=%+v, want zero questions", view)
	}
	detail, err := svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if detail.Total != 0 || len(detail.Answers) != 0 {
		t.Fatalf("empty mistake session resume=%+v, want zero questions", detail)
	}
}

func TestStartSessionPracticeNeverRequiresVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatalf("category: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 3,
	}); err != nil {
		t.Fatalf("practice should always be accessible (daily-limited, not VIP-gated): %v", err)
	}
}

func TestStartSessionExamWorksForVIPProfile(t *testing.T) {
	q, svc, profileID := seed(t)
	billingSvc := billing.Service{Q: q}
	if _, err := billingSvc.GrantDays(context.Background(), profileID, 7, "admin", "test", uuid.NullUUID{}); err != nil {
		t.Fatalf("grant vip: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	}); err != nil {
		t.Fatalf("VIP profile should be able to start exam: %v", err)
	}
}

func TestStartSessionGrandMockNotEligibleLowMastery(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	// Enough study volume to clear the volume floor, but every answer wrong —
	// so the blocking reason is accuracy, not coverage.
	studyQuestions(t, q, svc.Learning, profileID, 0, learning.Again)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "grand_mock", Locale: "uz-Latn",
	}); !errors.Is(err, session.ErrMockNotEligible) {
		t.Fatalf("err=%v want ErrMockNotEligible", err)
	}
}

// TestStartSessionGrandMockNotEligibleTooFewStudied: with only one correct
// answer per category, QuestionsStudied is far below the volume floor. Bank-
// honest readiness is also low, but the gate reports too_few_studied first
// (action order: VIP → study more → accuracy).
func TestStartSessionGrandMockNotEligibleTooFewStudied(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	studied := studyOnePerCategoryCorrectly(t, q, svc.Learning, profileID)

	elig, err := svc.MockEligibility(context.Background(), profileID)
	if err != nil {
		t.Fatalf("MockEligibility: %v", err)
	}
	if elig.MinRequiredQuestions <= studied {
		t.Fatalf("volume floor is %d but %d questions were studied — too low for this test to mean anything",
			elig.MinRequiredQuestions, studied)
	}
	if elig.Eligible || elig.Reason != session.MockReasonTooFewStudied {
		t.Fatalf("eligible=%v reason=%q mastery=%d%%, want blocked on %q",
			elig.Eligible, elig.Reason, elig.MasteryPercent, session.MockReasonTooFewStudied)
	}

	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "grand_mock", Locale: "uz-Latn",
	}); !errors.Is(err, session.ErrMockNotEligible) {
		t.Fatalf("err=%v want ErrMockNotEligible", err)
	}
}

// TestStartSessionGrandMockNoVIPRequiresPayment pins that a missing
// subscription surfaces as ErrRequiresVIP (402 vip_required), not the generic
// mock error. The client routes vip_required to /premium; when this returned
// ErrMockNotEligible instead, a non-VIP user who reached the start URL got an
// unexplained error and was sent away from the paywall.
func TestStartSessionGrandMockNoVIPRequiresPayment(t *testing.T) {
	q, svc, profileID := seed(t)
	grantMastery(t, q, svc.Learning, profileID)
	// No VIP entitlement granted.
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "grand_mock", Locale: "uz-Latn",
	}); !errors.Is(err, session.ErrRequiresVIP) {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionGrandMockEligible(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	grantMastery(t, q, svc.Learning, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "grand_mock", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.Mode != "grand_mock" {
		t.Fatalf("expected mode=grand_mock, got %q", view.Mode)
	}
	if view.Total != session.ExamQuestionCount || len(view.QuestionIDs) != session.ExamQuestionCount {
		t.Fatalf("expected %d questions, got %d", session.ExamQuestionCount, len(view.QuestionIDs))
	}
	if view.TimeLimitSec == nil || *view.TimeLimitSec != session.ExamTimeLimitSec {
		t.Fatalf("expected time limit %d, got %v", session.ExamTimeLimitSec, view.TimeLimitSec)
	}
}

func TestSubmitAnswerGrandMockStopsOnThirdMistake(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	grantMastery(t, q, svc.Learning, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "grand_mock", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	// answer the first 3 questions wrong, same pattern as
	// TestSubmitAnswerExamStopsOnThirdMistake — grand_mock shares the exam
	// pipeline via IsExamLike, so the 3rd mistake must stop it too.
	for i := 0; i < 3; i++ {
		wrongID := wrongAnswerID(t, q, view.QuestionIDs[i])
		res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[i], wrongID, session.SubmitAnswerOpts{})
		if err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if i < 2 {
			if res.Stopped {
				t.Fatalf("must not stop before the 3rd mistake, i=%d", i)
			}
		} else {
			if !res.Stopped || res.StopReason != "too_many_errors" {
				t.Fatalf("expected stop on 3rd mistake, got %+v", res)
			}
		}
	}
	stored, err := q.GetExamSession(context.Background(), view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || !stored.StoppedReason.Valid || stored.StoppedReason.String != "too_many_errors" {
		t.Fatalf("expected grand_mock session finalized as too_many_errors/failed, got %+v", stored)
	}
}

func TestFinishSessionGrandMockPassFail(t *testing.T) {
	t.Run("passes at 18/20 with 2 wrong", func(t *testing.T) {
		q, svc, profileID := seed(t)
		grantVIP(t, q, profileID)
		grantMastery(t, q, svc.Learning, profileID)
		view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
			Mode: "grand_mock", Locale: "uz-Latn",
		})
		if err != nil {
			t.Fatalf("StartSession: %v", err)
		}
		for i, qid := range view.QuestionIDs {
			var answerID uuid.UUID
			if i < 2 {
				answerID = wrongAnswerID(t, q, qid)
			} else {
				answerID = correctAnswerID(t, q, qid)
			}
			if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, answerID, session.SubmitAnswerOpts{}); err != nil {
				t.Fatalf("submit %d: %v", i, err)
			}
		}
		res, err := svc.FinishSession(context.Background(), profileID, view.ID)
		if err != nil {
			t.Fatalf("FinishSession: %v", err)
		}
		if res.Status != "passed" || res.Score != 18 {
			t.Fatalf("expected passed 18/20, got %+v", res)
		}
		if res.CertificateShareCode == "" {
			t.Fatal("expected certificate_share_code on grand mock pass")
		}
		pub, err := svc.GetPublicCertificate(context.Background(), res.CertificateShareCode)
		if err != nil {
			t.Fatalf("GetPublicCertificate: %v", err)
		}
		if pub.Score != 18 || pub.Total != 20 {
			t.Fatalf("public cert = %+v", pub)
		}
		// Idempotent finish keeps the same share code.
		again, err := svc.FinishSession(context.Background(), profileID, view.ID)
		if err != nil {
			t.Fatalf("FinishSession again: %v", err)
		}
		if again.CertificateShareCode != res.CertificateShareCode {
			t.Fatalf("share code changed on idempotent finish: %q vs %q", again.CertificateShareCode, res.CertificateShareCode)
		}
	})

	t.Run("fails when correct count falls short", func(t *testing.T) {
		q, svc, profileID := seed(t)
		grantVIP(t, q, profileID)
		grantMastery(t, q, svc.Learning, profileID)
		view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
			Mode: "grand_mock", Locale: "uz-Latn",
		})
		if err != nil {
			t.Fatalf("StartSession: %v", err)
		}
		// Answer 18 questions (16 correct, 2 wrong — the max wrong that
		// doesn't auto-stop the exam-like pipeline) and finish manually with
		// 2 questions left unanswered, so EvaluateExam sees correct=16 <
		// total-ExamErrorsAllowed(18) while wrong stays within the allowed 2.
		for i, qid := range view.QuestionIDs[:18] {
			var answerID uuid.UUID
			if i < 2 {
				answerID = wrongAnswerID(t, q, qid)
			} else {
				answerID = correctAnswerID(t, q, qid)
			}
			if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, answerID, session.SubmitAnswerOpts{}); err != nil {
				t.Fatalf("submit %d: %v", i, err)
			}
		}
		res, err := svc.FinishSession(context.Background(), profileID, view.ID)
		if err != nil {
			t.Fatalf("FinishSession: %v", err)
		}
		if res.Status != "failed" || res.Score != 16 || res.StoppedReason != "completed" {
			t.Fatalf("expected failed 16/20 (completed), got %+v", res)
		}
	})
}

func TestSubmitAnswerBumpsStreak(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID, session.SubmitAnswerOpts{}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	streakView, err := svc.Progress.GetStreak(context.Background(), profileID)
	if err != nil {
		t.Fatalf("GetStreak: %v", err)
	}
	if streakView.Current != 1 || streakView.TodayDone != 1 {
		t.Fatalf("expected streak bumped after answering, got %+v", streakView)
	}
}

func TestSubmitAnswerRecordsLeaderboardPointOnCorrectAnswer(t *testing.T) {
	q, svc, profileID := seed(t)
	rdb := redisx.NewTest(t)
	svc.Leaderboard = leaderboard.NewService(rdb, q, billing.Service{Q: q})

	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 1})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctAnswerID, err := q.GetCorrectAnswerID(context.Background(), view.QuestionIDs[0])
	if err != nil {
		t.Fatalf("GetCorrectAnswerID: %v", err)
	}

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctAnswerID, session.SubmitAnswerOpts{}); err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}

	res, err := svc.Leaderboard.GetLeaderboard(context.Background(), profileID, leaderboard.PeriodDaily)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if res.YouScore != 1 {
		t.Errorf("YouScore = %d, want 1", res.YouScore)
	}
}

func TestSubmitAnswerWorksWithNilLeaderboard(t *testing.T) {
	// svc.Leaderboard defaults to nil (seed() doesn't set it) — confirms
	// existing/unrelated tests and any caller that never wires a
	// leaderboard.Service keep working unchanged.
	q, svc, profileID := seed(t)
	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 1})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctAnswerID, err := q.GetCorrectAnswerID(context.Background(), view.QuestionIDs[0])
	if err != nil {
		t.Fatalf("GetCorrectAnswerID: %v", err)
	}
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctAnswerID, session.SubmitAnswerOpts{}); err != nil {
		t.Fatalf("SubmitAnswer with nil Leaderboard: %v", err)
	}
}
