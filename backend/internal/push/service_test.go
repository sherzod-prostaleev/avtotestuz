package push

import (
	"context"
	"encoding/json"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func newTestService(t *testing.T, configured bool) (*Service, *sqlc.Queries, *FakeSender) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	fake := &FakeSender{}
	cfg := Config{}
	if configured {
		cfg = Config{
			PublicKey:  "BPtest-public-key-not-real",
			PrivateKey: "test-private-key-not-real",
			Subject:    "mailto:test@example.com",
		}
	}
	return NewService(pool, q, cfg, fake), q, fake
}

func TestSubscribeRequiresConfig(t *testing.T) {
	svc, q, _ := newTestService(t, false)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901140001"})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Subscribe(context.Background(), profile.ID, SubscribeInput{
		Endpoint: "https://push.example/sub/1",
		P256dh:   "p",
		Auth:     "a",
	})
	if err != ErrUnconfigured {
		t.Fatalf("err = %v, want ErrUnconfigured", err)
	}
}

func TestSubscribeAndStatus(t *testing.T) {
	svc, q, _ := newTestService(t, true)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901140002"})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Subscribe(context.Background(), profile.ID, SubscribeInput{
		Endpoint:  "https://push.example/sub/2",
		P256dh:    "p256",
		Auth:      "auth",
		UserAgent: "vitest",
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Configured || !st.Subscribed || st.SubscriptionCount != 1 {
		t.Fatalf("status = %+v", st)
	}
	if st.VAPIDPublicKey == "" {
		t.Fatal("want vapid public key in status")
	}
}

func TestNotifySendsAndMarks(t *testing.T) {
	svc, q, fake := newTestService(t, true)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901140003"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Subscribe(context.Background(), profile.ID, SubscribeInput{
		Endpoint: "https://push.example/sub/3",
		P256dh:   "p",
		Auth:     "a",
	}); err != nil {
		t.Fatal(err)
	}
	sent, err := svc.Notify(context.Background(), profile.ID, "push_test", NotifyPayload{
		Title: "T", Body: "B",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Fatalf("sent = %d", sent)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("calls = %d", len(fake.Calls))
	}
	var payload NotifyPayload
	if err := json.Unmarshal(fake.Calls[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Title != "T" || payload.Body != "B" {
		t.Fatalf("payload = %+v", payload)
	}
	var sentAt *string
	err = svc.Pool.QueryRow(context.Background(),
		`SELECT sent_at::text FROM notification WHERE profile_id = $1 AND channel = 'webpush'`,
		profile.ID).Scan(&sentAt)
	if err != nil {
		t.Fatal(err)
	}
	if sentAt == nil {
		t.Fatal("want sent_at set")
	}
}

func TestUnsubscribe(t *testing.T) {
	svc, q, _ := newTestService(t, true)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901140004"})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "https://push.example/sub/4"
	if err := svc.Subscribe(context.Background(), profile.ID, SubscribeInput{
		Endpoint: endpoint, P256dh: "p", Auth: "a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Unsubscribe(context.Background(), profile.ID, endpoint); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Subscribed {
		t.Fatal("want unsubscribed")
	}
}
