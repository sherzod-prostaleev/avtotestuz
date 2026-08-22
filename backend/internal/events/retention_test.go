package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/testdb"
)

// event_batch exists only to make a retried POST /events idempotent: one row per
// (profile, client-generated key), checked with ON CONFLICT DO NOTHING. Nothing
// ever removed those rows, so the table grew for the life of the database to
// remember keys no client will ever present again -- 85,247 rows at audit time,
// second-largest table in the schema, purely bookkeeping.
//
// These tests pin what the sweep must and must not do. The "must not" half
// matters more: deleting a key that is still inside a client's retry window
// would let a retried batch be counted twice, which is the exact bug the table
// was added to prevent.
func TestPurgeExpiredBatchesKeepsTheRetryWindow(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	svc := events.NewService(q, pool)

	profile, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901994001"})
	if err != nil {
		t.Fatal(err)
	}

	insert := func(age time.Duration) uuid.UUID {
		key := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO event_batch (profile_id, idempotency_key, event_count, created_at)
			VALUES ($1, $2, 1, now() - $3::interval)`,
			profile.ID, key, age.String()); err != nil {
			t.Fatal(err)
		}
		return key
	}

	fresh := insert(1 * time.Hour)
	edge := insert(6 * 24 * time.Hour)
	stale := insert(30 * 24 * time.Hour)

	removed, err := svc.PurgeExpiredBatches(ctx, 7*24*time.Hour, 100)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed=%d want 1", removed)
	}

	exists := func(key uuid.UUID) bool {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM event_batch WHERE profile_id=$1 AND idempotency_key=$2`,
			profile.ID, key).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	}

	if !exists(fresh) {
		t.Error("an hour-old key was purged; a retry of that batch would double-count")
	}
	if !exists(edge) {
		t.Error("a key inside the window was purged")
	}
	if exists(stale) {
		t.Error("a 30-day-old key survived the sweep")
	}
}

// The first sweep on a table nothing has ever pruned has to remove everything
// accumulated so far. Doing that in one statement would take a long lock and
// write one enormous WAL record, so the sweep works in bounded chunks and loops.
// This proves the loop actually drains rather than stopping after one chunk.
func TestPurgeExpiredBatchesDrainsInChunks(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	svc := events.NewService(q, pool)

	profile, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901994002"})
	if err != nil {
		t.Fatal(err)
	}
	const total = 25
	for i := 0; i < total; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO event_batch (profile_id, idempotency_key, event_count, created_at)
			VALUES ($1, $2, 1, now() - interval '40 days')`,
			profile.ID, uuid.New()); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := svc.PurgeExpiredBatches(ctx, 7*24*time.Hour, 4)
	if err != nil {
		t.Fatal(err)
	}
	if removed != total {
		t.Fatalf("removed=%d want %d -- the chunk loop stopped early", removed, total)
	}

	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_batch`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d rows left after the sweep", left)
	}
}

// A zero or negative retention would delete keys that are still live, so it is
// refused rather than silently treated as "purge everything".
func TestPurgeExpiredBatchesRejectsNonPositiveRetention(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q, pool)

	if _, err := svc.PurgeExpiredBatches(ctx, 0, 100); err == nil {
		t.Error("zero retention was accepted")
	}
	if _, err := svc.PurgeExpiredBatches(ctx, -time.Hour, 100); err == nil {
		t.Error("negative retention was accepted")
	}
}
