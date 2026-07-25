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
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestAdminPaymentsListDetailProviders(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	q := sqlc.New(pool)

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3)`,
		profileID, "+998901112233", "Pay User"); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	paidAt := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, paid_at, meta)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900, $5, '{"secret":"x","ok":1}')`,
		paymentID, profileID, tariffID, "admin-pay-"+paymentID.String(), paidAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time, perform_time)
		VALUES ($1, $2, 5990000, 2, $3, $4)`,
		"payme-"+paymentID.String(), paymentID, paidAt.UnixMilli()-1000, paidAt.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	clickPay := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, $3, 39900, 'click', 'created', $4, 7, 39900)`,
		clickPay, profileID, tariffID, "admin-click-"+clickPay.String()); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment_provider_status (provider, enabled, updated_by)
		VALUES ('payme', true, 'seed'), ('click', true, 'seed')
		ON CONFLICT (provider) DO UPDATE SET enabled = EXCLUDED.enabled`); err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{
		Svc:     svc,
		Pool:    pool,
		Secret:  secret,
		Billing: billing.Service{Q: q, Pool: pool},
	}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	t.Run("list filters status provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/admin/v1/payments/transactions?status=paid&provider=payme&page=1&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data ListPaymentsResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Total != 1 || len(env.Data.Items) != 1 {
			t.Fatalf("got %+v", env.Data)
		}
		if env.Data.Items[0].AmountUzs != 59900 || env.Data.Items[0].PhoneMasked == "+998901112233" {
			t.Fatalf("row=%+v", env.Data.Items[0])
		}
	})

	t.Run("detail redacts meta and refund note", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/payments/transactions/"+paymentID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data PaymentDetail `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Meta["secret"] != "[redacted]" || env.Data.Meta["ok"] != float64(1) {
			t.Fatalf("meta=%v", env.Data.Meta)
		}
		if env.Data.Refund.ActionAvailable {
			t.Fatal("expected no admin refund action")
		}
		if env.Data.Payme == nil || env.Data.Payme.State != 2 {
			t.Fatalf("payme=%+v", env.Data.Payme)
		}
	})

	t.Run("providers list and patch with audit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/payments/providers", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
		}

		body := bytes.NewBufferString(`{"enabled":false}`)
		req = httptest.NewRequest(http.MethodPatch, "/admin/v1/payments/providers/click", body)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("patch status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data billing.ProviderStatus `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data.Enabled {
			t.Fatal("want click disabled")
		}
		var auditCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM admin_audit_log
			 WHERE action='payments.providers.patch' AND entity_id='click'`).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if auditCount < 1 {
			t.Fatal("expected provider patch audit")
		}
	})

	t.Run("finance can read payments editor denied", func(t *testing.T) {
		financeID := uuid.New()
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
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
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/payments/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+finAccess)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("finance list status=%d", w.Code)
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
		req = httptest.NewRequest(http.MethodGet, "/admin/v1/payments/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+edAccess)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("editor status=%d want 403", w.Code)
		}

		req = httptest.NewRequest(http.MethodPatch, "/admin/v1/payments/providers/payme",
			bytes.NewBufferString(`{"enabled":true}`))
		req.Header.Set("Authorization", "Bearer "+finAccess)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("finance patch providers status=%d want 403 (keys.manage)", w.Code)
		}
	})

	t.Run("recon dry-run", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/payments/recon?hours=24", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("recon status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data struct {
				DryRun bool `json:"dry_run"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if !env.Data.DryRun {
			t.Fatal("expected dry_run true")
		}
	})
}
