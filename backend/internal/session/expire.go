package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
)

const (
	// ExpiryGrace is how far past its own deadline a session must be before
	// the sweep will close it. The deadline alone is already enough -- an
	// exam past it cannot take another answer -- so this is only here so that
	// a learner submitting their last answer at the buzzer is never racing a
	// background job for their own session.
	ExpiryGrace = 15 * time.Minute

	// expirySweepInterval and expirySweepLimit together decide how fast a
	// backlog drains. Bounded on purpose: the sessions have been sitting for
	// weeks and nothing depends on clearing them quickly, whereas finishing
	// thousands of them in one burst would put a spike of transactions and
	// readiness snapshots on a database that is also serving lessons.
	expirySweepInterval = time.Hour
	expirySweepLimit    = 200
)

// listExpiredTimedSessions is deliberately written out rather than added to
// session.sql: the cutoff has to come from Service.now() so that tests can
// move the clock, which SQL's own now() would not let them do.
const listExpiredTimedSessions = `
SELECT id FROM exam_session
WHERE status = 'in_progress'
  AND time_limit_sec IS NOT NULL
  AND started_at + make_interval(secs => time_limit_sec) < $1
ORDER BY started_at
LIMIT $2`

// ExpireTimedOutSessions finishes in-progress sessions whose time limit ran
// out more than grace ago, and returns how many it finished.
//
// It closes exactly the sessions that can no longer be played and nothing
// else. The predicate that guarantees that is `time_limit_sec IS NOT NULL`:
// only exam, grand_mock and placement are given a limit when they are
// created, so variant, practice, review and mistakes sessions are outside
// this sweep by the shape of the data rather than by a list of modes someone
// has to remember to keep in step. Those modes are left open on purpose --
// resuming them where the class stopped is the point -- and the test named
// TestExpireNeverTouchesASessionWithoutATimeLimit exists to keep it that way.
//
// Each session is finished in its own short transaction through the same
// finishInternal that a real time-out goes through, so a swept session is
// graded and recorded exactly as it would have been had the learner submitted
// one more answer or pressed finish. The row is locked first, and
// finishInternal is a no-op on a session that is no longer in progress, so a
// learner finishing at the same moment wins and the sweep skips it.
//
// One session's failure does not stop the sweep: the error is returned to the
// caller for logging after every other candidate has had its turn.
func (s *Service) ExpireTimedOutSessions(ctx context.Context, grace time.Duration, limit int) (int, error) {
	if s.Pool == nil {
		return 0, errors.New("session transaction pool is not configured")
	}
	if limit <= 0 {
		limit = expirySweepLimit
	}
	if grace < 0 {
		grace = 0
	}

	cutoff := s.now().Add(-grace)
	rows, err := s.Pool.Query(ctx, listExpiredTimedSessions, cutoff, limit)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var finished int
	var firstErr error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return finished, err
		}
		done, err := s.expireOne(ctx, id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if done {
			finished++
		}
	}
	return finished, firstErr
}

// expireOne finishes a single expired session, reporting whether it actually
// wrote one. It re-verifies every condition under the row lock rather than
// trusting the candidate query: the row may have been finished, or the sweep
// may have selected it wrongly, and a session that is still playable must
// survive either way.
func (s *Service) expireOne(ctx context.Context, id uuid.UUID) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlc.New(tx)
	row, err := q.GetExamSessionForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // deleted with its profile while we queued
		}
		return false, err
	}
	if row.Status != "in_progress" || !row.TimeLimitSec.Valid {
		return false, nil
	}
	deadline := row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32) * time.Second)
	if s.now().Before(deadline) {
		return false, nil
	}

	if _, err := s.transactional(q).finishInternal(ctx, row, false, true); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// RunExpiryWorker closes timed-out sessions until ctx is cancelled.
//
// It sweeps once at start because the backlog this was written for already
// exists and a deploy is the moment someone is watching, then hourly. Each
// sweep is capped, so the first one after a long gap is no heavier than any
// other and the remainder is picked up an hour later.
func RunExpiryWorker(ctx context.Context, svc *Service, log *zap.Logger) {
	if svc == nil || svc.Pool == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}
	sweep := func() {
		finished, err := svc.ExpireTimedOutSessions(ctx, ExpiryGrace, expirySweepLimit)
		if err != nil && ctx.Err() == nil {
			log.Error("session expiry sweep", zap.Error(err), zap.Int("finished", finished))
			return
		}
		if finished > 0 {
			log.Info("session expiry sweep", zap.Int("finished", finished))
		}
	}

	log.Info("session expiry worker started",
		zap.Duration("grace", ExpiryGrace), zap.Duration("interval", expirySweepInterval))
	sweep()

	t := time.NewTicker(expirySweepInterval)
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
