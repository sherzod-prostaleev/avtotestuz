package auth

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct{ R *redis.Client }

// Allow implements a fixed-window counter: true while count <= limit.
func (l Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	n, err := l.R.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		_ = l.R.Expire(ctx, key, window).Err()
	}
	return n <= int64(limit), nil
}

// Cooldown returns true if the key was free (and sets it for d).
func (l Limiter) Cooldown(ctx context.Context, key string, d time.Duration) (bool, error) {
	ok, err := l.R.SetNX(ctx, key, "1", d).Result()
	return ok, err
}

// Count reads a fixed-window counter without incrementing it. Used to check
// a lockout before spending work (e.g. a bcrypt comparison) on an attempt
// that is going to be refused anyway. A missing key counts as zero.
func (l Limiter) Count(ctx context.Context, key string) (int64, error) {
	n, err := l.R.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return n, nil
}

// Reset clears a counter, e.g. after a successful login retires a run of
// failed attempts.
func (l Limiter) Reset(ctx context.Context, key string) error {
	return l.R.Del(ctx, key).Err()
}
