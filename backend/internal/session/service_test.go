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
