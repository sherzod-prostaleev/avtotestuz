// Package events provides batch ingestion of analytics-style client events
// (e.g. view_question, session_finish) into the event table.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
)

// maxBatchSize is a sane per-request cap — clients batch periodically, not
// unboundedly.
const (
	maxBatchSize      = 100
	maxEventNameRunes = 64
	maxPropsBytes     = 4096
)

var eventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

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
	Q    *sqlc.Queries
	Pool *pgxpool.Pool
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool) *Service {
	return &Service{Q: q, Pool: pool}
}

// LogBatch inserts every event, all attributed to profileID. An empty
// batch or a batch with more than 100 events is ErrInvalidRequest (a
// sane per-request cap — clients batch periodically, not unboundedly).
func (s *Service) LogBatch(ctx context.Context, profileID uuid.UUID, idempotencyKey string, events []Event) error {
	batchID, err := uuid.Parse(strings.TrimSpace(idempotencyKey))
	if err != nil || len(events) == 0 || len(events) > maxBatchSize || s.Pool == nil {
		return ErrInvalidRequest
	}
	for _, event := range events {
		if !validEvent(event) {
			return ErrInvalidRequest
		}
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		INSERT INTO event_batch (profile_id, idempotency_key, event_count)
		VALUES ($1,$2,$3)
		ON CONFLICT (profile_id, idempotency_key) DO NOTHING`, profileID, batchID, len(events))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	// A single multi-row COPY instead of one INSERT per event: a full
	// 100-event batch used to cost 100 round-trips. event is RANGE
	// partitioned on ts (0005/0050); COPY ... FROM STDIN routes rows to the
	// right partition the same way INSERT does (supported since PG 11), and
	// still fires the destination partition's row-level triggers, so this
	// stays inside the transaction/rollback semantics below unchanged.
	rows := make([][]any, len(events))
	for i, e := range events {
		rows[i] = []any{
			uuid.NullUUID{UUID: profileID, Valid: true},
			e.Name,
			propsOrEmptyObject(e.Props),
			pgtype.Timestamptz{Time: tsOrNow(e.TS), Valid: true},
		}
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"event"},
		[]string{"profile_id", "name", "props", "ts"},
		pgx.CopyFromRows(rows),
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validEvent(event Event) bool {
	name := strings.TrimSpace(event.Name)
	if len([]rune(name)) == 0 || len([]rune(name)) > maxEventNameRunes || !eventNamePattern.MatchString(name) {
		return false
	}
	if len(event.Props) > maxPropsBytes {
		return false
	}
	if len(event.Props) > 0 {
		var object map[string]any
		if !json.Valid(event.Props) || json.Unmarshal(event.Props, &object) != nil || object == nil {
			return false
		}
	}
	if event.TS != nil {
		now := time.Now()
		if event.TS.Before(now.Add(-7*24*time.Hour)) || event.TS.After(now.Add(5*time.Minute)) {
			return false
		}
	}
	return true
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
