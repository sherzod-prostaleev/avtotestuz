package b2b_test

import (
	"context"
	"errors"
	"fmt"
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 30, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 10, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test"); err == nil {
		t.Fatal("expected ErrOrgSuspended for a suspended org")
	}
}

func TestOpenEnrollWindowRefusesWhenSeatsFull(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 3, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	// Seat the org's whole license with active stations.
	for i := 0; i < 3; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO b2b_station (org_id, public_key, hwid_hash, label)
			VALUES ($1, $2, $3, 'PC')`,
			orgID, []byte(fmt.Sprintf("key-full-%d", i)), fmt.Sprintf("hwid-full-%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	_, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted when active stations fill every seat", err)
	}
}

// TestOpenEnrollWindowOpensAtSeatsBoundary pins the free <= 0 check to
// exactly zero free seats, not "fewer than zero". With N seats and N-1
// active stations there is exactly one free seat: the window must open, and
// max_uses must be exactly 1. Paired with
// TestOpenEnrollWindowRefusesWhenSeatsFull (free == 0 must refuse), the two
// tests together nail the threshold: a mutation from "<= 0" to "< 0" would
// make the full-seats test above wrongly succeed; a mutation from "<= 0" to
// "<= 1" would make this test wrongly refuse.
func TestOpenEnrollWindowOpensAtSeatsBoundary(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 3, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	// N-1 active stations against N seats: exactly one seat free.
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO b2b_station (org_id, public_key, hwid_hash, label)
			VALUES ($1, $2, $3, 'PC')`,
			orgID, []byte(fmt.Sprintf("key-boundary-%d", i)), fmt.Sprintf("hwid-boundary-%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatalf("expected the window to open with exactly one free seat: %v", err)
	}
	if code.MaxUses != 1 {
		t.Fatalf("max_uses=%d, want 1 (exactly one free seat)", code.MaxUses)
	}
}

func TestOpenInstallerKeyIsIdempotent(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 30, now(), now() + interval '365 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	first, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	if first.MaxUses != 30 {
		t.Fatalf("max_uses=%d, want 30", first.MaxUses)
	}
	// Expiry tracks the licence, not a fixed TTL: a 365-day licence must not
	// yield a code that dies before a 30-PC rollout finishes.
	if d := time.Until(first.ExpiresAt); d < 300*24*time.Hour {
		t.Fatalf("expires in %v, want ~365 days", d)
	}

	second, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	if second.Code != first.Code || second.ID != first.ID {
		t.Fatalf("second call minted a new key (%s) instead of reusing %s", second.Code, first.Code)
	}
}

func TestRotateInstallerKeyRevokesTheOldOne(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 5, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	old, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := store.RotateInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Code == old.Code {
		t.Fatal("rotate returned the same code")
	}

	// The old key must no longer enrol.
	_, err = store.EnrollStation(ctx, b2b.EnrollInput{
		Code: old.Code, PublicKey: newPub(t), HWIDHash: testHWID("rotate-old"), Label: "PC",
	})
	if !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("old code err=%v, want ErrNotFound", err)
	}
	// The new one must.
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: fresh.Code, PublicKey: newPub(t), HWIDHash: testHWID("rotate-new"), Label: "PC",
	}); err != nil {
		t.Fatalf("new code failed to enrol: %v", err)
	}
}

func TestActiveInstallerKeyNilWithoutOne(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	row, err := store.ActiveInstallerKey(ctx, orgID)
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if row != nil {
		t.Fatalf("row=%+v, want nil", row)
	}
}

func TestOpenInstallerKeyNeedsALiveLicense(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenInstallerKey(ctx, orgID, "admin:test"); !errors.Is(err, b2b.ErrNoLicense) {
		t.Fatalf("err=%v, want ErrNoLicense", err)
	}
}

// TestOpenInstallerKeyRefusesSuspendedOrg exercises installerKeyTx's own
// suspended-org check, which is a separate block from OpenEnrollWindow's
// (not a shared helper) and so isn't covered by
// TestOpenEnrollWindowRefusesSuspendedOrgAndDeadLicense.
func TestOpenInstallerKeyRefusesSuspendedOrg(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 10, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}

	_, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if !errors.Is(err, b2b.ErrOrgSuspended) {
		t.Fatalf("err=%v, want ErrOrgSuspended for a suspended org", err)
	}
}

