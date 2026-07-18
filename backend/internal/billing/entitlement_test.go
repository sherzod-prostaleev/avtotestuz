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
