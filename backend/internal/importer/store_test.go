package importer_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

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

// TestStoreFiveAnswerCorrectAtPosition5 guards against a regression where a
// question with 5 answers whose CORRECT answer sits at position 5 was
// silently dropped by the importer (store.go rejected position>4, which the
// validator already permits up to 5 answers). Dropping that answer would
// have stored the question with zero correct answers — silent data
// corruption on real licensed exam content.
func TestStoreFiveAnswerCorrectAtPosition5(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	ds, images := fixture.Sample()
	blobs := blob.NewLocalDir(t.TempDir())

	texts := func(latn, cyrl, ru string) map[string]string {
		return map[string]string{"uz-Latn": latn, "uz-Cyrl": cyrl, "ru": ru}
	}

	fiveAnswerQ := importer.CanonQuestion{
		ExtID:    "test-5ans-correct-at-5",
		Category: "signs",
		Texts:    texts("Savol (5 javob)", "Савол (5 жавоб)", "Вопрос (5 ответов)"),
		Answers: []importer.CanonAnswer{
			{Position: 1, Correct: false, Texts: texts("Javob 1", "Жавоб 1", "Ответ 1")},
			{Position: 2, Correct: false, Texts: texts("Javob 2", "Жавоб 2", "Ответ 2")},
			{Position: 3, Correct: false, Texts: texts("Javob 3", "Жавоб 3", "Ответ 3")},
			{Position: 4, Correct: false, Texts: texts("Javob 4", "Жавоб 4", "Ответ 4")},
			{Position: 5, Correct: true, Texts: texts("Javob 5 (to'g'ri)", "Жавоб 5 (тўғри)", "Ответ 5 (правильный)")},
		},
	}
	ds.Questions = append(ds.Questions, fiveAnswerQ)

	rep, err := importer.Store(ctx, pool, blobs, ds, importer.StoreOptions{
		MarkVerified: true, Images: images, Source: "fixture",
	})
	if err != nil {
		t.Fatalf("store: %v\n%s", err, rep)
	}
	if rep.QuestionsQuarantined != 0 {
		t.Fatalf("expected no quarantined questions, report: %+v", rep)
	}

	var answerCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM answer a JOIN question q ON q.id = a.question_id WHERE q.source_ext_id = $1`,
		fiveAnswerQ.ExtID).Scan(&answerCount); err != nil {
		t.Fatalf("count answers: %v", err)
	}
	if answerCount != 5 {
		t.Fatalf("expected 5 answers stored, got %d — 5th answer was silently dropped", answerCount)
	}

	var pos5AnswerID uuid.UUID
	var pos5Correct bool
	if err := pool.QueryRow(ctx,
		`SELECT a.id, a.is_correct FROM answer a JOIN question q ON q.id = a.question_id
		 WHERE q.source_ext_id = $1 AND a.position = 5`,
		fiveAnswerQ.ExtID).Scan(&pos5AnswerID, &pos5Correct); err != nil {
		t.Fatalf("query position-5 answer: %v (5th answer missing from DB)", err)
	}
	if !pos5Correct {
		t.Fatalf("position-5 answer should be marked correct in the answer row")
	}

	var correctAnswerID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT correct_answer_id FROM question WHERE source_ext_id = $1`,
		fiveAnswerQ.ExtID).Scan(&correctAnswerID); err != nil {
		t.Fatalf("query question.correct_answer_id: %v", err)
	}
	if correctAnswerID != pos5AnswerID {
		t.Fatalf("question.correct_answer_id = %s, want position-5 answer %s (correct answer at position 5 not linked — question would have zero correct answers)",
			correctAnswerID, pos5AnswerID)
	}
}

func TestStoreQuarantinesBroken(t *testing.T) {
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	ds.Questions[0].Answers = ds.Questions[0].Answers[:1] // break invariant (below valid 2-5 range) → variant 1 poisoned
	rep, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if rep.QuestionsQuarantined != 1 || rep.VariantsStored != 1 || rep.VariantsSkipped != 1 {
		t.Fatalf("report: %+v", rep)
	}
}

// Omitting the signs field on reimport must not wipe question_sign rows that
// were applied out-of-band (linkquestionsigns / prior import with signs set).
func TestStorePreservesQuestionSignsWhenOmitted(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	ds, images := fixture.Sample()
	blobs := blob.NewLocalDir(t.TempDir())

	if _, err := importer.Store(ctx, pool, blobs, ds, importer.StoreOptions{
		MarkVerified: true, Images: images, Source: "fixture",
	}); err != nil {
		t.Fatalf("initial store: %v", err)
	}

	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM question_sign`).Scan(&before); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if before == 0 {
		t.Fatal("fixture sample is expected to seed some question_sign rows")
	}

	for i := range ds.Questions {
		ds.Questions[i].Signs = nil
	}
	if _, err := importer.Store(ctx, pool, blobs, ds, importer.StoreOptions{
		MarkVerified: true, Images: images, Source: "fixture",
	}); err != nil {
		t.Fatalf("re-store without signs: %v", err)
	}

	var after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM question_sign`).Scan(&after); err != nil {
		t.Fatalf("recount links: %v", err)
	}
	if after != before {
		t.Fatalf("question_sign wiped on omit: before=%d after=%d", before, after)
	}
}
