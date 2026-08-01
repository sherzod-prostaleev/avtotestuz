package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/testdb"
)

func TestLogBatchInsertsEvents(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	svc := events.NewService(q, pool)

	batchKey := uuid.NewString()
	err = svc.LogBatch(context.Background(), profile.ID, batchKey, []events.Event{
		{Name: "view_question", Props: json.RawMessage(`{"question_id":"x"}`)},
		{Name: "session_finish"},
	})
	if err != nil {
		t.Fatalf("LogBatch: %v", err)
	}
	if err := svc.LogBatch(context.Background(), profile.ID, batchKey, []events.Event{
		{Name: "view_question", Props: json.RawMessage(`{"question_id":"x"}`)},
		{Name: "session_finish"},
	}); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	var eventCount, batchCount int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*)::int FROM event WHERE profile_id=$1`, profile.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*)::int FROM event_batch WHERE profile_id=$1`, profile.ID).Scan(&batchCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 || batchCount != 1 {
		t.Fatalf("events=%d batches=%d want 2/1", eventCount, batchCount)
	}
}

func TestLogBatchRollsBackMarkerAndEarlierEventsOnInsertFailure(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234568"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION test_fail_event_insert() RETURNS trigger AS $$
		BEGIN
		  IF NEW.name = 'force_failure' THEN RAISE EXCEPTION 'forced event failure'; END IF;
		  RETURN NEW;
		END $$ LANGUAGE plpgsql;
		CREATE TRIGGER test_fail_event_insert_trigger BEFORE INSERT ON event_default
		FOR EACH ROW EXECUTE FUNCTION test_fail_event_insert()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_fail_event_insert_trigger ON event_default`)
		_, _ = pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS test_fail_event_insert()`)
	})
	svc := events.NewService(q, pool)
	err = svc.LogBatch(context.Background(), profile.ID, uuid.NewString(), []events.Event{
		{Name: "first_valid"},
		{Name: "force_failure"},
	})
	if err == nil {
		t.Fatal("expected forced insert failure")
	}
	var eventsN, batchesN int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*)::int FROM event WHERE profile_id=$1`, profile.ID).Scan(&eventsN); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*)::int FROM event_batch WHERE profile_id=$1`, profile.ID).Scan(&batchesN); err != nil {
		t.Fatal(err)
	}
	if eventsN != 0 || batchesN != 0 {
		t.Fatalf("partial write remained: events=%d batches=%d", eventsN, batchesN)
	}
}

func TestLogBatchRejectsEmpty(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q, pool)
	if err := svc.LogBatch(context.Background(), uuid.New(), uuid.NewString(), nil); err != events.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestLogBatchRejectsOversized(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q, pool)
	big := make([]events.Event, 101)
	for i := range big {
		big[i] = events.Event{Name: "x"}
	}
	if err := svc.LogBatch(context.Background(), uuid.New(), uuid.NewString(), big); err != events.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}
