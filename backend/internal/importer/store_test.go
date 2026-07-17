package importer_test

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/testdb"
)

func TestStoreSampleAndIdempotent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	ds, images := fixture.Sample()
	blobs := blob.NewLocalDir(t.TempDir())

	rep, err := importer.Store(ctx, pool, blobs, ds, importer.StoreOptions{
		MarkVerified: true, Images: images, Source: "fixture",
	})
	if err != nil {
		t.Fatalf("store: %v\n%s", err, rep)
	}
	if rep.QuestionsValid != 40 || rep.QuestionsQuarantined != 0 ||
		rep.VariantsStored != 2 || rep.VariantsSkipped != 0 || rep.Signs != 4 {
		t.Fatalf("report: %+v", rep)
	}

	var qCount, vqCount, aCount int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM question").Scan(&qCount)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM variant_question").Scan(&vqCount)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM answer").Scan(&aCount)
	if qCount != 40 || vqCount != 40 || aCount != 160 {
		t.Fatalf("q=%d vq=%d a=%d", qCount, vqCount, aCount)
	}
	// every valid question has correct_answer_id set
	var missing int
	_ = pool.QueryRow(ctx,
		"SELECT count(*) FROM question WHERE validation_status='valid' AND correct_answer_id IS NULL").Scan(&missing)
	if missing != 0 {
		t.Fatalf("%d questions missing correct_answer_id", missing)
	}

	// idempotent re-run: same counts, no duplicates
	if _, err := importer.Store(ctx, pool, blobs, ds, importer.StoreOptions{
		MarkVerified: true, Images: images, Source: "fixture",
	}); err != nil {
		t.Fatalf("re-store: %v", err)
	}
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM question").Scan(&qCount)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM answer").Scan(&aCount)
	if qCount != 40 || aCount != 160 {
		t.Fatalf("after re-run q=%d a=%d", qCount, aCount)
	}
}

func TestStoreQuarantinesBroken(t *testing.T) {
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	ds.Questions[0].Answers = ds.Questions[0].Answers[:3] // break invariant → variant 1 poisoned
	rep, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if rep.QuestionsQuarantined != 1 || rep.VariantsStored != 1 || rep.VariantsSkipped != 1 {
		t.Fatalf("report: %+v", rep)
	}
}
