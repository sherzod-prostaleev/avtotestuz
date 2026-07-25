package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminLoginAndLearnerIsolation(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	id, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops")
	if err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{Svc: svc, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)

	t.Run("login", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"email":"ops@example.uz","password":"password123"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data struct {
				Tokens TokenPair  `json:"tokens"`
				Admin  MeResponse `json:"admin"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Admin.ID != id || env.Data.Tokens.AccessToken == "" {
			t.Fatalf("unexpected login payload %+v", env.Data)
		}
		if !contains(env.Data.Admin.Permissions, "users.read") {
			t.Fatalf("superadmin missing users.read: %v", env.Data.Admin.Permissions)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+env.Data.Tokens.AccessToken)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("me status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("learner jwt rejected on admin", func(t *testing.T) {
		learnerTok, err := auth.IssueAccess(secret, id, "user", accessTTL)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+learnerTok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", w.Code)
		}
	})

	t.Run("admin jwt rejected by learner parser", func(t *testing.T) {
		adminTok, err := IssueAccess(secret, id, "ops@example.uz", accessTTL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := auth.ParseAccess(secret, adminTok); err == nil {
			t.Fatal("expected learner ParseAccess to reject admin token")
		}
	})

	t.Run("bad password", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"email":"ops@example.uz","password":"wrong-password"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", w.Code)
		}
	})
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
