package push

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/testdb"
)

func seedDigestFixture(t *testing.T) (*Service, *sqlc.Queries, *FakeSender) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	q := sqlc.New(pool)
	fake := &FakeSender{}
	svc := NewService(pool, q, Config{
		PublicKey:  "BPtest-public-key-not-real",
		PrivateKey: "test-private-key-not-real",
		Subject:    "mailto:test@example.com",
	}, fake)
	return svc, q, fake
}

func TestRunFSRSDueDigestDryRunAndSend(t *testing.T) {
	svc, q, fake := seedDigestFixture(t)
	ctx := context.Background()

	profile, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901150001"})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := q.RandomQuestionIDs(ctx, 3)
	if err != nil || len(ids) < 2 {
		t.Fatalf("questions: %v len=%d", err, len(ids))
	}
	for _, qid := range ids[:2] {
		if _, err := q.UpsertQuestionMemory(ctx, sqlc.UpsertQuestionMemoryParams{
			ProfileID:      profile.ID,
			QuestionID:     qid,
			Stability:      0.1,
			Difficulty:     5,
			DueAt:          pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
			LastReviewedAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			Reps:           1,
			Lapses:         1,
			State:          1,
		}); err != nil {
			t.Fatalf("memory: %v", err)
		}
	}
	if err := svc.Subscribe(ctx, profile.ID, SubscribeInput{
		Endpoint: "https://push.example/digest/1",
		P256dh:   "p",
		Auth:     "a",
	}); err != nil {
		t.Fatal(err)
	}

	dry, err := svc.RunFSRSDueDigest(ctx, DigestOpts{DryRun: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Candidates != 1 || dry.Notified != 0 {
		t.Fatalf("dry = %+v", dry)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("dry-run must not send, calls=%d", len(fake.Calls))
	}

	sent, err := svc.RunFSRSDueDigest(ctx, DigestOpts{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if sent.Candidates != 1 || sent.Notified != 1 || sent.Deliveries != 1 || sent.Errors != 0 {
		t.Fatalf("send = %+v", sent)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("calls = %d", len(fake.Calls))
	}
	var payload NotifyPayload
	if err := json.Unmarshal(fake.Calls[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Title == "" || payload.Body == "" {
		t.Fatalf("empty copy: %+v", payload)
	}
	if payload.URL != "/uz-Latn/session/start?mode=review&count=2" {
		t.Fatalf("url = %q", payload.URL)
	}

	// Cooldown: second run should skip the same profile.
	again, err := svc.RunFSRSDueDigest(ctx, DigestOpts{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if again.Candidates != 0 {
		t.Fatalf("cooldown candidates = %+v", again)
	}
}

func TestRunFSRSDueDigestRequiresVAPID(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := NewService(pool, q, Config{}, &FakeSender{})
	_, err := svc.RunFSRSDueDigest(context.Background(), DigestOpts{})
	if err != ErrUnconfigured {
		t.Fatalf("err = %v, want ErrUnconfigured", err)
	}
}

func TestFSRSDueCopyLocales(t *testing.T) {
	ru := fsrsDuePayload("ru", 7)
	if ru.URL != "/ru/session/start?mode=review&count=7" {
		t.Fatalf("ru url = %q", ru.URL)
	}
	if ru.Title != "Время повторения" {
		t.Fatalf("ru title = %q", ru.Title)
	}
	capped := fsrsDuePayload("uz-Latn", 99)
	if capped.URL != "/uz-Latn/session/start?mode=review&count=20" {
		t.Fatalf("capped url = %q", capped.URL)
	}
}
