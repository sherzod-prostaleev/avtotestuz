package learning

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"avtotest.uz/backend/internal/db/sqlc"
)

func bucketRows(n int32) []sqlc.PassRateByReadinessBucketRow {
	return []sqlc.PassRateByReadinessBucketRow{{BucketLo: 80, N: n, Passed: n}}
}

// counter returns a loader and a pointer to how many times it ran.
func counter(rows []sqlc.PassRateByReadinessBucketRow, err error) (func(context.Context) ([]sqlc.PassRateByReadinessBucketRow, error), *int) {
	var calls int
	var mu sync.Mutex
	return func(context.Context) ([]sqlc.PassRateByReadinessBucketRow, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return rows, err
	}, &calls
}

func TestPassRateCacheLoadsOnceWithinItsTTL(t *testing.T) {
	c := NewPassRateCache(5 * time.Minute)
	load, calls := counter(bucketRows(7), nil)

	for i := 0; i < 3; i++ {
		got, err := c.get(context.Background(), load)
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
		if len(got) != 1 || got[0].N != 7 {
			t.Fatalf("get %d: got %+v", i, got)
		}
	}
	if *calls != 1 {
		t.Fatalf("loader ran %d times, want 1", *calls)
	}
}

func TestPassRateCacheReloadsOnceTheTTLHasPassed(t *testing.T) {
	c := NewPassRateCache(5 * time.Minute)
	now := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return now }
	load, calls := counter(bucketRows(7), nil)

	if _, err := c.get(context.Background(), load); err != nil {
		t.Fatalf("first get: %v", err)
	}
	now = now.Add(5*time.Minute - time.Second)
	if _, err := c.get(context.Background(), load); err != nil {
		t.Fatalf("get inside ttl: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("loader ran %d times before the ttl elapsed, want 1", *calls)
	}

	now = now.Add(2 * time.Second)
	if _, err := c.get(context.Background(), load); err != nil {
		t.Fatalf("get after ttl: %v", err)
	}
	if *calls != 2 {
		t.Fatalf("loader ran %d times after the ttl elapsed, want 2", *calls)
	}
}

// A failed load must not be remembered: caching it would turn one slow moment
// on the database into ttl-long blindness for every reader.
func TestPassRateCacheDoesNotRememberAFailedLoad(t *testing.T) {
	c := NewPassRateCache(5 * time.Minute)
	boom := errors.New("boom")
	failing, failCalls := counter(nil, boom)

	if _, err := c.get(context.Background(), failing); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if _, err := c.get(context.Background(), failing); !errors.Is(err, boom) {
		t.Fatalf("second get: want boom, got %v", err)
	}
	if *failCalls != 2 {
		t.Fatalf("loader ran %d times, want 2 (the error must not be cached)", *failCalls)
	}

	ok, okCalls := counter(bucketRows(3), nil)
	got, err := c.get(context.Background(), ok)
	if err != nil {
		t.Fatalf("recovery get: %v", err)
	}
	if len(got) != 1 || got[0].N != 3 {
		t.Fatalf("recovery get: got %+v", got)
	}
	if *okCalls != 1 {
		t.Fatalf("recovery loader ran %d times, want 1", *okCalls)
	}
}

// The histogram is global, so every in-flight reader wants the same rows. One
// load must serve all of them rather than each starting its own scan -- the
// stampede is the whole reason this cache exists.
func TestPassRateCacheCollapsesConcurrentReadersOntoOneLoad(t *testing.T) {
	c := NewPassRateCache(5 * time.Minute)
	var calls int
	var mu sync.Mutex
	load := func(context.Context) ([]sqlc.PassRateByReadinessBucketRow, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return bucketRows(9), nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.get(context.Background(), load)
			if err != nil || len(got) != 1 || got[0].N != 9 {
				t.Errorf("concurrent get: rows=%+v err=%v", got, err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("loader ran %d times for 16 concurrent readers, want 1", calls)
	}
}

// A nil cache is the opt-out: callers that never installed one keep the
// uncached behaviour exactly, which is what leaves every existing test and
// every service built without a cache reading live rows.
func TestNilPassRateCacheLoadsEveryTime(t *testing.T) {
	var c *PassRateCache
	load, calls := counter(bucketRows(1), nil)

	for i := 0; i < 3; i++ {
		if _, err := c.get(context.Background(), load); err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
	}
	if *calls != 3 {
		t.Fatalf("loader ran %d times through a nil cache, want 3", *calls)
	}
}
