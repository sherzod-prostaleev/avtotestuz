package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

// seatedOrg inserts an active org with `seats` classroom seats live for 30 days.
func seatedOrg(t *testing.T, pool *pgxpool.Pool, seats int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, $2, 0, now(), now() + interval '30 days', 'test')`, orgID, seats); err != nil {
		t.Fatal(err)
	}
	return orgID
}

func newPub(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestEnrollStationBindsAndCapsSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-1",
		Label: "PC-1", AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.OrgID != orgID || first.StationID == uuid.Nil || first.ProfileID == uuid.Nil {
		t.Fatalf("bad result: %+v", first)
	}

	// The shadow profile is a station, not a learner.
	var kind, phone string
	if err := pool.QueryRow(ctx,
		`SELECT kind, phone FROM profile WHERE id = $1`, first.ProfileID).Scan(&kind, &phone); err != nil {
		t.Fatal(err)
	}
	if kind != "station" || !strings.HasPrefix(phone, "st:") {
		t.Fatalf("shadow profile kind=%q phone=%q", kind, phone)
	}

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-2", Label: "PC-2",
	}); err != nil {
		t.Fatal(err)
	}

	// Third machine: seats are gone.
	_, err = store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-3", Label: "PC-3",
	})
	if !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted", err)
	}
}

func TestEnrollStationRejectsBadCodes(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: "AVTO-ZZZZ-ZZZZ", PublicKey: newPub(t), HWIDHash: "hw-x", Label: "PC",
	}); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("unknown code err=%v, want ErrNotFound", err)
	}

	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET expires_at = now() - interval '1 minute' WHERE id = $1`,
		code.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-y", Label: "PC",
	}); err == nil {
		t.Fatal("expired code must be refused")
	}

	code2, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org_enroll_code SET revoked_at = now() WHERE id = $1`, code2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code2.Code, PublicKey: newPub(t), HWIDHash: "hw-z", Label: "PC",
	}); err == nil {
		t.Fatal("revoked code must be refused")
	}
}

func TestEnrollStationRebindsSameMachine(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-same", Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Re-imaged machine, same hardware, fresh key: the old bind is revoked and
	// the seat is reused rather than double-spent.
	second, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: "hw-same", Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.StationID == first.StationID {
		t.Fatal("re-enroll must create a new station row")
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != 1 {
		t.Fatalf("used=%d err=%v, want 1 active station", used, err)
	}
}

func TestEnrollStationConcurrentStampedeRespectsSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	const seats = 5
	const machines = 20
	orgID := seatedOrg(t, pool, seats)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make([]error, machines)
	for i := 0; i < machines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = store.EnrollStation(ctx, b2b.EnrollInput{
				Code:      code.Code,
				PublicKey: newPub(t),
				HWIDHash:  "hw-" + uuid.NewString(),
				Label:     "PC",
			})
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != seats {
		t.Fatalf("%d enrollments succeeded, want exactly %d", ok, seats)
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != seats {
		t.Fatalf("used=%d err=%v, want %d", used, err, seats)
	}
}
