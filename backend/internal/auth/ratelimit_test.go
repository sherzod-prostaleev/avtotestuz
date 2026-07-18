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
