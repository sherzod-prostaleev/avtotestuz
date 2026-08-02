package auth

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct{ R *redis.Client }

// allowScript increments a fixed-window counter and arms its expiry in a
// single atomic step.
//
// It replaces an INCR followed by a separate `_ = Expire(...)` that ran only
// when the counter came back as 1 and threw the error away. Two independent
// commands meant the window could be lost: if the process died, the context
// was cancelled, or Redis dropped that one command between INCR and EXPIRE,
// the key stayed alive forever — and an immortal counter that has already
// reached the limit locks that phone or IP out permanently, with no recovery
// short of a manual DEL by an operator who first has to work out why one user
// can never log in again.
//
// The PTTL guard is what makes the window self-healing rather than merely
// atomic: PTTL is -1 for a key with no expiry and -2 for a missing one, so any
// counter that lost its TTL for any reason (a key written by the old code, an
// operator PERSIST, a failover that replicated the value but not the expiry)
// gets a fresh window on its next increment instead of staying immortal.
//
// Re-arming does mean a key that lost its TTL restarts its window on the next
// call rather than expiring at the original deadline. That is the intended
// trade: the alternative is a permanent lockout, and the counter it protects
// is a coarse abuse limit, not an accounting ledger.
var allowScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 or redis.call('PTTL', KEYS[1]) < 0 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return n
`)

// Allow implements a fixed-window counter: true while count <= limit.
//
// A Redis error denies the request (fail closed): these counters guard OTP
// sends, login attempts and other spend-money/spend-trust actions, so an
// unavailable limiter must not turn into an unlimited one.
func (l Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	ms := window.Milliseconds()
	if ms < 1 {
		// PEXPIRE rejects a non-positive TTL; a sub-millisecond window is
		// nonsense anyway, so clamp instead of failing the caller closed.
		ms = 1
	}
	n, err := allowScript.Run(ctx, l.R, []string{key}, ms).Int64()
	if err != nil {
		return false, err
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
