package events

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

const (
	// BatchRetention is how long an idempotency key stays honoured.
	//
	// The key exists so that a POST /events the client retried -- lost response,
	// flaky mobile connection, a PWA replaying a queue it built while offline --
	// is not counted twice. Once the client has moved on it will never present
	// that key again, so anything past the longest plausible retry is dead
	// weight. A week is far beyond any client-side queue here and cheap to keep:
	// the row is four small columns.
	//
	// Erring long is the safe direction. Too short silently reopens the
	// double-count this table was added to prevent; too long only costs rows.
	BatchRetention = 7 * 24 * time.Hour

	// purgeChunk bounds one DELETE. The first sweep has to clear everything
	// accumulated since the table was created (85k rows in production at the
	// time this was written) and one statement for that would hold a lock and
	// write a single enormous WAL record.
	purgeChunk = 5000

	purgeInterval = 6 * time.Hour
)

// ErrInvalidRetention rejects a window that would delete keys still in use.
var ErrInvalidRetention = errors.New("events: retention must be positive")

// PurgeExpiredBatches deletes idempotency keys older than retention, in chunks
// of at most chunk rows, and returns how many it removed.
//
// The subselect is what `event_batch_created_idx` is for: the table's own
// primary key is (profile_id, idempotency_key), so without that index this scan
// would be the very full scan the sweep exists to prevent the table from
// needing. Deleting by ctid keeps the chunk boundary exact -- a LIMIT inside
// DELETE ... USING would not be.
func (s *Service) PurgeExpiredBatches(ctx context.Context, retention time.Duration, chunk int) (int64, error) {
	if retention <= 0 {
		return 0, ErrInvalidRetention
	}
	if chunk <= 0 {
		chunk = purgeChunk
	}
	if s.Pool == nil {
		return 0, errors.New("events: pool is not configured")
	}

	var total int64
	for {
		tag, err := s.Pool.Exec(ctx, `
			DELETE FROM event_batch
			WHERE ctid IN (
			  SELECT ctid FROM event_batch
			  WHERE created_at < now() - $1::interval
			  LIMIT $2
			)`, retention.String(), chunk)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n < int64(chunk) {
			return total, nil
		}
		// Yield between chunks so a first sweep of a very large backlog cannot
		// monopolise a pool connection, and so cancellation is honoured.
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
	}
}

// RunRetentionWorker prunes expired idempotency keys until ctx is cancelled.
//
// It sweeps once at start rather than waiting out the first interval: the
// backlog this was written for already exists, and a deploy is the moment
// someone is watching.
func RunRetentionWorker(ctx context.Context, svc *Service, log *zap.Logger) {
	if svc == nil || svc.Pool == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}
	sweep := func() {
		removed, err := svc.PurgeExpiredBatches(ctx, BatchRetention, purgeChunk)
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("event batch retention sweep", zap.Error(err))
		case removed > 0:
			log.Info("event batch retention sweep", zap.Int64("removed", removed))
		}
	}

	log.Info("event batch retention worker started",
		zap.Duration("retention", BatchRetention), zap.Duration("interval", purgeInterval))
	sweep()

	t := time.NewTicker(purgeInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}
