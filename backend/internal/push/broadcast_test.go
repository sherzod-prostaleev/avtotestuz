package push

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestBroadcastSupportDryRunAndSend(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	fake := &FakeSender{}
	svc := NewService(pool, q, Config{
		PublicKey:  "BPtest-public",
		PrivateKey: "test-private",
		Subject:    "mailto:test@example.com",
	}, fake)

	ctx := context.Background()
	p1, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901160001"})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901160002"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Subscribe(ctx, p1.ID, SubscribeInput{
		Endpoint: "https://push.example/b1", P256dh: "p", Auth: "a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Subscribe(ctx, p2.ID, SubscribeInput{
		Endpoint: "https://push.example/b2", P256dh: "p", Auth: "a",
	}); err != nil {
		t.Fatal(err)
	}

	dry, err := svc.BroadcastSupport(ctx, BroadcastOpts{
		Title: "Hi", Body: "News", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Recipients != 2 || dry.Notified != 0 || !dry.DryRun {
		t.Fatalf("dry=%+v", dry)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("dry run should not send, got %d", len(fake.Calls))
	}

	live, err := svc.BroadcastSupport(ctx, BroadcastOpts{
		Title: "Hi", Body: "News", URL: "/uz-Latn/dashboard", DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live.Recipients != 2 || live.Notified != 2 || live.Deliveries < 2 {
		t.Fatalf("live=%+v sent=%d", live, len(fake.Calls))
	}
}

func TestBroadcastRequiresConfig(t *testing.T) {
	svc, _, _ := newTestService(t, false)
	_, err := svc.BroadcastSupport(context.Background(), BroadcastOpts{Body: "x"})
	if err != ErrUnconfigured {
		t.Fatalf("err=%v", err)
	}
}
