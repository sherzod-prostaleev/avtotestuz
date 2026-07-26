package progress_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *progress.Service, uuid.UUID, uuid.UUID) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	return q, progress.NewService(q), profile.ID, qids[0]
}

func TestSaveListUnsaveQuestion(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	ctx := context.Background()

	if err := svc.SaveQuestion(ctx, profileID, questionID); err != nil {
		t.Fatalf("SaveQuestion: %v", err)
	}
	// saving twice must not error (idempotent)
	if err := svc.SaveQuestion(ctx, profileID, questionID); err != nil {
		t.Fatalf("SaveQuestion (repeat): %v", err)
	}

	items, err := svc.ListSaved(ctx, profileID)
	if err != nil {
		t.Fatalf("ListSaved: %v", err)
	}
	if len(items) != 1 || items[0].QuestionID != questionID {
		t.Fatalf("ListSaved = %+v", items)
	}

	if err := svc.UnsaveQuestion(ctx, profileID, questionID); err != nil {
		t.Fatalf("UnsaveQuestion: %v", err)
	}
	items, err = svc.ListSaved(ctx, profileID)
	if err != nil {
		t.Fatalf("ListSaved after unsave: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty after unsave, got %+v", items)
	}
}

func TestGetStreakFreshProfile(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	view, err := svc.GetStreak(context.Background(), profileID)
	if err != nil {
		t.Fatalf("GetStreak: %v", err)
	}
	if view.Current != 0 || view.Best != 0 || view.DailyGoal != 30 {
		t.Fatalf("fresh streak = %+v, want DailyGoal=30 (limit_config default)", view)
	}
}

func TestRecordActivityFirstCallCreatesStreak(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	view, err := svc.RecordActivity(context.Background(), profileID)
	if err != nil {
		t.Fatalf("RecordActivity: %v", err)
	}
	if view.Current != 1 || view.Best != 1 || view.TodayDone != 1 || view.DailyGoal != 30 {
		t.Fatalf("first activity = %+v", view)
	}
}

func TestRecordActivitySameDayOnlyBumpsTodayDone(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	ctx := context.Background()
	if _, err := svc.RecordActivity(ctx, profileID); err != nil {
		t.Fatalf("first: %v", err)
	}
	view, err := svc.RecordActivity(ctx, profileID)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if view.Current != 1 || view.TodayDone != 2 {
		t.Fatalf("second same-day activity = %+v", view)
	}
}
