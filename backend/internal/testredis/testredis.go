// Package testredis hands tests an isolated Redis database.
//
// It uses REDIS_TEST_URL (or the compose default) and always binds to
// logical DB 15, flushing it on setup and cleanup so nonces from one test
// never satisfy another's replay check. That DB is shared by every caller,
// so only one package may use this helper at a time — today that is
// internal/b2b. A second package using it concurrently under `go test ./...`
// parallelism would FlushDB the first package's live state mid-run.
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
		t.Fatalf("redis unavailable at %s: %v", url, err)
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
