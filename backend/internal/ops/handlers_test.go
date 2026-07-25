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
