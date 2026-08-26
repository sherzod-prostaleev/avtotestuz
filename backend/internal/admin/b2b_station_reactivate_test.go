package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// revokeTestStation puts an enrolled station into the state 37 classroom PCs
// were left in on 2026-08-26: row intact, keypair intact, shadow profile
// intact, and only status standing between the school and a working kiosk.
func revokeTestStation(t *testing.T, h *Handler, stationID uuid.UUID) {
	t.Helper()
	if _, err := h.Pool.Exec(context.Background(),
		`UPDATE b2b_station SET status = 'revoked' WHERE id = $1`, stationID); err != nil {
		t.Fatal(err)
	}
}

func stationStatus(t *testing.T, h *Handler, stationID uuid.UUID) string {
	t.Helper()
	var status string
	if err := h.Pool.QueryRow(context.Background(),
		`SELECT status FROM b2b_station WHERE id = $1`, stationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// TestReactivateStationBringsAPCBackAndIsAudited is the route the BUXORO
// repair had to be done without: an admin puts a revoked PC back on the licence
// from the panel, and the agent on that machine recovers by itself at its next
// token renewal. Doing it by hand against production, as that incident
// required, leaves no audit trail and no record of who decided it.
func TestReactivateStationBringsAPCBackAndIsAudited(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "station-reactivate@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Reactivate School", 5, 30)
	stationID, _ := enrollTestStation(t, h, orgID, "PC-BACK")
	revokeTestStation(t, h, stationID)

	req := httptest.NewRequest(http.MethodPost,
		"/admin/v1/b2b/orgs/"+orgID.String()+"/stations/"+stationID.String()+"/reactivate", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reactivate status=%d body=%s", w.Code, w.Body.String())
	}

	var env struct {
		Data struct {
			Reactivated bool `json:"reactivated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.Reactivated {
		t.Fatalf("body=%s, want reactivated true", w.Body.String())
	}
	if got := stationStatus(t, h, stationID); got != "active" {
		t.Fatalf("status=%q, want active", got)
	}

	var audits int
	if err := h.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log WHERE action = 'b2b.stations.reactivate' AND entity_id = $1`,
		stationID.String()).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("audit rows=%d, want 1: putting a PC back on a licence is a decision someone made", audits)
	}
}

// TestReactivateStationReportsAFullLicence: the seat cap has to survive this
// route, and the panel has to be able to say why the click did nothing.
func TestReactivateStationReportsAFullLicence(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "station-reactivate-full@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Full School", 1, 30)

	spare, _ := enrollTestStation(t, h, orgID, "PC-SPARE")
	revokeTestStation(t, h, spare)
	enrollTestStation(t, h, orgID, "PC-LIVE") // takes the only seat

	req := httptest.NewRequest(http.MethodPost,
		"/admin/v1/b2b/orgs/"+orgID.String()+"/stations/"+spare.String()+"/reactivate", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, "seats_exhausted") {
		t.Fatalf("body=%s, want a seats_exhausted code the panel can act on", got)
	}
	if got := stationStatus(t, h, spare); got != "revoked" {
		t.Fatalf("status=%q, want it left revoked", got)
	}
}

// TestReactivateStationRefusesAnotherOrgsPC keeps the org in the path
// authoritative: a station id appears in agent config and in every station
// list, so it must not be enough on its own to spend another school's seat.
func TestReactivateStationRefusesAnotherOrgsPC(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "station-reactivate-scope@example.uz")
	orgID := createInstallerTestOrg(t, r, access, "Owner School", 5, 30)
	otherOrg := createInstallerTestOrg(t, r, access, "Other School", 5, 30)
	stationID, _ := enrollTestStation(t, h, orgID, "PC-OWNED")
	revokeTestStation(t, h, stationID)

	req := httptest.NewRequest(http.MethodPost,
		"/admin/v1/b2b/orgs/"+otherOrg.String()+"/stations/"+stationID.String()+"/reactivate", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
	if got := stationStatus(t, h, stationID); got != "revoked" {
		t.Fatalf("status=%q, want it left revoked", got)
	}
}
