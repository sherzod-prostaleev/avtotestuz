package explanation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *explanation.Service, uuid.UUID, uuid.UUID) {
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
	svc := explanation.NewService(q, explanation.TemplateDraftGenerator{})
	return q, svc, profile.ID, qids[0]
}

func TestCreateDraftStoresBlocks(t *testing.T) {
	q, svc, _, questionID := seed(t)
	ctx := context.Background()

	if err := svc.CreateDraft(ctx, questionID); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	explID, err := q.GetExplanationIDByQuestionID(ctx, questionID)
	if err != nil {
		t.Fatalf("explanation id: %v", err)
	}
	row, err := q.GetExplanationTranslationByExplanationAndLocale(ctx, sqlc.GetExplanationTranslationByExplanationAndLocaleParams{
		ExplanationID: explID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("translation: %v", err)
	}
	if row.Status != "draft" {
		t.Fatalf("status = %q, want draft", row.Status)
	}
	var blocks []explanation.Block
	if err := json.Unmarshal(row.Blocks, &blocks); err != nil {
		t.Fatalf("unmarshal blocks: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}
}

func TestVerifyMarksTranslationVerified(t *testing.T) {
	q, svc, profileID, questionID := seed(t)
	ctx := context.Background()

	if err := svc.CreateDraft(ctx, questionID); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := svc.Verify(ctx, questionID, profileID); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	explID, err := q.GetExplanationIDByQuestionID(ctx, questionID)
	if err != nil {
		t.Fatalf("explanation id: %v", err)
	}
	row, err := q.GetExplanationTranslationByExplanationAndLocale(ctx, sqlc.GetExplanationTranslationByExplanationAndLocaleParams{
		ExplanationID: explID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("translation: %v", err)
	}
	if row.Status != "verified" || !row.VerifiedBy.Valid {
		t.Fatalf("expected verified status with verified_by set, got %+v", row)
	}
}

func TestVerifyWithoutDraftReturnsNotFound(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	pool := testdb.New(t)
	_, _ = pool.Exec(context.Background(), "DELETE FROM explanation_translation; DELETE FROM explanation;")
	if err := svc.Verify(context.Background(), questionID, profileID); err != explanation.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestRecordFeedbackRequiresExistingExplanation(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	pool := testdb.New(t)
	_, _ = pool.Exec(context.Background(), "DELETE FROM explanation_translation; DELETE FROM explanation;")
	if err := svc.RecordFeedback(context.Background(), profileID, questionID, true); err != explanation.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestRecordFeedbackUpsertsVote(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	ctx := context.Background()
	if err := svc.CreateDraft(ctx, questionID); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := svc.RecordFeedback(ctx, profileID, questionID, true); err != nil {
		t.Fatalf("first feedback: %v", err)
	}
	// changing the vote must upsert, not error or duplicate
	if err := svc.RecordFeedback(ctx, profileID, questionID, false); err != nil {
		t.Fatalf("second feedback: %v", err)
	}
}
