package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestListPaymentsAndAudit(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)

	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, "+998909998877"); err != nil {
		t.Fatal(err)
	}
	tariffID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatal(err)
	}
	paymentID := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		VALUES ($1, $2, $3, 59900, 'payme', 'paid', $4, 30, 59900)`,
		paymentID, profileID, tariffID, "ops-pay-"+paymentID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO audit_log (action, entity, entity_id, after)
		VALUES ('ops.test', 'payment', $1, '{}')`, paymentID); err != nil {
		t.Fatal(err)
	}

	h := &Handler{
		Billing: billing.Service{Q: q, Pool: pool},
		Pool:    pool,
		Token:   "ops-secret-token",
	}
	r := chi.NewRouter()
	h.Routes(r)

	t.Run("payments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ops/payments?status=paid", nil)
		req.Header.Set("X-Ops-Token", "ops-secret-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data []PaymentRow `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data) != 1 || env.Data[0].AmountUzs != 59900 {
			t.Fatalf("payments=%+v", env.Data)
		}
	})

	t.Run("audit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ops/audit", nil)
		req.Header.Set("X-Ops-Token", "ops-secret-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data []AuditRow `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data) < 1 {
			t.Fatalf("audit empty at %s", time.Now())
		}
		found := false
		for _, row := range env.Data {
			if row.Action == "ops.test" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing ops.test in %+v", env.Data)
		}
	})
}

func TestListPaymentsRequiresToken(t *testing.T) {
	pool := testdb.New(t)
	h := &Handler{Pool: pool, Token: "x"}
	r := chi.NewRouter()
	h.Routes(r)
	req := httptest.NewRequest(http.MethodGet, "/ops/payments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401 (%v)", w.Code, fmt.Errorf("unauthorized"))
	}
}
