package billing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func createTestProfile(t *testing.T, q *sqlc.Queries) sqlc.Profile {
	t.Helper()
	p, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone:        "+998901234567",
		ReferralCode: pgtype.Text{String: "ABCD1234", Valid: true},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return p
}

func TestStatusFreshProfile(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile := createTestProfile(t, q)

	svc := Service{Q: q}
	active, until, err := svc.Status(context.Background(), profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active || until != nil {
		t.Fatalf("fresh profile should have no entitlement: active=%v until=%v", active, until)
	}
}

func TestGrantDaysStacking(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile := createTestProfile(t, q)
	svc := Service{Q: q}
	ctx := context.Background()

	until1, err := svc.GrantDays(ctx, profile.ID, 30, "admin", "test", uuid.NullUUID{})
	if err != nil {
		t.Fatalf("first grant: %v", err)
	}
	wantMin := time.Now().Add(29 * 24 * time.Hour)
	wantMax := time.Now().Add(31 * 24 * time.Hour)
	if until1.Before(wantMin) || until1.After(wantMax) {
		t.Fatalf("until1=%v not within expected ~30d window", until1)
	}

	active, until, err := svc.Status(ctx, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !active || until == nil {
		t.Fatal("expected active entitlement after grant")
	}

	until2, err := svc.GrantDays(ctx, profile.ID, 30, "admin", "test", uuid.NullUUID{})
	if err != nil {
		t.Fatalf("second grant: %v", err)
	}
	diff := until2.Sub(until1)
	if diff < 29*24*time.Hour || diff > 31*24*time.Hour {
		t.Fatalf("expected stacking (~30d apart): until1=%v until2=%v diff=%v", until1, until2, diff)
	}
}

func TestRevokeEntitlementForPayment(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := Service{Q: q}
	ctx := context.Background()
	profile := createTestProfile(t, q)

	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatalf("tariff: %v", err)
	}
	paymentID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900)`,
		paymentID, profile.ID, tariffID, "revoke-"+paymentID.String()); err != nil {
		t.Fatalf("payment: %v", err)
	}

	payNull := uuid.NullUUID{UUID: paymentID, Valid: true}
	if _, err := svc.GrantDaysForPayment(ctx, profile.ID, 30, "purchase", "paid", uuid.NullUUID{}, payNull); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if active, _, err := svc.Status(ctx, profile.ID); err != nil || !active {
		t.Fatalf("pre-revoke active=%v err=%v", active, err)
	}

	if err := svc.RevokeEntitlementForPayment(ctx, paymentID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if active, _, err := svc.Status(ctx, profile.ID); err != nil {
		t.Fatal(err)
	} else if active {
		t.Fatal("want inactive after revoke")
	}

	// Idempotent.
	if err := svc.RevokeEntitlementForPayment(ctx, paymentID); err != nil {
		t.Fatalf("second revoke: %v", err)
	}
	// Missing payment grant is a no-op.
	if err := svc.RevokeEntitlementForPayment(ctx, uuid.New()); err != nil {
		t.Fatalf("missing grant: %v", err)
	}
}

func TestRevokeEntitlementForPaymentKeepsLaterStack(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := Service{Q: q}
	ctx := context.Background()
	profile := createTestProfile(t, q)

	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatalf("tariff: %v", err)
	}
	mkPayment := func(key string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
			                     tariff_days_snapshot, tariff_price_uzs_snapshot)
			VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900)`,
			id, profile.ID, tariffID, key); err != nil {
			t.Fatalf("payment: %v", err)
		}
		return id
	}
	first := mkPayment("revoke-stack-1")
	second := mkPayment("revoke-stack-2")
	if _, err := svc.GrantDaysForPayment(ctx, profile.ID, 30, "purchase", "a", uuid.NullUUID{},
		uuid.NullUUID{UUID: first, Valid: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantDaysForPayment(ctx, profile.ID, 30, "purchase", "b", uuid.NullUUID{},
		uuid.NullUUID{UUID: second, Valid: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeEntitlementForPayment(ctx, first); err != nil {
		t.Fatal(err)
	}
	active, until, err := svc.Status(ctx, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !active || until == nil {
		t.Fatal("later stacked purchase must keep VIP active")
	}
	// Second grant alone is ~30d from when first would have ended; after
	// clamping first, tip math may shrink — but until must still be in future.
	if !until.After(time.Now()) {
		t.Fatalf("until=%v want future", until)
	}
}
