package auth

import (
	"context"
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
