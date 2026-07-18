package learning_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *learning.Service, uuid.UUID, []uuid.UUID) {
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
		t.Fatalf("get variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v %d", err, len(qids))
	}
	svc := learning.NewService(q)
	return q, svc, profile.ID, qids
}

func TestRecordReviewFirstTimeCreatesRow(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	card, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("RecordReview: %v", err)
	}
	if card.Reps != 1 || card.Lapses != 0 {
		t.Fatalf("unexpected card: %+v", card)
	}

	row, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: qids[0]})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	if row.Reps != 1 {
		t.Fatalf("stored reps = %d, want 1", row.Reps)
	}
}

func TestRecordReviewUpdatesCategoryMastery(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	catID, err := q.GetQuestionCategoryID(context.Background(), qids[0])
	if err != nil {
		t.Fatalf("category: %v", err)
	}

	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good); err != nil {
		t.Fatalf("review 1: %v", err)
	}
	m, err := q.GetCategoryMastery(context.Background(), sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
	if err != nil {
		t.Fatalf("mastery: %v", err)
	}
	if m.Seen != 1 || m.Correct != 1 {
		t.Fatalf("mastery after 1 correct = %+v", m)
	}

	if _, err := svc.RecordReview(context.Background(), profileID, qids[1], learning.Again); err != nil {
		t.Fatalf("review 2: %v", err)
	}
	catID2, err := q.GetQuestionCategoryID(context.Background(), qids[1])
	if err != nil {
		t.Fatalf("category 2: %v", err)
	}
	if catID2 == catID {
		m2, err := q.GetCategoryMastery(context.Background(), sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
		if err != nil {
			t.Fatalf("mastery: %v", err)
		}
		if m2.Seen != 2 || m2.Correct != 1 {
			t.Fatalf("mastery after 1 correct + 1 wrong (same category) = %+v", m2)
		}
	}
}

func TestRecordReviewInvalidRating(t *testing.T) {
	_, svc, profileID, qids := seed(t)
	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Rating(99)); err != learning.ErrInvalidRating {
		t.Fatalf("err=%v want ErrInvalidRating", err)
	}
}

func TestRecordReviewSecondTimeUsesStoredState(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	first, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Stability <= first.Stability {
		t.Fatalf("a 2nd successful review must increase stability further: first=%v second=%v", first.Stability, second.Stability)
	}
	if second.Reps != 2 {
		t.Fatalf("Reps = %d, want 2", second.Reps)
	}
	row, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: qids[0]})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	if row.Reps != 2 {
		t.Fatalf("stored reps = %d, want 2", row.Reps)
	}
}
