package b2b_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/devicefp"
	"avtotest.uz/backend/internal/testdb"
)

func TestStationVIPBindAndGate(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO b2b_org (name) VALUES ('Demo School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, home_seats, starts_at, ends_at, note)
		VALUES ($1, 1, 0, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	code, err := store.CreateActivateCode(ctx, orgID, "PC-1", "test", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	fp := "station-fp-" + uuid.NewString()
	st, err := store.ActivateStation(ctx, code.Code, fp, "Lab 1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "active" {
		t.Fatalf("status=%s", st.Status)
	}

	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != 1 {
		t.Fatalf("used=%d err=%v", used, err)
	}

	// Seat exhausted
	code2, err := store.CreateActivateCode(ctx, orgID, "PC-2", "test", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ActivateStation(ctx, code2.Code, "other-fp", "Lab 2", "test"); err == nil {
		t.Fatal("expected seats exhausted")
	}

	q := sqlc.New(pool)
	bill := billing.Service{Q: q, StationVIP: store}
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3)`,
		profileID, "+998901112233", "Student"); err != nil {
		t.Fatal(err)
	}

	active, until, err := bill.Status(devicefp.WithContext(ctx, fp), profileID)
	if err != nil || !active || until == nil {
		t.Fatalf("expected station VIP active=%v until=%v err=%v", active, until, err)
	}

	active, _, err = bill.Status(devicefp.WithContext(ctx, "home-device"), profileID)
	if err != nil || active {
		t.Fatalf("home device should not get station VIP, active=%v err=%v", active, err)
	}

	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}
	active, _, err = bill.Status(devicefp.WithContext(ctx, fp), profileID)
	if err != nil || active {
		t.Fatalf("suspended org must revoke station VIP, active=%v err=%v", active, err)
	}
}
