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

func TestAdminPaymentVoidPreservesFinancialHistory(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}
	secret := []byte("test-admin-secret-at-least-32-bytes!!")
	q := sqlc.New(pool)

	if _, err := store.EnsureSuperadmin(t.Context(), "ops@example.uz", "password123", "Ops"); err != nil {
		t.Fatal(err)
	}

	profileID := uuid.New()
	referrerID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3), ($4, $5, $6)`,
		profileID, "+998901112233", "Pay User",
		referrerID, "+998909998877", "Referrer"); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}

	paymentID := uuid.New()
	paidAt := time.Now().UTC().Add(-time.Hour)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot, paid_at)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900, $5)`,
		paymentID, profileID, tariffID, "del-pay-"+paymentID.String(), paidAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time, perform_time)
		VALUES ($1, $2, 5990000, 2, $3, $4)`,
		"payme-del-"+paymentID.String(), paymentID, paidAt.UnixMilli()-1000, paidAt.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	entID := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO entitlement (id, profile_id, source, payment_id, starts_at, ends_at, note)
		VALUES ($1, $2, 'purchase', $3, $4, $5, 'vip from pay')`,
		entID, profileID, paymentID, paidAt, paidAt.Add(30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO referral_ledger (profile_id, entry_type, amount_uzs, payment_id, meta)
		VALUES ($1, 'commission', 5000, $2, '{}')`,
		referrerID, paymentID); err != nil {
		t.Fatal(err)
	}

	manualPay := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, $3, 39900, 'manual', 'pending', $4, 7, 39900)`,
		manualPay, profileID, tariffID, "del-manual-"+manualPay.String()); err != nil {
		t.Fatal(err)
	}
	cardID := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO manual_pay_card (id, pan_full, pan_last4, holder_name, sort_order, enabled)
		VALUES ($1, '8600123412341234', '1234', 'Test', 1, true)`, cardID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO manual_pay_assignment (payment_id, card_id, amount_uzs, manual_state, assigned_at, hold_until)
		VALUES ($1, $2, 39900, 'awaiting_transfer', now(), now() + interval '15 minutes')`,
		manualPay, cardID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO manual_pay_event (fingerprint, raw_text, amount_uzs, pan_last4, status, matched_payment_id, parse_ok)
		VALUES ($1, 'push', 39900, '1234', 'matched', $2, true)`,
		"fp-"+manualPay.String(), manualPay); err != nil {
		t.Fatal(err)
	}

	clickPay := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, $3, 19900, 'click', 'canceled', $4, 7, 19900)`,
		clickPay, profileID, tariffID, "del-click-"+clickPay.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO click_transaction (click_trans_id, click_paydoc_id, payment_id, amount_uzs, state)
		VALUES ($1, $2, $3, 19900, -1)`,
		"click-"+clickPay.String(), "doc-"+clickPay.String(), clickPay); err != nil {
		t.Fatal(err)
	}

	svc := Service{Store: store, Secret: secret}
	h := &Handler{
		Svc:     svc,
		Pool:    pool,
		Secret:  secret,
		Billing: billing.Service{Q: q, Pool: pool, Secret: secret},
	}
	r := chi.NewRouter()
	r.Route("/admin/v1", h.Routes)
	access := loginAccess(t, r, "ops@example.uz", "password123")

	voidRequest := func(t *testing.T, token string, id uuid.UUID, reason string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]string{"reason": reason})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/v1/payments/transactions/"+id.String()+"/void", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("settled Payme payment and all related rows are preserved", func(t *testing.T) {
		w := voidRequest(t, access, paymentID, "settled payment must stay immutable")
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d want 409 body=%s", w.Code, w.Body.String())
		}

		var n int
		checks := []struct {
			q string
		}{
			{`SELECT COUNT(*)::int FROM payment WHERE id = $1`},
			{`SELECT COUNT(*)::int FROM payme_transaction WHERE payment_id = $1`},
			{`SELECT COUNT(*)::int FROM entitlement WHERE payment_id = $1`},
			{`SELECT COUNT(*)::int FROM referral_ledger WHERE payment_id = $1`},
		}
		for _, c := range checks {
			if err := pool.QueryRow(context.Background(), c.q, paymentID).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("expected preserved row for %q, got %d", c.q, n)
			}
		}

		var voidCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM payment_void WHERE payment_id=$1`, paymentID).Scan(&voidCount); err != nil {
			t.Fatal(err)
		}
		if voidCount != 0 {
			t.Fatalf("settled payment unexpectedly voided: %d", voidCount)
		}
	})

	t.Run("manual payment is voided without deleting assignment or event", func(t *testing.T) {
		w := voidRequest(t, access, manualPay, "operator confirmed duplicate checkout")
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM manual_pay_assignment WHERE payment_id = $1`, manualPay).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("assignment was not preserved: %d", n)
		}
		var paymentStatus, assignmentStatus, eventStatus string
		var matched *uuid.UUID
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM payment WHERE id=$1`, manualPay).Scan(&paymentStatus); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(),
			`SELECT manual_state FROM manual_pay_assignment WHERE payment_id=$1`, manualPay).Scan(&assignmentStatus); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(),
			`SELECT status, matched_payment_id FROM manual_pay_event WHERE fingerprint = $1`,
			"fp-"+manualPay.String()).Scan(&eventStatus, &matched); err != nil {
			t.Fatal(err)
		}
		if paymentStatus != "voided" || assignmentStatus != "rejected" || eventStatus != "unmatched" || matched != nil {
			t.Fatalf("payment=%s assignment=%s event=%s matched=%v", paymentStatus, assignmentStatus, eventStatus, matched)
		}
		var voidCount, auditCount int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM payment_void WHERE payment_id=$1`, manualPay).Scan(&voidCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM admin_audit_log WHERE action='payments.transactions.void' AND entity_id=$1`, manualPay.String()).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if voidCount != 1 || auditCount != 1 {
			t.Fatalf("void=%d audit=%d want 1/1", voidCount, auditCount)
		}
	})

	t.Run("click transaction remains linked after void", func(t *testing.T) {
		w := voidRequest(t, access, clickPay, "checkout abandoned by the operator")
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var n int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*)::int FROM click_transaction WHERE payment_id = $1`, clickPay).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("click transaction was not preserved: %d", n)
		}
		var status string
		if err := pool.QueryRow(context.Background(), `SELECT status FROM payment WHERE id=$1`, clickPay).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "voided" {
			t.Fatalf("status=%s want voided", status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		w := voidRequest(t, access, uuid.New(), "payment does not exist in database")
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d want 404", w.Code)
		}
	})

	t.Run("editor forbidden", func(t *testing.T) {
		hash, err := auth.HashPassword("password123")
		if err != nil {
			t.Fatal(err)
		}
		editorID := uuid.New()
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user (id, email, display_name, password_hash, status)
			 VALUES ($1,$2,'Ed',$3,'active')`, editorID, "editor-del@example.uz", hash); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO admin_user_role (admin_user_id, role_id)
			 SELECT $1, id FROM admin_role WHERE code='editor'`, editorID); err != nil {
			t.Fatal(err)
		}
		edAccess := loginAccess(t, r, "editor-del@example.uz", "password123")
		orphan := uuid.New()
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
			                     tariff_days_snapshot, tariff_price_uzs_snapshot)
			VALUES ($1, $2, $3, 1000, 'sandbox', 'created', $4, 1, 1000)`,
			orphan, profileID, tariffID, "del-orphan-"+orphan.String()); err != nil {
			t.Fatal(err)
		}
		w := voidRequest(t, edAccess, orphan, "operator requested a safe payment void")
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403", w.Code)
		}
	})
}
