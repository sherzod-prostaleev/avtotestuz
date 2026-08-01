package push

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

type concurrencySender struct {
	active int32
	max    int32
}

func (s *concurrencySender) Send(_ context.Context, _ Subscription, _ []byte) error {
	active := atomic.AddInt32(&s.active, 1)
	for {
		maxSeen := atomic.LoadInt32(&s.max)
		if active <= maxSeen || atomic.CompareAndSwapInt32(&s.max, maxSeen, active) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	atomic.AddInt32(&s.active, -1)
	return nil
}

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

func TestBroadcastUsesBoundedConcurrency(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	sender := &concurrencySender{}
	svc := NewService(pool, q, Config{
		PublicKey: "public", PrivateKey: "private", Subject: "mailto:test@example.com",
	}, sender)

	for i := 0; i < 16; i++ {
		profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
			Phone: fmt.Sprintf("+99890117%04d", i),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.Subscribe(context.Background(), profile.ID, SubscribeInput{
			Endpoint: fmt.Sprintf("https://push.example/concurrent/%d", i), P256dh: "p", Auth: "a",
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.BroadcastSupport(context.Background(), BroadcastOpts{Title: "T", Body: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Notified != 16 {
		t.Fatalf("notified=%d want 16", result.Notified)
	}
	if maxSeen := atomic.LoadInt32(&sender.max); maxSeen <= 1 || maxSeen > broadcastWorkers {
		t.Fatalf("max concurrency=%d, want 2..%d", maxSeen, broadcastWorkers)
	}
}
