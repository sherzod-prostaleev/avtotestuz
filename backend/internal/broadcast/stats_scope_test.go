package broadcast

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// TestRefreshStatsSkipsTerminalCampaigns pins the scope of the stats sweep.
//
// refreshCampaignStats runs on every worker tick -- every 2 seconds, forever,
// whether or not anything is being delivered. It used to aggregate
// broadcast_recipient with no WHERE at all and let the join filter afterwards,
// so a finished campaign's rows were re-counted for the life of the database.
// EXPLAIN showed it plainly: HashAggregate over a Seq Scan of the whole table.
// One campaign to a 100k-user base is 100k rows, and they would be scanned
// 43,200 times a day to rewrite counters that can no longer change.
//
// The fix narrows the aggregate to campaigns that are still 'sending'. This
// test proves the narrowing by planting counters that are deliberately wrong on
// a finished campaign: if the sweep still touched it, it would "helpfully"
// correct them, and that correction is the observable signature of the full
// scan. A live campaign in the same database must still be recomputed.
func TestRefreshStatsSkipsTerminalCampaigns(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	adminID := insertAdmin(t, pool)

	if _, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901993001"}); err != nil {
		t.Fatal(err)
	}

	svc := &Service{
		Pool: pool,
		Q:    q,
		Cfg:  Config{MaxRecipients: 1000, ProcessingLease: time.Millisecond},
	}

	newCampaign := func(title string) uuid.UUID {
		c, err := svc.Create(ctx, CreateInput{
			AdminID: adminID, Title: title, Body: "B", Audience: AudienceAllActive,
			Channels: ChannelsInapp, Confirm: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.expandQueued(ctx); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}

	finished := newCampaign("finished")
	live := newCampaign("live")

	// `finished` looks like a campaign that drained: every recipient sent, and
	// counters that disagree with reality. Nothing in the product can produce
	// this state -- it exists only so a recount is detectable.
	if _, err := pool.Exec(ctx, `
		UPDATE broadcast_recipient SET status='sent' WHERE campaign_id=$1`, finished); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE broadcast_campaign
		SET status='completed', pending_count=0, sent_count=999, failed_count=0,
		    finished_at=now()
		WHERE id=$1`, finished); err != nil {
		t.Fatal(err)
	}

	// `live` is mid-delivery: still sending, recipients still pending.
	if _, err := pool.Exec(ctx, `
		UPDATE broadcast_campaign
		SET status='sending', sent_count=0, pending_count=0, finished_at=NULL
		WHERE id=$1`, live); err != nil {
		t.Fatal(err)
	}

	if err := svc.refreshCampaignStats(ctx); err != nil {
		t.Fatal(err)
	}

	var sentOnFinished int
	if err := pool.QueryRow(ctx,
		`SELECT sent_count FROM broadcast_campaign WHERE id=$1`, finished).Scan(&sentOnFinished); err != nil {
		t.Fatal(err)
	}
	if sentOnFinished != 999 {
		t.Fatalf("finished campaign was recounted (sent_count=%d, want the planted 999): "+
			"the sweep is still aggregating recipients of terminal campaigns", sentOnFinished)
	}

	var pendingOnLive int
	var statusOnLive string
	if err := pool.QueryRow(ctx,
		`SELECT pending_count, status FROM broadcast_campaign WHERE id=$1`, live).
		Scan(&pendingOnLive, &statusOnLive); err != nil {
		t.Fatal(err)
	}
	if pendingOnLive == 0 {
		t.Fatal("live campaign was not recomputed: narrowing the sweep must not skip 'sending'")
	}
	if statusOnLive != "sending" {
		t.Fatalf("live campaign status=%s want sending", statusOnLive)
	}
}

// TestRefreshStatsStillFinalisesDrainedCampaign guards the transition the sweep
// exists for: the last recipient lands, and the campaign must move out of
// 'sending' on the next tick. Narrowing the aggregate to 'sending' campaigns
// keeps this working precisely because the campaign is still 'sending' at the
// moment it drains -- it is the tick after that skips it.
func TestRefreshStatsStillFinalisesDrainedCampaign(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	adminID := insertAdmin(t, pool)

	if _, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901993002"}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{Pool: pool, Q: q, Cfg: Config{MaxRecipients: 1000, ProcessingLease: time.Millisecond}}

	camp, err := svc.Create(ctx, CreateInput{
		AdminID: adminID, Title: "T", Body: "B", Audience: AudienceAllActive,
		Channels: ChannelsInapp, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.expandQueued(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE broadcast_campaign SET status='sending', finished_at=NULL WHERE id=$1`, camp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE broadcast_recipient SET status='sent' WHERE campaign_id=$1`, camp.ID); err != nil {
		t.Fatal(err)
	}

	if err := svc.refreshCampaignStats(ctx); err != nil {
		t.Fatal(err)
	}

	var status string
	var finishedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT status, finished_at FROM broadcast_campaign WHERE id=$1`, camp.ID).
		Scan(&status, &finishedAt); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("status=%s want completed", status)
	}
	if finishedAt == nil {
		t.Fatal("finished_at not stamped")
	}
}
