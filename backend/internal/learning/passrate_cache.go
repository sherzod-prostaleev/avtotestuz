package learning

import (
	"context"
	"sync"
	"time"

	"avtotest.uz/backend/internal/db/sqlc"
)

// DefaultPassRateTTL is how long the pass-rate histogram is served from
// memory. The number it feeds is an estimate shown next to a readiness
// percentage, so minutes of staleness are invisible to a reader, while the
// query behind it is a full scan of exam_session.
const DefaultPassRateTTL = 5 * time.Minute

// PassRateCache memoises PassRateByReadinessBucket, which Stats calls on every
// request and which every reader gets the same answer from.
//
// The query takes no arguments: it is one histogram over every finished
// exam-like session in the database, identical for every profile. Uncached, it
// was a sequential scan of the whole exam_session table on each /me/stats --
// a cost that grows with every session ever started, including the ones left
// open and never resumed, to recompute a number that had not changed.
//
// A nil *PassRateCache is a working cache that never caches. That is what
// keeps a Service built without one -- every test, and any caller that has not
// opted in -- reading live rows exactly as before.
type PassRateCache struct {
	mu   sync.Mutex
	ttl  time.Duration
	now  func() time.Time
	rows []sqlc.PassRateByReadinessBucketRow
	at   time.Time
	held bool
}

// NewPassRateCache returns a cache that serves stored rows for ttl. A ttl of
// zero or less disables storing, which makes it equivalent to no cache.
func NewPassRateCache(ttl time.Duration) *PassRateCache {
	return &PassRateCache{ttl: ttl, now: time.Now}
}

// get returns the histogram, calling load at most once per ttl.
//
// load runs while the lock is held, so concurrent readers arriving during a
// slow scan wait for that one scan instead of each starting another. That
// single-flight behaviour matters most exactly when the database is slow,
// which is when a stampede would otherwise do the most damage.
//
// A failed load is not stored: the error goes to this caller and the next one
// tries again.
func (c *PassRateCache) get(
	ctx context.Context,
	load func(context.Context) ([]sqlc.PassRateByReadinessBucketRow, error),
) ([]sqlc.PassRateByReadinessBucketRow, error) {
	if c == nil {
		return load(ctx)
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.held && c.ttl > 0 && c.now().Sub(c.at) < c.ttl {
		return c.rows, nil
	}
	rows, err := load(ctx)
	if err != nil {
		return nil, err
	}
	if c.ttl > 0 {
		c.rows, c.at, c.held = rows, c.now(), true
	}
	return rows, nil
}
