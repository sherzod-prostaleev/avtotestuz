package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/testdb"
)

func TestAdminB2B(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3)`,
		profileID, "+998903334455", "Student"); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	var orgID uuid.UUID
	t.Run("create org license member grant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs",
			bytes.NewBufferString(`{"name":"Avtomaktab Demo"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("create org status=%d body=%s", w.Code, w.Body.String())
		}
		var orgEnv struct {
			Data B2BOrgRow `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &orgEnv); err != nil {
			t.Fatal(err)
		}
		orgID = orgEnv.Data.ID

		req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/licenses",
			bytes.NewBufferString(`{"seats":2,"days":30,"note":"pilot"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("license status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/admin/v1/b2b/orgs/"+orgID.String()+"/members",
			bytes.NewBufferString(`{"profile_id":"`+profileID.String()+`","role":"student"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("member status=%d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost,
			"/admin/v1/b2b/orgs/"+orgID.String()+"/members/"+profileID.String()+"/grant",
			bytes.NewBufferString(`{"days":14,"note":"class A"}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("grant status=%d body=%s", w.Code, w.Body.String())
		}

		var source string
		if err := pool.QueryRow(context.Background(),
			`SELECT source FROM entitlement WHERE profile_id=$1 ORDER BY created_at DESC LIMIT 1`,
			profileID).Scan(&source); err != nil {
			t.Fatal(err)
		}
		if source != "b2b" {
			t.Fatalf("source=%s", source)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/b2b/orgs/"+orgID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("detail status=%d", w.Code)
		}
		var detailEnv struct {
			Data B2BOrgDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &detailEnv); err != nil {
			t.Fatal(err)
		}
		if len(detailEnv.Data.Members) != 1 || detailEnv.Data.Org.Seats < 2 {
			t.Fatalf("detail=%+v", detailEnv.Data)
		}
	})
}
