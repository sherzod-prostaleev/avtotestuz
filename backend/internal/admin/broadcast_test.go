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
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/push"
	"avtotest.uz/backend/internal/site"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminSupportBroadcast(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	q := sqlc.New(pool)
	fake := &push.FakeSender{}
	pushSvc := push.NewService(pool, q, push.Config{
		PublicKey: "BPtest", PrivateKey: "priv", Subject: "mailto:t@example.com",
	}, fake)

	h := &Handler{
		Svc:    Service{Store: store, Secret: secret},
		Pool:   pool,
		Secret: secret,
		Push:   pushSvc,
	}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("banner put get + audit", func(t *testing.T) {
		body := `{"enabled":true,"message":"VIP skidka","href":"/uz-Latn/premium"}`
		req := httptest.NewRequest(http.MethodPut, "/admin/v1/support/banner", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data site.SupportBanner `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Data.Enabled || env.Data.Message != "VIP skidka" {
			t.Fatalf("banner=%+v", env.Data)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/support/banner", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get status=%d", w.Code)
		}

		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM admin_audit_log WHERE action='support.banner.put'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n < 1 {
			t.Fatalf("audit count=%d", n)
		}
	})

	t.Run("webpush dry_run + send", func(t *testing.T) {
		profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901170001"})
		if err != nil {
			t.Fatal(err)
		}
		if err := pushSvc.Subscribe(context.Background(), profile.ID, push.SubscribeInput{
			Endpoint: "https://push.example/bc1", P256dh: "p", Auth: "a",
		}); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/v1/support/broadcasts/webpush",
			bytes.NewBufferString(`{"title":"Go","body":"Hello","dry_run":true}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("dry status=%d body=%s", w.Code, w.Body.String())
		}
		var dryEnv struct {
			Data push.BroadcastResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &dryEnv); err != nil {
			t.Fatal(err)
		}
		if dryEnv.Data.Recipients < 1 || !dryEnv.Data.DryRun {
			t.Fatalf("dry=%+v", dryEnv.Data)
		}

		req = httptest.NewRequest(http.MethodPost, "/admin/v1/support/broadcasts/webpush",
			bytes.NewBufferString(`{"title":"Go","body":"Hello","url":"/uz-Latn/dashboard","dry_run":false}`))
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("send status=%d body=%s", w.Code, w.Body.String())
		}
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM admin_audit_log WHERE action='support.broadcast.webpush'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n < 2 {
			t.Fatalf("webpush audit count=%d", n)
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
			 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor-bc@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor-bc@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/support/banner", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("editor status=%d want 403", w.Code)
		}
	})
}
