package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"avtotest.uz/backend/internal/account"
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"github.com/go-chi/chi/v5"
)

func TestB2BMobilePromoIsolationAndControl(t *testing.T) {
	h, r, access := newInstallerTestHandler(t, "qr-ops@example.uz")
	org := createInstallerTestOrg(t, r, access, "QR school", 3, 30)
	other := createInstallerTestOrg(t, r, access, "Other school", 3, 30)
	stationID, profileID := enrollTestStation(t, h, org, "QR-PC")
	_, otherProfile := enrollTestStation(t, h, other, "OTHER-PC")
	endpoint := "/admin/v1/b2b/orgs/" + org.String() + "/mobile-promo"
	request := func(method, token string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, endpoint, bytes.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := request("PUT", "", []byte(`{"enabled":true,"url":"https://example.com"}`)); w.Code != 401 {
		t.Fatalf("unauthenticated write: %d", w.Code)
	}
	raw := "https://drivergo.uz/r/REF-C62LC2?x=%2f&x=A+b#Exact"
	save := func(enabled bool) {
		body, _ := json.Marshal(map[string]any{"enabled": enabled, "url": raw})
		w := request("PUT", access, body)
		if w.Code != 200 {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
		var env struct {
			Data b2b.MobilePromo `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.URL != raw || env.Data.QRDataURL == "" || env.Data.Enabled != enabled {
			t.Fatalf("bad saved promo: %+v", env.Data)
		}
	}
	save(true)
	bs := b2b.Store{Pool: h.Pool}
	p, err := bs.StationMobilePromo(t.Context(), profileID)
	if err != nil || p.URL != raw || !p.Enabled {
		t.Fatalf("own station: %+v %v", p, err)
	}
	p, err = bs.StationMobilePromo(t.Context(), otherProfile)
	if err != nil || p.Enabled || p.URL != "" {
		t.Fatalf("cross-school leak: %+v %v", p, err)
	}
	for _, bad := range []string{`{"enabled":true}`, `{"enabled":true,"url":"javascript:alert(1)"}`, `{"enabled":true,"url":" https://example.com"}`, `{"enabled":true,"url":"https://example.com","extra":1}`, `{"enabled":false,"url":""} {}`} {
		if w := request("PUT", access, []byte(bad)); w.Code != 400 {
			t.Fatalf("invalid input: %d %s", w.Code, w.Body.String())
		}
	}
	// Exercise the real authenticated kiosk route, with no caller-selected org.
	ar := chi.NewRouter()
	ah := account.Handler{Q: sqlc.New(h.Pool), Billing: billing.Service{Pool: h.Pool}}
	ah.Routes(ar.With(auth.Required(h.Secret)))
	token, err := auth.IssueStationAccess(h.Secret, stationID, profileID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/me/mobile-promo?org_id="+other.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	ar.ServeHTTP(w, req)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("REF-C62LC2")) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("station endpoint: %d %s", w.Code, w.Body.String())
	}
	save(false)
	p, err = bs.StationMobilePromo(t.Context(), profileID)
	if err != nil || p.Enabled || p.URL != "" || p.QRDataURL != "" {
		t.Fatalf("disabled leak: %+v %v", p, err)
	}
	save(true)
	if _, err := h.Pool.Exec(t.Context(), `UPDATE b2b_org SET status='suspended' WHERE id=$1`, org); err != nil {
		t.Fatal(err)
	}
	p, err = bs.StationMobilePromo(t.Context(), profileID)
	if err != nil || p.Enabled {
		t.Fatalf("suspended school: %+v %v", p, err)
	}
	var count int
	if err := h.Pool.QueryRow(t.Context(), `SELECT count(*) FROM admin_audit_log WHERE action='b2b.orgs.mobile_promo' AND entity_id=$1`, org.String()).Scan(&count); err != nil || count != 3 {
		t.Fatalf("audit count %d: %v", count, err)
	}
}
