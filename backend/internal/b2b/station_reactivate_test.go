package b2b_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
)

// revokedStation enrolls a PC and then revokes it, which is the state every
// station at Avtomotohavaskorlar BUXORO was left in on 2026-08-26.
func revokedStation(t *testing.T, store b2b.Store, code string, orgID uuid.UUID, hwid string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	res, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeStation(ctx, orgID, res.StationID); err != nil {
		t.Fatal(err)
	}
	return res.StationID
}

// TestReactivateStationPutsAPCBackOnTheLicence is the whole point: a station
// that was revoked -- by an admin, by seat reclamation, or by the enrollment
// bug this shipped alongside -- can be put back without anyone visiting the
// machine. The agent still holds its keypair and station id, so its next token
// renewal (at most two minutes away) simply succeeds.
func TestReactivateStationPutsAPCBackOnTheLicence(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	stationID := revokedStation(t, store, code.Code, orgID, testHWID("hw-1"))

	if err := store.ReactivateStation(ctx, orgID, stationID); err != nil {
		t.Fatalf("reactivate: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM b2b_station WHERE id = $1`, stationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("status=%q, want active", status)
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != 1 {
		t.Fatalf("used=%d err=%v, want the seat taken back", used, err)
	}
}

// TestReactivateStationCannotOverfillTheLicence keeps the seat count honest.
// Reactivation is the one way back onto a licence that does not go through
// EnrollStation, so without its own check it would be a hole straight through
// the seat cap -- revoke ten PCs on a five-seat licence, then reactivate all
// ten.
func TestReactivateStationCannotOverfillTheLicence(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	spare := revokedStation(t, store, code.Code, orgID, testHWID("hw-spare"))
	// Fill both seats.
	for i, hw := range []string{"hw-a", "hw-b"} {
		if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
			Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID(hw), Label: "PC",
		}); err != nil {
			t.Fatalf("filling seat %d: %v", i+1, err)
		}
	}

	if err := store.ReactivateStation(ctx, orgID, spare); !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted", err)
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != 2 {
		t.Fatalf("used=%d err=%v, want the licence untouched at 2", used, err)
	}
}

// TestReactivateStationIsScopedToItsOrg: a station id is not a secret -- it
// sits in agent config and in every admin station list -- so the org in the
// path has to be the org that owns the row, or one school's admin could put
// another school's PC back on a licence they do not pay for.
func TestReactivateStationIsScopedToItsOrg(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgA := seatedOrg(t, pool, 5)
	codeA, err := store.OpenEnrollWindow(ctx, orgA, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	stationA := revokedStation(t, store, codeA.Code, orgA, testHWID("hw-a"))
	orgB := seatedOrg(t, pool, 5)

	if err := store.ReactivateStation(ctx, orgB, stationA); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM b2b_station WHERE id = $1`, stationA).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "revoked" {
		t.Fatalf("status=%q, want it left revoked", status)
	}
}

// TestReactivateStationRefusesWhenThereIsNothingToReactivate covers both an
// unknown id and a station that is already active, which is what a second
// click on a stale list produces. It mirrors RevokeStation, which answers the
// same way when there is no active row to revoke.
func TestReactivateStationRefusesWhenThereIsNothingToReactivate(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	live, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: testHWID("hw-live"), Label: "PC",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.ReactivateStation(ctx, orgID, live.StationID); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("already active: err=%v, want ErrNotFound", err)
	}
	if err := store.ReactivateStation(ctx, orgID, uuid.New()); !errors.Is(err, b2b.ErrNotFound) {
		t.Fatalf("unknown id: err=%v, want ErrNotFound", err)
	}
}

// TestReactivateStationRefusesASuspendedOrg: a suspended school's stations get
// no VIP anyway (ActiveStationVIP joins on o.status = 'active'), so putting one
// back would spend a seat and change nothing an operator could see. Saying so
// is better than appearing to work.
func TestReactivateStationRefusesASuspendedOrg(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 5)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	stationID := revokedStation(t, store, code.Code, orgID, testHWID("hw-1"))
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_org SET status = 'suspended' WHERE id = $1`, orgID); err != nil {
		t.Fatal(err)
	}

	if err := store.ReactivateStation(ctx, orgID, stationID); !errors.Is(err, b2b.ErrOrgSuspended) {
		t.Fatalf("err=%v, want ErrOrgSuspended", err)
	}
}

// TestAJustReactivatedStationIsNotEvictedBeforeItCanReport closes the gap
// between the two ways a seat moves.
//
// reclaimStaleSeat takes a seat from a station that has been silent for half an
// hour, on the reasoning that a live classroom PC renews its token far more
// often than that. A station that was just put back by hand has been silent for
// far longer -- it was revoked, so it could not renew anything -- and its PC
// has not been switched on yet. Judged on last_seen_at alone it looks like the
// deadest row in the org, so the very next enrollment on a full licence would
// take the seat an admin had just deliberately given it.
func TestAJustReactivatedStationIsNotEvictedBeforeItCanReport(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 1)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	hwid := testHWID("one-master-image")
	stationID := revokedStation(t, store, code.Code, orgID, hwid)

	// Long silent, exactly as a PC revoked yesterday would be.
	if _, err := pool.Exec(ctx,
		`UPDATE b2b_station SET last_seen_at = now() - interval '18 hours' WHERE id = $1`,
		stationID); err != nil {
		t.Fatal(err)
	}
	if err := store.ReactivateStation(ctx, orgID, stationID); err != nil {
		t.Fatal(err)
	}

	// The one seat is now taken. A new PC from the same disk image must be
	// refused, not handed the seat that was just restored.
	if _, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: newPub(t), HWIDHash: hwid, Label: "PC-new",
	}); !errors.Is(err, b2b.ErrSeatsExhausted) {
		t.Fatalf("err=%v, want ErrSeatsExhausted", err)
	}
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM b2b_station WHERE id = $1`, stationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("status=%q, want the reactivated station left alone", status)
	}
}

// TestReactivateStationRestoresTheWholeClassroom is the BUXORO case end to
// end: 20 PCs imaged from one master disk, all revoked, all put back in one
// pass. They share an hwid, so this also pins that reactivation is not
// re-introducing the one-active-station-per-image rule that caused the
// incident.
func TestReactivateStationRestoresTheWholeClassroom(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := b2b.Store{Pool: pool}
	ctx := context.Background()

	const lab = 20
	orgID := seatedOrg(t, pool, 55)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	cloned := testHWID("one-master-image")
	ids := make([]uuid.UUID, 0, lab)
	for i := 0; i < lab; i++ {
		res, err := store.EnrollStation(ctx, b2b.EnrollInput{
			Code: code.Code, PublicKey: newPub(t), HWIDHash: cloned, Label: "HPStoreBukhara",
		})
		if err != nil {
			t.Fatalf("PC %d: %v", i+1, err)
		}
		if err := store.RevokeStation(ctx, orgID, res.StationID); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, res.StationID)
	}

	for i, id := range ids {
		if err := store.ReactivateStation(ctx, orgID, id); err != nil {
			t.Fatalf("PC %d of %d could not be reactivated: %v", i+1, lab, err)
		}
	}
	used, err := store.CountActiveStations(ctx, orgID)
	if err != nil || used != lab {
		t.Fatalf("used=%d err=%v, want all %d back", used, err, lab)
	}
}
