package arena

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

func TestMintTicketRespectsArenaFlag(t *testing.T) {
	pool := testdb.New(t)
	r := redisx.NewTest(t)
	svc := &Service{
		Pool: pool,
		R:    r,
		Lim:  auth.Limiter{R: r},
		Hub:  NewHub(),
		Now:  time.Now,
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		UPDATE feature_flag SET value_json = 'false'::jsonb WHERE key = 'arena_enabled'`); err != nil {
		t.Fatal(err)
	}
	_, _, err := svc.MintTicket(ctx, uuid.New())
	if err != ErrFeatureDisabled {
		t.Fatalf("want ErrFeatureDisabled, got %v", err)
	}
	_, _ = pool.Exec(ctx, `UPDATE feature_flag SET value_json = 'true'::jsonb WHERE key = 'arena_enabled'`)
	tok, _, err := svc.MintTicket(ctx, uuid.New())
	if err != nil || tok == "" {
		t.Fatalf("re-enabled mint: %v %q", err, tok)
	}
}

func TestMintRedeemTicketSingleUse(t *testing.T) {
	r := redisx.NewTest(t)
	svc := &Service{
		R:   r,
		Lim: auth.Limiter{R: r},
		Hub: NewHub(),
		Now: time.Now,
	}
	pid := uuid.New()
	tok, exp, err := svc.MintTicket(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if exp != 30 || tok == "" {
		t.Fatalf("tok=%q exp=%d", tok, exp)
	}
	got, err := svc.RedeemTicket(context.Background(), tok)
	if err != nil || got != pid {
		t.Fatalf("redeem: %v %v", got, err)
	}
	_, err = svc.RedeemTicket(context.Background(), tok)
	if err != ErrTicketInvalid {
		t.Fatalf("replay want ErrTicketInvalid, got %v", err)
	}
	_, err = svc.RedeemTicket(context.Background(), "nope")
	if err != ErrTicketInvalid {
		t.Fatalf("bad token: %v", err)
	}
}

func TestJoinLuaAtomicNoDoublePair(t *testing.T) {
	r := redisx.NewTest(t)
	const n = 20
	ctx := context.Background()
	ownKey := "arena:q:10"
	var paired atomic.Int64
	var wg sync.WaitGroup
	seen := sync.Map{}

	for i := 0; i < n; i++ {
		wg.Add(1)
		id := uuid.New()
		go func(id uuid.UUID) {
			defer wg.Done()
			res, err := r.Eval(ctx, arenaJoinLua, []string{ownKey}, id.String(), time.Now().UnixMilli(), ownKey, 120).Result()
			if err != nil {
				t.Errorf("eval: %v", err)
				return
			}
			arr := res.([]interface{})
			if arr[0].(string) == "paired" {
				paired.Add(1)
				opp := arr[1].(string)
				if _, loaded := seen.LoadOrStore(id.String(), true); loaded {
					t.Errorf("self paired twice: %s", id)
				}
				if _, loaded := seen.LoadOrStore(opp, true); loaded {
					t.Errorf("opponent paired twice: %s", opp)
				}
			}
		}(id)
	}
	wg.Wait()
	remaining, err := r.ZCard(ctx, ownKey).Result()
	if err != nil {
		t.Fatal(err)
	}
	matches := paired.Load()
	if matches != int64(n/2) || remaining != 0 {
		t.Fatalf("want %d matches and empty queue; got matches=%d remaining=%d", n/2, matches, remaining)
	}
}

func joinLua(t *testing.T, r *redis.Client, id uuid.UUID, bucket int) []interface{} {
	t.Helper()
	keys := queueSearchKeys(bucket)
	res, err := r.Eval(context.Background(), arenaJoinLua, keys, id.String(), time.Now().UnixMilli(), keys[0], 120).Result()
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	return res.([]interface{})
}

// Regression for the prod outage: ratings 969 and 1015 sit in buckets 9 and
// 10, and the old script only ever looked in the joiner's own bucket.
func TestJoinPairsAcrossRatingBuckets(t *testing.T) {
	r := redisx.NewTest(t)
	waiter, joiner := uuid.New(), uuid.New()
	if got := joinLua(t, r, waiter, Bucket(969)); got[0] != "queued" {
		t.Fatalf("first join: %v", got)
	}
	got := joinLua(t, r, joiner, Bucket(1015))
	if got[0] != "paired" || got[1] != waiter.String() {
		t.Fatalf("players one bucket apart were not paired: %v", got)
	}
	if n, _ := r.Exists(context.Background(), "arena:queued:"+waiter.String()).Result(); n != 0 {
		t.Fatal("paired waiter still marked queued")
	}
}

func TestJoinPrefersTheClosestBucket(t *testing.T) {
	r := redisx.NewTest(t)
	far, near, joiner := uuid.New(), uuid.New(), uuid.New()
	// 18 is out of reach of 9 (> MaxSearchSteps) so the two waiters cannot
	// pair with each other, but both are within reach of 10.
	joinLua(t, r, far, 18)
	time.Sleep(2 * time.Millisecond)
	joinLua(t, r, near, 9)
	got := joinLua(t, r, joiner, 10)
	if got[0] != "paired" || got[1] != near.String() {
		t.Fatalf("want the closer bucket's waiter, got %v", got)
	}
}

func TestJoinDropsGhostWaiters(t *testing.T) {
	r := redisx.NewTest(t)
	ctx := context.Background()
	ghost, joiner := uuid.New(), uuid.New()
	joinLua(t, r, ghost, 10)
	// The ghost's marker expired (timeout, crash) but its ZSET entry stayed.
	if err := r.Del(ctx, "arena:queued:"+ghost.String()).Err(); err != nil {
		t.Fatal(err)
	}
	got := joinLua(t, r, joiner, 10)
	if got[0] != "queued" {
		t.Fatalf("joiner paired with a ghost: %v", got)
	}
	members, _ := r.ZRange(ctx, "arena:q:10", 0, -1).Result()
	if len(members) != 1 || members[0] != joiner.String() {
		t.Fatalf("ghost not removed from the queue: %v", members)
	}
	marker, _ := r.Get(ctx, "arena:queued:"+joiner.String()).Result()
	if !strings.HasPrefix(marker, "10:") {
		t.Fatalf("marker %q must carry bucket and join time", marker)
	}
}

func TestReplacedSocketCloseIsNotADisconnect(t *testing.T) {
	svc := &Service{Hub: NewHub(), matches: map[uuid.UUID]*Match{}}
	pid := uuid.New()
	oldConn := &Conn{ProfileID: pid, out: make(chan []byte, 1)}
	newConn := &Conn{ProfileID: pid, out: make(chan []byte, 1)}
	// Register would close oldConn's (absent) websocket; set the state the
	// replacement leaves behind directly.
	svc.Hub.conns[pid] = newConn
	// Would dereference the nil Redis client if it treated this as a drop.
	svc.OnDisconnect(pid, oldConn)
}
