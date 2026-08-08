package broadcast

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/push"
	"avtotest.uz/backend/internal/testdb"
)

func TestValidateAndNormalize(t *testing.T) {
	ch, err := NormalizeChannels("webpush")
	if err != nil || ch != ChannelsBoth {
		t.Fatalf("ch=%q err=%v", ch, err)
	}
	if _, err := SanitizeActionURL("https://evil.example"); err == nil {
		t.Fatal("expected action url reject")
	}
	if _, err := SanitizeImageURL("http://insecure.example/a.png", []string{"cdn.example"}); err == nil {
		t.Fatal("expected http reject")
	}
	if _, err := SanitizeImageURL("https://cdn.example/a.png", []string{"cdn.example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SanitizeImageURL("https://evil.example/a.png", []string{"cdn.example"}); err == nil {
		t.Fatal("expected host reject")
	}
	if _, err := SanitizeImageURL("https://evil.example/a.png", nil); err == nil {
		t.Fatal("expected empty allowlist reject")
	}
	if _, err := SanitizeImageURL("https://evil.example/a.png", []string{"", "  "}); err == nil {
		t.Fatal("expected blank-only allowlist reject")
	}
	if got, err := SanitizeImageURL("", nil); err != nil || got != "" {
		t.Fatalf("empty image must be allowed, got %q err=%v", got, err)
	}
}

func TestCreateExpandDeliverInapp(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	fake := &push.FakeSender{}
	pushSvc := push.NewService(pool, q, push.Config{
		PublicKey: "pub", PrivateKey: "priv", Subject: "mailto:t@example.com",
	}, fake)

	adminID := insertAdmin(t, pool)
	p1, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901880001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901880002"}); err != nil {
		t.Fatal(err)
	}

	svc := &Service{
		Pool: pool,
		Q:    q,
		Push: pushSvc,
		Cfg:  Config{MaxRecipients: 1000},
	}

	counts, err := svc.DryRun(context.Background(), AudienceAllActive, ChannelsBoth)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Recipients < 2 {
		t.Fatalf("recipients=%d", counts.Recipients)
	}

	camp, err := svc.Create(context.Background(), CreateInput{
		AdminID:  adminID,
		Title:    "Hello",
		Body:     "World",
		Audience: AudienceAllActive,
		Channels: ChannelsBoth,
		Confirm:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if camp.Status != "queued" {
		t.Fatalf("status=%s", camp.Status)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := svc.ProcessOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Get(context.Background(), camp.ID)
		if err != nil {
			t.Fatal(err)
		}
		camp = got
		if got.Status == "completed" || got.Status == "completed_with_errors" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if camp.Status != "completed" && camp.Status != "completed_with_errors" {
		t.Fatalf("final status=%s sent=%d pending=%d failed=%d err=%s",
			camp.Status, camp.SentCount, camp.PendingCount, camp.FailedCount, camp.ErrorSummary)
	}
	if camp.SentCount < 2 {
		t.Fatalf("sent=%d want >=2", camp.SentCount)
	}

	unread, err := q.CountUnreadInappNotifications(context.Background(), p1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unread < 1 {
		t.Fatalf("unread=%d", unread)
	}

	// Idempotent create with same key returns same campaign.
	camp2, err := svc.Create(context.Background(), CreateInput{
		AdminID:        adminID,
		Title:          "Hello",
		Body:           "World",
		Audience:       AudienceAllActive,
		Channels:       ChannelsInapp,
		Confirm:        true,
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	camp3, err := svc.Create(context.Background(), CreateInput{
		AdminID:        adminID,
		Title:          "Hello",
		Body:           "World again",
		Audience:       AudienceAllActive,
		Channels:       ChannelsInapp,
		Confirm:        true,
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if camp2.ID != camp3.ID {
		t.Fatalf("idempotency failed: %s vs %s", camp2.ID, camp3.ID)
	}
}

func insertAdmin(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO admin_user (id, email, display_name, password_hash, status)
		VALUES ($1, $2, 'Ops', 'x', 'active')`, id, id.String()+"@example.uz")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
