package session_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
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
	svc := session.NewService(q, billing.Service{Q: q})
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return q, svc, profile.ID
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
	_, svc, profileID := seed(t)
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

func TestStartSessionInvalidMode(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "not-a-mode", Locale: "uz-Latn",
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
	// fixture.Sample() assigns 40 questions round-robin across 4 categories
	// (10 each) and limit_config seeds daily_practice_questions free_value=10
	// (migration 0003_billing.up.sql) — so a fresh profile's first practice
	// session of this category exhausts the entire daily allowance.
	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}

	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 100,
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if len(view.QuestionIDs) != 10 {
		t.Fatalf("expected count clamped to the free daily allowance of 10, got %d", len(view.QuestionIDs))
	}
	// Record the answers directly via InsertSessionAnswer (rather than
	// svc.SubmitAnswer, which does not exist until Task 4) so that
	// CountPracticeAnswersToday sees today's practice-mode answers and the
	// daily allowance is exhausted, exactly like a real submit would record.
	for i, qid := range view.QuestionIDs {
		correctID, err := q.GetCorrectAnswerID(context.Background(), qid)
		if err != nil {
			t.Fatalf("correct answer: %v", err)
		}
		if _, err := q.InsertSessionAnswer(context.Background(), sqlc.InsertSessionAnswerParams{
			SessionID: view.ID, QuestionID: qid, AnswerID: correctID, IsCorrect: true, Position: int16(i + 1),
		}); err != nil {
			t.Fatalf("insert session answer: %v", err)
		}
	}

	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
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
	full, err := q.GetAnswerForScoring(context.Background(), sqlc.GetAnswerForScoringParams{ID: ans[0].ID, QuestionID: questionID})
	_ = full
	_ = err
	for _, a := range ans {
		full, err := q.GetAnswerForScoring(context.Background(), sqlc.GetAnswerForScoringParams{ID: a.ID, QuestionID: questionID})
		if err == nil && full.IsCorrect {
			return a.ID
		}
	}
	t.Fatal("no correct answer found")
	return uuid.Nil
}

func TestSubmitAnswerVariantModeImmediateFeedback(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID)
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

func TestSubmitAnswerRejectsInvalidAnswerID(t *testing.T) {
	_, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], uuid.New())
	if err == nil {
		t.Fatal("random answer id must be rejected as invalid")
	}
	if err != session.ErrInvalidAnswer {
		t.Fatalf("err=%v want ErrInvalidAnswer", err)
	}
	_ = res
}

func TestSubmitAnswerExamModeRealSubmissionWithholdsFeedback(t *testing.T) {
	q, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID)
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if !res.Recorded {
		t.Fatalf("expected Recorded=true, got %+v", res)
	}
	if res.Correct != nil {
		t.Fatalf("exam mode must withhold Correct, got %+v", res)
	}
	if res.CorrectAnswerID != nil {
		t.Fatalf("exam mode must withhold CorrectAnswerID, got %+v", res)
	}
}

func TestSubmitAnswerRejectsDuplicateAndWrongQuestionPair(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != session.ErrAlreadyAnswered {
		t.Fatalf("err=%v want ErrAlreadyAnswered", err)
	}

	otherCorrect := correctAnswerID(t, q, view.QuestionIDs[1])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], otherCorrect); err != session.ErrAlreadyAnswered {
		t.Fatalf("already-answered check must run before mismatch check: err=%v", err)
	}
}

func TestSubmitAnswerExamStopsOnThirdMistake(t *testing.T) {
	q, svc, profileID := seed(t)
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
		res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[i], wrongID)
		if err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if res.Correct != nil {
			t.Fatalf("exam mode must withhold Correct, i=%d got %+v", i, res)
		}
		if res.CorrectAnswerID != nil {
			t.Fatalf("exam mode must withhold CorrectAnswerID, i=%d got %+v", i, res)
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

	if _, err := svc.SubmitAnswer(context.Background(), profileB.ID, view.ID, view.QuestionIDs[0], correctID); err != session.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestFinishSessionVariantModeUnlocksNextBilet(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
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
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
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
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
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

func TestGetSessionRedactsCorrectnessDuringInProgressExam(t *testing.T) {
	q, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	detail, err := svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Answers) != 1 || detail.Answers[0].Correct != nil {
		t.Fatalf("in-progress exam must redact correctness: %+v", detail.Answers)
	}

	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("finish: %v", err)
	}
	detail, err = svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession after finish: %v", err)
	}
	if detail.Answers[0].Correct == nil {
		t.Fatal("finished session must reveal correctness")
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

func TestListVariantStatusesSequentialUnlock(t *testing.T) {
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
	if statuses[1].Unlocked {
		t.Fatal("variant 2 must be locked before variant 1 is passed")
	}

	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("finish: %v", err)
	}

	statuses, err = svc.ListVariantStatuses(context.Background(), profileID)
	if err != nil {
		t.Fatalf("ListVariantStatuses: %v", err)
	}
	if !statuses[1].Unlocked {
		t.Fatal("variant 2 must unlock after variant 1 hits the threshold")
	}
}
