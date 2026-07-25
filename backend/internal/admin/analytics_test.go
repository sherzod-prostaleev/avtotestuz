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

func TestAdminAnalyticsOverview(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	p1 := uuid.New()
	p2 := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name, created_at) VALUES
		 ($1, '+998901111111', 'A', now() - interval '2 hours'),
		 ($2, '+998902222222', 'B', now() - interval '10 days')`,
		p1, p2); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, paid_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900, now() - interval '1 day')`,
		uuid.New(), p1, tariffID, "analytics-pay-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO entitlement (profile_id, source, starts_at, ends_at)
		VALUES ($1, 'purchase', now() - interval '1 day', now() + interval '20 days')`, p1); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event (profile_id, name, props, ts) VALUES
		 ($1, 'session_start', '{}', now() - interval '1 hour'),
		 ($1, 'session_start', '{}', now() - interval '2 hours'),
		 ($1, 'checkout_start', '{}', now() - interval '3 hours')`, p1); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: Service{Store: store, Secret: secret}, Pool: pool, Secret: secret}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("overview aggregates", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/analytics/overview", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data AnalyticsOverview `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.ProfilesTotal < 2 || env.Data.ProfilesCreated24h < 1 {
			t.Fatalf("profiles=%+v", env.Data)
		}
		if env.Data.PaymentsPaidTotal != 1 || env.Data.RevenuePaidUzsTotal != 59900 {
			t.Fatalf("payments=%+v", env.Data)
		}
		if env.Data.EntitlementsActive < 1 || env.Data.EventsLast7d < 3 {
			t.Fatalf("ent/events=%+v", env.Data)
		}
		if len(env.Data.TopEventNames7d) == 0 || env.Data.TopEventNames7d[0].Name != "session_start" {
			t.Fatalf("top=%+v", env.Data.TopEventNames7d)
		}
		if env.Data.GeneratedAt.IsZero() || time.Since(env.Data.GeneratedAt) > time.Minute {
			t.Fatalf("generated_at=%v", env.Data.GeneratedAt)
		}
	})

	t.Run("editor denied; finance ok", func(t *testing.T) {
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		editorID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor@example.uz", "password123")
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/analytics/overview", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("editor status=%d want 403", w.Code)
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
		req = httptest.NewRequest(http.MethodGet, "/admin/v1/analytics/overview", nil)
		req.Header.Set("Authorization", "Bearer "+finAccess)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("finance status=%d", w.Code)
		}
	})
}

func TestAdminInvestorsOverview(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/investors/overview", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data AnalyticsOverview `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(env.Data.Note, "Investor") {
		t.Fatalf("note=%q", env.Data.Note)
	}

	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	editorID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO admin_user (id, email, display_name, password_hash, status)
		 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor-inv@example.uz", hash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO admin_user_role (admin_user_id, role_id)
		 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
		t.Fatal(err)
	}
	edAccess := loginAccess(t, r, "editor-inv@example.uz", "password123")
	req = httptest.NewRequest(http.MethodGet, "/admin/v1/investors/overview", nil)
	req.Header.Set("Authorization", "Bearer "+edAccess)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("editor status=%d want 403", w.Code)
	}
}
