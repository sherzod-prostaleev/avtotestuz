package redisx

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// NewTest returns a client against TEST_REDIS_URL (default DB 1), flushed clean.
func NewTest(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379/1"
	}
	c, err := New(url)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flushdb: %v (run `make up`)", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
