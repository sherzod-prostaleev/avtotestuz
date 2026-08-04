package b2b_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

func TestOpenEnrollWindowSizesToFreeSeats(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, 30, 0, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	code, err := store.OpenEnrollWindow(ctx, orgID, 2*time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if code.MaxUses != 30 {
		t.Fatalf("max_uses=%d, want 30 (all seats free)", code.MaxUses)
	}
	if !strings.HasPrefix(code.Code, "AVTO-") {
		t.Fatalf("code=%q, want AVTO- prefix", code.Code)
	}
	if code.ExpiresAt.Sub(time.Now().UTC()) > 2*time.Hour+time.Minute {
		t.Fatalf("expires_at too far out: %v", code.ExpiresAt)
	}

	// Opening a second window closes the first: a school must never have two
	// live codes in circulation.
	code2, err := store.OpenEnrollWindow(ctx, orgID, 2*time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	var revoked *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM b2b_org_enroll_code WHERE id = $1`, code.ID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked == nil {
		t.Fatal("opening a new window must revoke the previous one")
	}
	if code2.Code == code.Code {
		t.Fatal("second window must mint a different code")
	}
}

func TestOpenEnrollWindowRefusesSuspendedOrgAndDeadLicense(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}

	// No license yet.
	if _, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test"); err == nil {
		t.Fatal("expected ErrNoLicense without a live license")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, 10, 0, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test"); err == nil {
		t.Fatal("expected ErrOrgSuspended for a suspended org")
	}
}
