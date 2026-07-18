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
	svc := events.NewService(q)

	err = svc.LogBatch(context.Background(), profile.ID, []events.Event{
		{Name: "view_question", Props: json.RawMessage(`{"question_id":"x"}`)},
		{Name: "session_finish"},
	})
	if err != nil {
		t.Fatalf("LogBatch: %v", err)
	}
}

func TestLogBatchRejectsEmpty(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q)
	if err := svc.LogBatch(context.Background(), uuid.New(), nil); err != events.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestLogBatchRejectsOversized(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q)
	big := make([]events.Event, 101)
	for i := range big {
		big[i] = events.Event{Name: "x"}
	}
	if err := svc.LogBatch(context.Background(), uuid.New(), big); err != events.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}
