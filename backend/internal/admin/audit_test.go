package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminAuditLog(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	adminID, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops")
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("list includes login audit + filters", func(t *testing.T) {
		before := map[string]any{"status": "active"}
		after := map[string]any{"status": "blocked", "reason": "abuse"}
		if err := store.WriteAudit(context.Background(), &adminID, "users.block", "profile", uuid.New().String(),
			before, after, nil, "vitest-ua", "req-audit-1"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/security/audit?page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListAdminAuditResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total < 2 {
			t.Fatalf("expected login + block audits, total=%d", env.Data.Total)
		}
		foundBlock := false
		for _, item := range env.Data.Items {
			if item.Action == "users.block" && item.EntityType == "profile" {
				foundBlock = true
				if item.AdminEmail != "ops@example.uz" {
					t.Fatalf("admin_email=%q", item.AdminEmail)
				}
				if len(item.AfterJSON) == 0 {
					t.Fatal("expected after_json")
				}
			}
		}
		if !foundBlock {
			t.Fatal("users.block row missing")
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/security/audit?action=users.block&entity_type=profile", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("filter status=%d", w.Code)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total < 1 {
			t.Fatalf("filtered total=%d", env.Data.Total)
		}
		for _, item := range env.Data.Items {
			if item.Action != "users.block" || item.EntityType != "profile" {
				t.Fatalf("unexpected row action=%s entity=%s", item.Action, item.EntityType)
			}
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
			 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor-audit@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor-audit@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/security/audit", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("editor status=%d want 403", w.Code)
		}
	})
}
