package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminMonitoring(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	snapCalls := 0
	h := &Handler{
		Svc:    Service{Store: store, Secret: secret},
		Pool:   pool,
		Secret: secret,
		MetricsSnapshot: func() map[string]any {
			snapCalls++
			return map[string]any{
				"uptime_seconds": int64(42),
				"requests_total": uint64(7),
				"requests_by_status_class": map[string]uint64{
					"2xx": 5, "3xx": 0, "4xx": 1, "5xx": 1, "other": 0,
				},
			}
		},
	}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("health postgres ok redis skipped", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/health", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data struct {
				Status string            `json:"status"`
				Live   string            `json:"live"`
				Checks map[string]string `json:"checks"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Status != "ok" || env.Data.Live != "ok" {
			t.Fatalf("got %+v", env.Data)
		}
		if env.Data.Checks["postgres"] != "ok" || env.Data.Checks["redis"] != "skipped" {
			t.Fatalf("checks=%v", env.Data.Checks)
		}
	})

	t.Run("metrics snapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data["requests_total"].(float64) != 7 || snapCalls < 1 {
			t.Fatalf("data=%v calls=%d", env.Data, snapCalls)
		}
		host, ok := env.Data["host"].(map[string]any)
		if !ok || host["available"].(bool) {
			t.Fatalf("host should be unavailable: %+v", env.Data["host"])
		}
	})

	t.Run("stream health sse", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/stream", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		go func() {
			time.Sleep(150 * time.Millisecond)
			cancel()
		}()
		r.ServeHTTP(w, req)
		body := w.Body.String()
		if !strings.Contains(body, "event: health") || !strings.Contains(body, `"status":"ok"`) {
			t.Fatalf("sse body=%q", body)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
			t.Fatalf("content-type=%q", ct)
		}
	})

	t.Run("jobs catalog honest manual", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data struct {
				Items []JobCatalogRow `json:"items"`
				Note  string          `json:"note"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data.Items) < 3 || env.Data.Note == "" {
			t.Fatalf("got %+v", env.Data)
		}
		for _, item := range env.Data.Items {
			if item.Status != "manual" || item.Kind != "cli" {
				t.Fatalf("item=%+v", item)
			}
		}
	})

	t.Run("feed and alerts", func(t *testing.T) {
		if err := store.WriteAudit(context.Background(), nil, "monitoring.test", "probe", "1",
			nil, map[string]any{"ok": true}, nil, "", ""); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/feed", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("feed status=%d body=%s", w.Code, w.Body.String())
		}
		var feedEnv struct {
			Data OpsFeedResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &feedEnv); err != nil {
			t.Fatal(err)
		}
		if len(feedEnv.Data.Items) < 1 {
			t.Fatalf("feed empty: %+v", feedEnv.Data)
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/alerts", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("alerts status=%d", w.Code)
		}
		var alertEnv struct {
			Data struct {
				Items []AlertEval `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &alertEnv); err != nil {
			t.Fatal(err)
		}
		if len(alertEnv.Data.Items) < 2 {
			t.Fatalf("alerts=%+v", alertEnv.Data.Items)
		}
		foundPG := false
		for _, a := range alertEnv.Data.Items {
			if a.ID == "postgres_ready" && a.Status == "ok" {
				foundPG = true
			}
		}
		if !foundPG {
			t.Fatalf("postgres_ready missing/ok: %+v", alertEnv.Data.Items)
		}
	})

	t.Run("finance denied; analyst allowed", func(t *testing.T) {
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		financeID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Fin',$3,'active')`, financeID, "finance@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='finance'`, financeID); err != nil {
			t.Fatal(err)
		}
		finAccess := loginAccess(t, r, "finance@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/health", nil)
		req.Header.Set("Authorization", "Bearer "+finAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("finance status=%d want 403", w.Code)
		}

		analystID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'An',$3,'active')`, analystID, "analyst@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='analyst'`, analystID); err != nil {
			t.Fatal(err)
		}
		anAccess := loginAccess(t, r, "analyst@example.uz", "password123")
		req = httptest.NewRequest(http.MethodGet, "/admin/v1/monitoring/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+anAccess)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("analyst status=%d", w.Code)
		}
	})
}
