// Package events provides batch ingestion of analytics-style client events
// (e.g. view_question, session_finish) into the event table.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

// maxBatchSize is a sane per-request cap — clients batch periodically, not
// unboundedly.
const maxBatchSize = 100

// ErrInvalidRequest indicates an empty or oversized event batch.
var ErrInvalidRequest = errors.New("invalid event batch")

// Event is a single client-reported analytics event.
type Event struct {
	Name  string
	Props json.RawMessage // may be nil/empty — defaults to '{}' at the DB layer
	TS    *time.Time      // nil → now()
}

// Service inserts batches of client events attributed to a profile.
type Service struct {
	Q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service {
	return &Service{Q: q}
}

// LogBatch inserts every event, all attributed to profileID. An empty
// batch or a batch with more than 100 events is ErrInvalidRequest (a
// sane per-request cap — clients batch periodically, not unboundedly).
func (s *Service) LogBatch(ctx context.Context, profileID uuid.UUID, events []Event) error {
	if len(events) == 0 || len(events) > maxBatchSize {
		return ErrInvalidRequest
	}

	for _, e := range events {
		if err := s.Q.InsertEvent(ctx, sqlc.InsertEventParams{
			ProfileID: uuid.NullUUID{UUID: profileID, Valid: true},
			Name:      e.Name,
			Props:     propsOrEmptyObject(e.Props),
			Ts:        pgtype.Timestamptz{Time: tsOrNow(e.TS), Valid: true},
		}); err != nil {
			return err
		}
	}

	return nil
}

// propsOrEmptyObject defaults a nil/empty props payload to '{}'.
func propsOrEmptyObject(props json.RawMessage) []byte {
	if len(props) == 0 {
		return []byte("{}")
	}
	return []byte(props)
}

// tsOrNow defaults a nil timestamp to now().
func tsOrNow(ts *time.Time) time.Time {
	if ts == nil {
		return time.Now()
	}
	return *ts
}
