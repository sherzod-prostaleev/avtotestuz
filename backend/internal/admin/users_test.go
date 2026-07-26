package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminUsersListDetailBlockSessions(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	adminID, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops")
	if err != nil {
		t.Fatal(err)
	}
	_ = adminID

	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name, referral_code, status)
		 VALUES ($1, $2, $3, $4, 'active')`,
		profileID, "+998901112233", "Ali Valiyev", "ALI123"); err != nil {
		t.Fatal(err)
	}
	sessionID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO refresh_token (id, profile_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		sessionID, profileID, "hash-active-1", time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO streak (profile_id, current, best, last_active_date, daily_goal, today_done)
		 VALUES ($1, 5, 10, CURRENT_DATE, 10, 3)`, profileID); err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{Svc: svc, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)

	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("list search by phone name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users?q=Ali&page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListLearnersResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || len(env.Data.Items) != 1 {
			t.Fatalf("got %+v", env.Data)
		}
		if env.Data.Items[0].Phone != "+998901112233" {
			t.Fatalf("list must show full phone for staff, got %q", env.Data.Items[0].Phone)
		}
		if env.Data.Items[0].PhoneMasked == "+998901112233" {
			t.Fatal("phone_masked must still be masked")
		}
		if env.Data.Items[0].Streak != 5 || env.Data.Items[0].Status != "active" {
			t.Fatalf("row=%+v", env.Data.Items[0])
		}
	})

	t.Run("detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/"+profileID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data LearnerDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Phone != "+998901112233" || env.Data.ReferralCode != "ALI123" {
			t.Fatalf("detail=%+v", env.Data)
		}
		raw := w.Body.String()
		if containsStr(raw, "password_hash") || containsStr(raw, `"password":`) {
			t.Fatal("detail must never expose password or hash")
		}
	})

	t.Run("grant vip days", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"days":30,"note":"promo"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/grant", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data struct {
				User  LearnerDetail `json:"user"`
				Days  int           `json:"days"`
				Until string        `json:"until"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Data.User.VIPActive || env.Data.Days != 30 || env.Data.Until == "" {
			t.Fatalf("grant=%+v", env.Data)
		}
		if len(env.Data.User.Entitlements) < 1 {
			t.Fatal("expected entitlement history")
		}
	})

	t.Run("sessions list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users/"+profileID.String()+"/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data []LearnerSessionRow `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data) != 1 || !env.Data[0].Active {
			t.Fatalf("sessions=%+v", env.Data)
		}
	})

	t.Run("block requires reason", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"reason":""}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/block", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", w.Code)
		}
	})

	t.Run("block and revoke sessions", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"reason":"abuse"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/block", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var status string
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM profile WHERE id = $1`, profileID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "banned" {
			t.Fatalf("status=%q want banned", status)
		}
		var active int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM refresh_token WHERE profile_id = $1 AND revoked_at IS NULL`,
			profileID).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active != 0 {
			t.Fatalf("active sessions=%d", active)
		}
		var auditCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM admin_audit_log WHERE action = 'users.block' AND entity_id = $1`,
			profileID.String()).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if auditCount < 1 {
			t.Fatal("expected audit_log for block")
		}
	})

	t.Run("unblock", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"reason":"appeal accepted"}`))
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/"+profileID.String()+"/unblock", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var status string
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM profile WHERE id = $1`, profileID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "active" {
			t.Fatalf("status=%q", status)
		}
	})

	t.Run("revoke single session", func(t *testing.T) {
		sid := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO refresh_token (id, profile_id, token_hash, expires_at)
			 VALUES ($1, $2, $3, $4)`,
			sid, profileID, "hash-active-2", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost,
			"/admin/v1/users/"+profileID.String()+"/sessions/"+sid.String()+"/revoke", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("permission gated without users.read", func(t *testing.T) {
		// Create support-less admin: editor role only
		editorID := uuid.New()
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1, $2, 'Ed', $3, 'active')`,
			editorID, "editor@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code = 'editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403", w.Code)
		}
	})
}

func loginAccess(t *testing.T, r chi.Router, email, password string) string {
	t.Helper()
	body := bytes.NewReader([]byte(`{"email":"` + email + `","password":"` + password + `"}`))
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Tokens TokenPair `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	return env.Data.Tokens.AccessToken
}

func containsStr(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
