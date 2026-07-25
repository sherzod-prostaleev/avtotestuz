package ops

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestPaymentProviderOpsToggle(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)

	h := &Handler{
		Billing: billing.Service{Q: sqlc.New(pool), Pool: pool},
		Token:   "ops-secret-token",
	}
	r := chi.NewRouter()
	h.Routes(r)

	t.Run("rejects missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ops/payment-providers", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("disable and list", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"enabled":false}`))
		req := httptest.NewRequest(http.MethodPatch, "/ops/payment-providers/click", body)
		req.Header.Set("X-Ops-Token", "ops-secret-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("patch status = %d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/ops/payment-providers", nil)
		req.Header.Set("X-Ops-Token", "ops-secret-token")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data []billing.ProviderStatus `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		var clickEnabled *bool
		for _, p := range env.Data {
			if p.Provider == "click" {
				v := p.Enabled
				clickEnabled = &v
			}
		}
		if clickEnabled == nil || *clickEnabled {
			t.Fatalf("click enabled = %v, want false", clickEnabled)
		}
	})
}

func TestSiteContactsOps(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)

	h := &Handler{
		Billing: billing.Service{Q: sqlc.New(pool), Pool: pool},
		Pool:    pool,
		Token:   "ops-secret-token",
	}
	r := chi.NewRouter()
	h.Routes(r)

	t.Run("rejects missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ops/site-contacts", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("put and get", func(t *testing.T) {
		body := bytes.NewReader([]byte(`{"phone":"+998 71 111 22 33","email":"ops@drivergo.uz","telegramUrl":"https://t.me/x"}`))
		req := httptest.NewRequest(http.MethodPut, "/ops/site-contacts", body)
		req.Header.Set("X-Ops-Token", "ops-secret-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/ops/site-contacts", nil)
		req.Header.Set("X-Ops-Token", "ops-secret-token")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get status = %d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Data["phone"] != "+998 71 111 22 33" || env.Data["email"] != "ops@drivergo.uz" {
			t.Fatalf("contacts = %+v", env.Data)
		}
	})
}
