// Package testredis hands tests an isolated Redis database.
//
// It uses REDIS_TEST_URL (or the compose default) and picks a per-test
// logical DB, flushing it on cleanup so nonces from one test never satisfy
// another's replay check.
package testredis

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// New returns a flushed client bound to logical DB 15.
func New(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		url = "redis://localhost:6379/15"
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_TEST_URL: %v", err)
	}
	c := redis.NewClient(opt)
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", url, err)
	}
	if err := c.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
	t.Cleanup(func() {
		_ = c.FlushDB(context.Background()).Err()
		_ = c.Close()
	})
	return c
}
