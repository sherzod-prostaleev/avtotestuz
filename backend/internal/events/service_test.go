package events_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

// TestLogBatchMultiRowInsertPreservesPerEventData exercises the batch-insert
// path (PERF-2: one COPY instead of one INSERT per event) with more than one
// event and asserts every row's name, props (including the nil->'{}'
// default) and explicit ts survive the round trip intact and attributed to
// the right profile.
func TestLogBatchMultiRowInsertPreservesPerEventData(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234569"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	svc := events.NewService(q, pool)

	explicitTS := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Millisecond)
	batch := []events.Event{
		{Name: "view_question", Props: json.RawMessage(`{"question_id":"a"}`)},
		{Name: "answer_submit", Props: json.RawMessage(`{"correct":true}`)},
		{Name: "session_finish"}, // nil props -> defaults to '{}'
		{Name: "session_start", Props: json.RawMessage(`{}`), TS: &explicitTS},
		{Name: "app_open"},
	}
	if err := svc.LogBatch(context.Background(), profile.ID, uuid.NewString(), batch); err != nil {
		t.Fatalf("LogBatch: %v", err)
	}

	rows, err := pool.Query(context.Background(),
		`SELECT name, props::text, ts FROM event WHERE profile_id=$1 ORDER BY name`, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type got struct {
		name  string
		props string
		ts    time.Time
	}
	var results []got
	for rows.Next() {
		var g got
		if err := rows.Scan(&g.name, &g.props, &g.ts); err != nil {
			t.Fatal(err)
		}
		results = append(results, g)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(results) != len(batch) {
		t.Fatalf("got %d rows, want %d: %+v", len(results), len(batch), results)
	}

	byName := make(map[string]got, len(results))
	for _, g := range results {
		byName[g.name] = g
	}
	want := map[string]string{
		"view_question":  `{"question_id": "a"}`,
		"answer_submit":  `{"correct": true}`,
		"session_finish": "{}",
		"session_start":  "{}",
		"app_open":       "{}",
	}
	for name, wantProps := range want {
		g, ok := byName[name]
		if !ok {
			t.Fatalf("missing event %q in %+v", name, byName)
		}
		if g.props != wantProps {
			t.Fatalf("event %q props=%q want %q", name, g.props, wantProps)
		}
	}
	if !byName["session_start"].ts.Equal(explicitTS) {
		t.Fatalf("session_start ts=%v want %v", byName["session_start"].ts, explicitTS)
	}
	if byName["app_open"].ts.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("app_open ts=%v want ~now (defaulted)", byName["app_open"].ts)
	}
}

func TestLogBatchRollsBackMarkerAndEarlierEventsOnInsertFailure(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234568"})
	if err != nil {
		t.Fatal(err)
	}
	// The trigger goes on the partitioned parent, not on event_default.
	// ensure_event_partitions leaves the current month in the default
	// partition and pre-creates the next 18, so which partition an event
	// written now lands in depends on the month the database was migrated:
	// fresh today it is event_default, migrated any earlier month it is
	// event_YYYYMM. CI builds a database per run and always saw the first
	// case; testdb keeps package databases between runs, so a developer's
	// machine hits the second one the moment a month rolls over, and the
	// trigger silently never fires. Postgres 13 and up clone a row trigger
	// created here onto every partition, present and future.
	if _, err := pool.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION test_fail_event_insert() RETURNS trigger AS $$
		BEGIN
		  IF NEW.name = 'force_failure' THEN RAISE EXCEPTION 'forced event failure'; END IF;
		  RETURN NEW;
		END $$ LANGUAGE plpgsql;
		CREATE TRIGGER test_fail_event_insert_trigger BEFORE INSERT ON event
		FOR EACH ROW EXECUTE FUNCTION test_fail_event_insert()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_fail_event_insert_trigger ON event`)
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
