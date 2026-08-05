package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/stationctx"
	"avtotest.uz/backend/internal/testdb"
)

func TestActiveStationVIPGrantsAndRevokes(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('Demo School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 5, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var profileID, stationID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), 'PC-1', 'station') RETURNING id`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO b2b_station (org_id, public_key, hwid_hash, label, station_profile_id)
		VALUES ($1, $2, 'hw-1', 'PC-1', $3) RETURNING id`,
		orgID, []byte(pub), profileID).Scan(&stationID); err != nil {
		t.Fatal(err)
	}

	bill := billing.Service{Q: sqlc.New(pool), StationVIP: store}

	active, until, err := bill.Status(stationctx.WithContext(ctx, stationID), profileID)
	if err != nil || !active || until == nil {
		t.Fatalf("bound station must have VIP: active=%v until=%v err=%v", active, until, err)
	}

	// A context with no station id gets nothing.
	if active, _, err := bill.Status(ctx, profileID); err != nil || active {
		t.Fatalf("bare context must not grant VIP: active=%v err=%v", active, err)
	}

	// An unknown station id gets nothing.
	if active, _, err := bill.Status(stationctx.WithContext(ctx, uuid.New()), profileID); err != nil || active {
		t.Fatalf("unknown station must not grant VIP: active=%v err=%v", active, err)
	}

	if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
		t.Fatal(err)
	}
	if active, _, err := bill.Status(stationctx.WithContext(ctx, stationID), profileID); err != nil || active {
		t.Fatalf("suspended org must revoke VIP: active=%v err=%v", active, err)
	}

	if err := store.SetOrgStatus(ctx, orgID, "active"); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeStation(ctx, orgID, stationID); err != nil {
		t.Fatal(err)
	}
	if active, _, err := bill.Status(stationctx.WithContext(ctx, stationID), profileID); err != nil || active {
		t.Fatalf("revoked station must lose VIP: active=%v err=%v", active, err)
	}
}
