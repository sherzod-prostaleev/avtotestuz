package redisx

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/testenv"
)

// testDBByPackage assigns each test package its own Redis logical database.
//
// NewTest flushes the database it hands out, so two packages sharing one index
// wipe each other's keys whenever they run at the same time — that is how a
// parallel `go test ./...` used to make internal/auth's OTP cooldown test fail
// while the cooldown logic itself was fine.
//
// The mapping is explicit rather than hashed because Redis exposes a fixed,
// small number of logical databases (16 by default), so a hash cannot promise
// distinct slots. An unlisted package fails loudly in NewTest instead of
// silently sharing, which is the whole point.
var testDBByPackage = map[string]int{
	"internal_auth":        1,
	"internal_demo":        2,
	"internal_leaderboard": 3,
	"internal_session":     4,
	"internal_server":      5,
	// Spec asked for slot 5; that is already taken by internal_server.
	"internal_arena": 6,
}

// baseRedisURL is the connection string whose database index gets replaced with
// the calling package's slot.
func baseRedisURL() string {
	if v := os.Getenv("TEST_REDIS_URL"); v != "" {
		return v
	}
	return "redis://localhost:6379/0"
}

// NewTest returns a client against this test package's own Redis database,
// flushed clean.
func NewTest(t *testing.T) *redis.Client {
	t.Helper()

	slug := testenv.PackageSlug()
	dbIndex, ok := testDBByPackage[slug]
	if !ok {
		t.Fatalf(
			"redisx.NewTest: package %q has no Redis test database assigned; add it to testDBByPackage in internal/redisx/testhelper.go (used: %s)",
			slug, usedSlots(),
		)
	}

	url, err := urlWithDB(baseRedisURL(), dbIndex)
	if err != nil {
		t.Fatalf("redis test url: %v", err)
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

// urlWithDB swaps the database index in a redis:// URL, whose path is the
// index (redis://host:6379/1).
func urlWithDB(base string, dbIndex int) (string, error) {
	slash := strings.LastIndex(base, "/")
	// Only a trailing "/<digits>" is a database index; "redis://host:6379" has
	// no index at all and its last slash belongs to the scheme.
	if slash > strings.Index(base, "://")+2 {
		if _, err := strconv.Atoi(base[slash+1:]); err == nil {
			return base[:slash+1] + strconv.Itoa(dbIndex), nil
		}
	}
	if base == "" {
		return "", fmt.Errorf("empty redis url")
	}
	return strings.TrimSuffix(base, "/") + "/" + strconv.Itoa(dbIndex), nil
}

func usedSlots() string {
	entries := make([]string, 0, len(testDBByPackage))
	for pkg, idx := range testDBByPackage {
		entries = append(entries, fmt.Sprintf("%s=%d", pkg, idx))
	}
	sort.Strings(entries)
	return strings.Join(entries, ", ")
}
