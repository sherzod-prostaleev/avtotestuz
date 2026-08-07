package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// enrollTestStation inserts a station with its own shadow profile the same way
// a real enrolment does -- kind = 'station' and an "st:" phone -- so the delete
// path is exercised against the shape production actually stores.
func enrollTestStation(t *testing.T, h *Handler, orgID uuid.UUID, label string) (stationID, profileID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if err := h.Pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, kind)
		VALUES ('st:' || gen_random_uuid(), $1, 'station')
		RETURNING id`, label).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	if err := h.Pool.QueryRow(ctx, `
		INSERT INTO b2b_station (org_id, label, status, activated_by, public_key, hwid_hash, station_profile_id)
		VALUES ($1, $2, 'active', 'test', $3, $4, $5)
		RETURNING id`,
		orgID, label, []byte("delete-test-pubkey-"+label), "delete-test-hwid-"+label, profileID,
	).Scan(&stationID); err != nil {
		t.Fatal(err)
	}
	return stationID, profileID
}

// Deleting a station must take the shadow profile with it. Revoke leaves both
// in place on purpose; delete is for a PC that is gone, and a profile no admin
// screen ever lists (the user list filters kind = 'user') would otherwise sit
// there forever holding the practice history of a machine that no longer
// exists.
func TestDeleteStationRemovesTheRowAndItsShadowProfile(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "station-delete@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Delete School", 5, 30)

	stationID, profileID := enrollTestStation(t, h, orgID, "PC-DELETE")

	req := httptest.NewRequest(http.MethodDelete,
		"/admin/v1/b2b/orgs/"+orgID.String()+"/stations/"+stationID.String()+"/purge", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete station status=%d body=%s", w.Code, w.Body.String())
	}

	var n int
	if err := h.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM b2b_station WHERE id = $1`, stationID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("station row still present after delete")
	}
	if err := h.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM profile WHERE id = $1`, profileID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("shadow profile survived the station delete")
	}

	// Idempotence matters here: a double-click in the panel must not 500.
	req = httptest.NewRequest(http.MethodDelete,
		"/admin/v1/b2b/orgs/"+orgID.String()+"/stations/"+stationID.String()+"/purge", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("second delete status=%d, want 404", w.Code)
	}
}

// The same cross-org isolation RevokeStation has: without the org_id scope any
// school's admin call could erase another school's PC by id.
func TestDeleteStationRefusesAnotherOrgsStation(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "station-delete-cross@example.uz")
	ownerOrg := createInstallerTestOrg(t, r, access, "Owner School", 5, 30)
	otherOrg := createInstallerTestOrg(t, r, access, "Other School", 5, 30)

	stationID, profileID := enrollTestStation(t, h, ownerOrg, "PC-OWNED")

	req := httptest.NewRequest(http.MethodDelete,
		"/admin/v1/b2b/orgs/"+otherOrg.String()+"/stations/"+stationID.String()+"/purge", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-org delete status=%d, want 404", w.Code)
	}

	var n int
	if err := h.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM b2b_station WHERE id = $1`, stationID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("the owning school's station was deleted by another org's call")
	}
	if err := h.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM profile WHERE id = $1`, profileID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("the owning school's shadow profile was deleted by another org's call")
	}
}
