package arena

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

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
			res, err := r.Eval(ctx, arenaJoinLua, []string{ownKey}, id.String(), time.Now().UnixMilli(), ownKey).Result()
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
