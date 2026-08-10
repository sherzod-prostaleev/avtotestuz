package broadcast

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/push"
	"avtotest.uz/backend/internal/testdb"
)

func TestStaleProcessingReclaimAndExactlyOnce(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	adminID := insertAdmin(t, pool)
	p, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901992001"})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		Pool: pool,
		Q:    q,
		Cfg:  Config{MaxRecipients: 1000, ProcessingLease: time.Millisecond},
	}
	camp, err := svc.Create(context.Background(), CreateInput{
		AdminID: adminID, Title: "T", Body: "B", Audience: AudienceAllActive,
		Channels: ChannelsInapp, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.expandQueued(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rid uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		SELECT id FROM broadcast_recipient WHERE campaign_id=$1 AND profile_id=$2`,
		camp.ID, p.ID).Scan(&rid); err != nil {
		t.Fatal(err)
	}
	// Simulate claim + crash: stuck in processing with a stale lease.
	_, err = pool.Exec(context.Background(), `
		UPDATE broadcast_recipient
		SET status='processing', attempt_count=1, updated_at=now() - interval '1 hour'
		WHERE id=$1`, rid)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(context.Background(), `
		UPDATE broadcast_campaign SET status='sending', finished_at=NULL WHERE id=$1`, camp.ID)

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := svc.ProcessOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		got, _ := svc.Get(context.Background(), camp.ID)
		if got.Status == "completed" || got.Status == "completed_with_errors" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM notification
		WHERE campaign_id=$1 AND profile_id=$2 AND channel='inapp'`, camp.ID, p.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("notifications=%d want exactly 1", n)
	}
	var st string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM broadcast_recipient WHERE id=$1`, rid).Scan(&st); err != nil {
		t.Fatal(err)
	}
	if st != "sent" {
		t.Fatalf("recipient status=%s want sent", st)
	}
}

func TestConcurrentReclaimNoDuplicateNotification(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	adminID := insertAdmin(t, pool)
	fake := &push.FakeSender{}
	pushSvc := push.NewService(pool, q, push.Config{
		PublicKey: "p", PrivateKey: "s", Subject: "mailto:t@t",
	}, fake)
	for i := 0; i < 12; i++ {
		if _, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
			Phone: "+99893" + uuid.NewString()[:8],
		}); err != nil {
			t.Fatal(err)
		}
	}
	svc := &Service{
		Pool: pool, Q: q, Push: pushSvc,
		Cfg: Config{MaxRecipients: 1000, ProcessingLease: time.Millisecond},
	}
	camp, err := svc.Create(context.Background(), CreateInput{
		AdminID: adminID, Title: "T", Body: "B", Audience: AudienceAllActive,
		Channels: ChannelsInapp, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.expandQueued(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `
		UPDATE broadcast_recipient
		SET status='processing', attempt_count=1, updated_at=now() - interval '1 hour'
		WHERE campaign_id=$1`, camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(context.Background(), `
		UPDATE broadcast_campaign SET status='sending', finished_at=NULL WHERE id=$1`, camp.ID)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				_ = svc.ProcessOnce(context.Background())
				got, _ := svc.Get(context.Background(), camp.ID)
				if got.Status == "completed" || got.Status == "completed_with_errors" {
					return
				}
				time.Sleep(15 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	var dup int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM (
		  SELECT profile_id FROM notification
		  WHERE campaign_id=$1 AND channel='inapp'
		  GROUP BY profile_id HAVING count(*) > 1
		) x`, camp.ID).Scan(&dup); err != nil {
		t.Fatal(err)
	}
	if dup != 0 {
		t.Fatalf("duplicate inapp rows for %d profiles", dup)
	}
	var stuck int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM broadcast_recipient
		WHERE campaign_id=$1 AND status='processing'`, camp.ID).Scan(&stuck); err != nil {
		t.Fatal(err)
	}
	if stuck != 0 {
		t.Fatalf("stuck processing=%d", stuck)
	}
}

func TestEmptyImageAllowlistRejectedOnCreate(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	adminID := insertAdmin(t, pool)
	svc := &Service{Pool: pool, Q: q, Cfg: Config{MaxRecipients: 1000, ImageHosts: nil}}
	_, err := svc.Create(context.Background(), CreateInput{
		AdminID: adminID, Title: "T", Body: "B",
		ImageURL: "https://cdn.example/a.png",
		Audience: AudienceAllActive, Channels: ChannelsInapp, Confirm: true,
	})
	if err == nil {
		t.Fatal("expected image allowlist rejection")
	}
}
