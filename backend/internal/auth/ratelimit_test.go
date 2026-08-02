package auth

import (
	"context"
	"testing"
	"time"

	"avtotest.uz/backend/internal/redisx"
)

func TestLimiterAllow(t *testing.T) {
	c := redisx.NewTest(t)
	lim := Limiter{R: c}
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, err := lim.Allow(ctx, "k1", 3, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	ok, err := lim.Allow(ctx, "k1", 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("4th attempt should be denied")
	}

	ok, err = lim.Allow(ctx, "k2", 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("new key should be independent and allowed")
	}
}

// TestLimiterAllowArmsWindowAtomically pins that a counter is never created
// without an expiry. The window used to be armed by a second command whose
// error was discarded, so it was only *probably* set.
func TestLimiterAllowArmsWindowAtomically(t *testing.T) {
	c := redisx.NewTest(t)
	lim := Limiter{R: c}
	ctx := context.Background()

	if _, err := lim.Allow(ctx, "ttl:first", 3, time.Minute); err != nil {
		t.Fatal(err)
	}
	ttl, err := c.PTTL(ctx, "ttl:first").Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl = %v, want a live window in (0, 1m]", ttl)
	}
}

// TestLimiterAllowRecoversWhenWindowExpiryWasLost covers the failure the fix is
// about: if the EXPIRE never landed — dropped connection, cancelled context,
// process killed between INCR and EXPIRE, or simply a key written by the old
// code — the counter lived forever and that phone/IP was rate limited until an
// operator noticed and ran DEL by hand.
//
// PERSIST reproduces exactly the state such a failure leaves behind (a counter
// with no TTL), which is the only part of the failure that is observable
// afterwards. The window must re-arm on the next increment and the lockout
// must then actually lapse.
func TestLimiterAllowRecoversWhenWindowExpiryWasLost(t *testing.T) {
	c := redisx.NewTest(t)
	lim := Limiter{R: c}
	ctx := context.Background()
	const key = "ttl:lost"
	const window = 200 * time.Millisecond

	if _, err := lim.Allow(ctx, key, 2, window); err != nil {
		t.Fatal(err)
	}
	if err := c.Persist(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	if ttl := c.PTTL(ctx, key).Val(); ttl > 0 {
		t.Fatalf("precondition: key should have lost its TTL, got %v", ttl)
	}

	ok, err := lim.Allow(ctx, key, 2, window)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("2nd attempt within a limit of 2 should be allowed")
	}
	ttl, err := c.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 {
		t.Fatalf("ttl = %v: the counter never expires, so this key is locked out forever", ttl)
	}

	if ok, err := lim.Allow(ctx, key, 2, window); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("3rd attempt should be denied while the window is open")
	}

	time.Sleep(window + 150*time.Millisecond)
	if ok, err := lim.Allow(ctx, key, 2, window); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("caller is still locked out after the window elapsed")
	}
}

func TestLimiterCooldown(t *testing.T) {
	c := redisx.NewTest(t)
	lim := Limiter{R: c}
	ctx := context.Background()

	ok, err := lim.Cooldown(ctx, "cd1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("first cooldown should succeed")
	}
	ok, err = lim.Cooldown(ctx, "cd1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second cooldown on same key should fail")
	}
}
