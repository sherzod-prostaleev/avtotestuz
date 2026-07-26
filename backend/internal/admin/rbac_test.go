package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminRBACMatrix(t *testing.T) {
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

	t.Run("superadmin lists matrix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/security/rbac", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data RBACMatrix `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data.Permissions) < 10 {
			t.Fatalf("expected permissions, got %d", len(env.Data.Permissions))
		}
		if len(env.Data.Roles) < 5 {
			t.Fatalf("expected roles, got %d", len(env.Data.Roles))
		}
		var superadmin *RBACRole
		for i := range env.Data.Roles {
			if env.Data.Roles[i].Code == "superadmin" {
				superadmin = &env.Data.Roles[i]
				break
			}
		}
		if superadmin == nil || len(superadmin.Permissions) != len(env.Data.Permissions) {
			t.Fatalf("superadmin permissions=%d total=%d", len(superadmin.Permissions), len(env.Data.Permissions))
		}
	})

	t.Run("editor forbidden", func(t *testing.T) {
		editorID := uuid.New()
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(t.Context(), `
			INSERT INTO admin_user (id, email, display_name, password_hash, status)
			VALUES ($1, 'editor@example.uz', 'Editor', $2, 'active')`, editorID, hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(t.Context(), `
			INSERT INTO admin_user_role (admin_user_id, role_id)
			SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		editorAccess := loginAccess(t, r, "editor@example.uz", "password123")

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/security/rbac", nil)
		req.Header.Set("Authorization", "Bearer "+editorAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
}
