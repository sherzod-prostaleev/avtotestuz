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

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/support"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminSupportInbox(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	created, err := support.Store{Pool: pool}.Create(context.Background(), support.CreateInput{
		ContactEmail: "a@b.uz",
		Subject:      "Inbox test",
		Body:         "Please help",
		Source:       "public",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("list get patch + audit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/support/tickets?status=open", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
		}
		var listEnv struct {
			Data support.ListResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listEnv); err != nil {
			t.Fatal(err)
		}
		if listEnv.Data.Total < 1 {
			t.Fatalf("total=%d", listEnv.Data.Total)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/support/tickets/"+created.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get status=%d", w.Code)
		}

		body := `{"status":"resolved","admin_note":"done"}`
		req = httptest.NewRequest(http.MethodPatch, "/admin/v1/support/tickets/"+created.ID.String(), bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("patch status=%d body=%s", w.Code, w.Body.String())
		}
		var patchEnv struct {
			Data support.Ticket `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &patchEnv); err != nil {
			t.Fatal(err)
		}
		if patchEnv.Data.Status != "resolved" {
			t.Fatalf("status=%s", patchEnv.Data.Status)
		}
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM admin_audit_log WHERE action='support.ticket.patch' AND entity_id=$1`,
			created.ID.String()).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n < 1 {
			t.Fatalf("audit=%d", n)
		}
	})

	t.Run("editor denied", func(t *testing.T) {
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		editorID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor-inbox@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor-inbox@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/support/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("editor status=%d want 403", w.Code)
		}
	})
}
