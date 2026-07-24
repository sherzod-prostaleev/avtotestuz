package demo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/demo"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func seedFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func newService(t *testing.T, pool *pgxpool.Pool) (*demo.Service, *sqlc.Queries) {
	t.Helper()
	q := sqlc.New(pool)
	ch := &content.Handler{Q: q, MediaBase: "http://media.test"}
	c := redisx.NewTest(t)
	return demo.NewService(q, ch, auth.Limiter{R: c}), q
}

// whitelistIDs recomputes the expected demo whitelist independently of the
// service, straight from the DB (variant 1's ordered question ids, first
// two), so tests don't just assert "whatever the service says" but the
// actual brief rule.
func whitelistIDs(t *testing.T, ctx context.Context, q *sqlc.Queries) []uuid.UUID {
	t.Helper()
	v, err := q.GetVariantByNumber(ctx, 1)
	if err != nil {
		t.Fatalf("GetVariantByNumber: %v", err)
	}
	ids, err := q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		t.Fatalf("ListVariantQuestionIDsOrdered: %v", err)
	}
	if len(ids) < 2 {
		t.Fatalf("fixture variant 1 has too few questions: %d", len(ids))
	}
	return ids[:2]
}

func TestGetQuestionReturnsWhitelistedQuestion(t *testing.T) {
	pool := testdb.New(t)
	seedFixture(t, pool)
	svc, q := newService(t, pool)
	ctx := context.Background()
	wl := whitelistIDs(t, ctx, q)

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		detail, _, err := svc.GetQuestion(ctx, "uz-Latn")
		if err != nil {
			t.Fatalf("GetQuestion: %v", err)
		}
		found := false
		for _, w := range wl {
			if w.String() == detail.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("returned question %s is not in the demo whitelist %v", detail.ID, wl)
		}
		if detail.Explanation != nil {
			t.Fatal("pre-grade demo question must not expose answer-revealing explanation prose")
		}
		seen[detail.ID] = true
	}
	// Not a strict requirement of the brief, but with 20 draws from a 2-item
	// whitelist we should see both at least once (guards against a
	// mis-wired "always pick index 0").
	if len(seen) < 2 {
		t.Fatalf("expected random selection to surface both whitelisted questions across 20 draws, saw %v", seen)
	}
}

func TestGetQuestionEmptyDBNotFound(t *testing.T) {
	pool := testdb.New(t) // no seed
	_, _ = pool.Exec(context.Background(), "DELETE FROM question_sign; DELETE FROM variant_question; DELETE FROM question_memory; DELETE FROM saved_question; DELETE FROM session_answer; DELETE FROM answer_translation; DELETE FROM answer; DELETE FROM question_translation; DELETE FROM question;")
	svc, _ := newService(t, pool)

	_, _, err := svc.GetQuestion(context.Background(), "uz-Latn")
	if !errors.Is(err, demo.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestSubmitAnswerCorrectAndWrong(t *testing.T) {
	pool := testdb.New(t)
	seedFixture(t, pool)
	svc, q := newService(t, pool)
	ctx := context.Background()
	wl := whitelistIDs(t, ctx, q)
	qid := wl[0]

	ans, err := q.ListAnswersByQuestionIDs(ctx, sqlc.ListAnswersByQuestionIDsParams{
		QuestionIds: []uuid.UUID{qid}, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatal(err)
	}
	correctID, err := q.GetCorrectAnswerID(ctx, qid)
	if err != nil {
		t.Fatal(err)
	}
	var wrongID uuid.UUID
	for _, a := range ans {
		if a.ID != correctID {
			wrongID = a.ID
			break
		}
	}
	if wrongID == uuid.Nil {
		t.Fatal("fixture question must have a wrong answer to test against")
	}

	correct, gotCorrectID, err := svc.SubmitAnswer(ctx, "1.2.3.4", qid, correctID)
	if err != nil {
		t.Fatalf("SubmitAnswer(correct): %v", err)
	}
	if !correct || gotCorrectID != correctID {
		t.Fatalf("correct=%v gotCorrectID=%s want true/%s", correct, gotCorrectID, correctID)
	}

	correct, gotCorrectID, err = svc.SubmitAnswer(ctx, "1.2.3.5", qid, wrongID)
	if err != nil {
		t.Fatalf("SubmitAnswer(wrong): %v", err)
	}
	if correct || gotCorrectID != correctID {
		t.Fatalf("correct=%v gotCorrectID=%s want false/%s", correct, gotCorrectID, correctID)
	}
}

func TestSubmitAnswerNotWhitelistedQuestion(t *testing.T) {
	pool := testdb.New(t)
	seedFixture(t, pool)
	svc, q := newService(t, pool)
	ctx := context.Background()

	v, err := q.GetVariantByNumber(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.ListVariantQuestionIDsOrdered(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) <= 2 {
		t.Fatal("fixture must have more than 2 questions in variant 1 for this test")
	}
	outsideID := ids[2] // 3rd question, position-wise — real, but not whitelisted

	correctID, err := q.GetCorrectAnswerID(ctx, outsideID)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = svc.SubmitAnswer(ctx, "9.9.9.9", outsideID, correctID)
	if !errors.Is(err, demo.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound (question exists but is outside the demo whitelist)", err)
	}
}

func TestSubmitAnswerInvalidAnswer(t *testing.T) {
	pool := testdb.New(t)
	seedFixture(t, pool)
	svc, q := newService(t, pool)
	ctx := context.Background()
	wl := whitelistIDs(t, ctx, q)

	_, _, err := svc.SubmitAnswer(ctx, "9.9.9.8", wl[0], uuid.New())
	if !errors.Is(err, demo.ErrInvalidAnswer) {
		t.Fatalf("err=%v want ErrInvalidAnswer", err)
	}
}

func TestSubmitAnswerEmptyDBNotFound(t *testing.T) {
	pool := testdb.New(t) // no seed
	svc, _ := newService(t, pool)

	_, _, err := svc.SubmitAnswer(context.Background(), "1.1.1.1", uuid.New(), uuid.New())
	if !errors.Is(err, demo.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestSubmitAnswerRateLimited(t *testing.T) {
	pool := testdb.New(t)
	seedFixture(t, pool)
	svc, q := newService(t, pool)
	ctx := context.Background()
	wl := whitelistIDs(t, ctx, q)
	correctID, err := q.GetCorrectAnswerID(ctx, wl[0])
	if err != nil {
		t.Fatal(err)
	}

	const limit = 60
	ip := "5.5.5.5"
	for i := 0; i < limit; i++ {
		if _, _, err := svc.SubmitAnswer(ctx, ip, wl[0], correctID); err != nil {
			t.Fatalf("attempt %d should be allowed: %v", i+1, err)
		}
	}
	if _, _, err := svc.SubmitAnswer(ctx, ip, wl[0], correctID); !errors.Is(err, demo.ErrRateLimited) {
		t.Fatalf("attempt %d: err=%v want ErrRateLimited", limit+1, err)
	}

	// A different IP is an independent bucket.
	if _, _, err := svc.SubmitAnswer(ctx, "6.6.6.6", wl[0], correctID); err != nil {
		t.Fatalf("different IP should not be rate limited: %v", err)
	}
}