// TestOpenInstallerKeyRefusesWhenSeatsFull exercises installerKeyTx's own
// seats-exhausted check, which is a separate block from OpenEnrollWindow's
// (not a shared helper) and so isn't covered by
// TestOpenEnrollWindowRefusesWhenSeatsFull.
func TestOpenInstallerKeyRefusesWhenSeatsFull(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 3, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	// Seat the org's whole license with active stations.
	for i := 0; i < 3; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO b2b_station (org_id, public_key, hwid_hash, label)
			VALUES ($1, $2, $3, 'PC')`,
			orgID, []byte(newPub(t)), testHWID(fmt.Sprintf("installer-full-%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	_, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted when active stations fill every seat", err)
	}
}

// TestRotateInstallerKeyAtFullOccupancyRevokesWithoutReplacement is the
// regression test for the emergency-stop bug: a fully-seated school (every
// licensed seat holds an active station) is exactly the steady state a
// leaked installer key matters most in, and rotate must still be able to
// kill it even though there is no free seat to mint a replacement into.
// Before this fix, installerKeyTx computed the seat check before minting and
// returned ErrSeatsExhausted without ever touching the live code -- so the
// documented emergency stop silently did nothing when it was needed most.
func TestRotateInstallerKeyAtFullOccupancyRevokesWithoutReplacement(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 3, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	// Open the live (leaked) key while seats are still free.
	leaked, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}

	// Now fill every seat -- the steady state where a leak matters most.
	for i := 0; i < 3; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO b2b_station (org_id, public_key, hwid_hash, label)
			VALUES ($1, $2, $3, 'PC')`,
			orgID, []byte(newPub(t)), testHWID(fmt.Sprintf("rotate-full-%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	_, err = store.RotateInstallerKey(ctx, orgID, "admin:test")
	if !errors.Is(err, b2b.ErrInstallerKeyRotatedNoSeats) {
		t.Fatalf("err=%v, want ErrInstallerKeyRotatedNoSeats at full occupancy", err)
	}

	// The emergency stop must have actually run: the leaked code no longer
	// enrols, even though rotate reported "no replacement" rather than a
	// fresh code.
	var revoked *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM b2b_org_enroll_code WHERE id = $1`, leaked.ID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked == nil {
		t.Fatal("rotate at full occupancy must still revoke the live key -- the emergency stop must always run")
	}

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: leaked.Code, PublicKey: newPub(t), HWIDHash: testHWID("rotate-full-post-check"), Label: "PC",
	}); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("leaked code err=%v, want ErrNotFound (it must be dead after rotate, replacement or not)", err)
	}
}

// TestRotateInstallerKeyWithFreeSeatsRevokesAndMints pairs with the
// full-occupancy test above: with a free seat available, rotate must revoke
// the old key AND mint a working replacement in the same call, not just one
// or the other.
func TestRotateInstallerKeyWithFreeSeatsRevokesAndMints(t *testing.T) {
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
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 10, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	old, err := store.OpenInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatal(err)
	}

	fresh, err := store.RotateInstallerKey(ctx, orgID, "admin:test")
	if err != nil {
		t.Fatalf("rotate with free seats must mint a replacement, got err=%v", err)
	}
	if fresh.Code == "" || fresh.Code == old.Code {
		t.Fatalf("rotate = %+v, want a fresh non-empty code (old was %q)", fresh, old.Code)
	}

	var revoked *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM b2b_org_enroll_code WHERE id = $1`, old.ID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked == nil {
		t.Fatal("rotate must revoke the old key even when it also mints a replacement")
	}

	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: fresh.Code, PublicKey: newPub(t), HWIDHash: testHWID("rotate-free-seats-new"), Label: "PC",
	}); err != nil {
		t.Fatalf("replacement code failed to enrol: %v", err)
	}
}
