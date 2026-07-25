package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestTariffsEndpoint(t *testing.T) {
	pool := testdb.New(t)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tariff (code, days, price_uzs, old_price_uzs, badge, sort_order, active) VALUES
			('t7', 7, 24900, 34900, NULL, 1, true) ON CONFLICT (code) DO NOTHING;
		INSERT INTO tariff_translation (tariff_id, locale, name, description)
		SELECT t.id, 'uz-Latn'::locale_code, 'Seven', 'desc7' FROM tariff t WHERE t.code = 't7'
		ON CONFLICT (tariff_id, locale) DO NOTHING;`)
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: Service{Q: sqlc.New(pool)}}
	r := chi.NewRouter()
	h.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/tariffs?locale=uz-Latn", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data []TariffDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	var found *TariffDTO
	for i := range body.Data {
		if body.Data[i].Code == "t7" {
			found = &body.Data[i]
			break
		}
	}
	if found == nil || found.PricePerDayUZS != 3557 {
		t.Fatalf("unexpected tariffs: %+v", body.Data)
	}

	// invalid locale → 400
	req2 := httptest.NewRequest(http.MethodGet, "/tariffs?locale=xx", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("bad locale status = %d, want 400", w2.Code)
	}
}

func TestValidatePromoEndpoint(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901000055') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO promo_code (code, kind, value, active) VALUES ('SAVE20', 'percent', 20, true) ON CONFLICT (code) DO UPDATE SET active = true, value = 20`); err != nil {
		t.Fatal(err)
	}

	const secret = "test-secret-12345"
	tok, err := auth.IssueAccess([]byte(secret), profileID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: Service{Q: sqlc.New(pool)}}
	r := chi.NewRouter()
	r.Use(auth.Required([]byte(secret)))
	h.AuthedRoutes(r)

	// Valid promo
	body := strings.NewReader(`{"code":"SAVE20","tariff_code":"gentra"}`)
	req := httptest.NewRequest(http.MethodPost, "/billing/promo/validate?locale=uz-Latn", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res struct {
		Data ValidatePromoResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Data.DiscountUzs != 11980 || res.Data.FinalAmountUzs != 47920 {
		t.Errorf("unexpected res: %+v", res.Data)
	}

	// Invalid promo code -> 404 promo_not_found
	body2 := strings.NewReader(`{"code":"INVALIDCODE","tariff_code":"gentra"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/billing/promo/validate?locale=uz-Latn", body2)
	req2.Header.Set("Authorization", "Bearer "+tok)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body=%s", w2.Code, w2.Body.String())
	}
}

func TestCheckoutEndpointReturnURL(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901000056') ON CONFLICT (phone) DO UPDATE SET id = EXCLUDED.id`, profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tariff (code, days, price_uzs, sort_order, active) VALUES ('gentra', 30, 59900, 1, true) ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 59900, days = 30`); err != nil {
		t.Fatal(err)
	}

	const secret = "test-secret-12345"
	const publicBase = "https://app.example.com"
	tok, err := auth.IssueAccess([]byte(secret), profileID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{
		Svc:               Service{Q: sqlc.New(pool), Pool: pool, PublicBaseURL: publicBase},
		PaymeMerchantID:   "M1",
		PaymeCheckoutHost: "https://checkout.paycom.uz",
		ClickServiceID:    "S1",
		ClickMerchantID:   "M1",
	}
	r := chi.NewRouter()
	r.Use(auth.Required([]byte(secret)))
	h.AuthedRoutes(r)

	wantReturn := publicBase + "/ru/checkout/pending"

	t.Run("payme", func(t *testing.T) {
		body := strings.NewReader(`{"tariff_code":"gentra","provider":"payme"}`)
		req := httptest.NewRequest(http.MethodPost, "/me/checkout?locale=ru", body)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		var res struct {
			Data CheckoutResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		b64 := strings.TrimPrefix(res.Data.CheckoutURL, "https://checkout.paycom.uz/")
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("decode payme url: %v", err)
		}
		if !strings.Contains(string(raw), "c="+wantReturn) {
			t.Errorf("payme callback = %q, want c=%s", string(raw), wantReturn)
		}
	})

	t.Run("click", func(t *testing.T) {
		body := strings.NewReader(`{"tariff_code":"gentra","provider":"click"}`)
		req := httptest.NewRequest(http.MethodPost, "/me/checkout?locale=ru", body)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		var res struct {
			Data CheckoutResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(res.Data.CheckoutURL)
		if err != nil {
			t.Fatalf("parse click url: %v", err)
		}
		if got := parsed.Query().Get("return_url"); got != wantReturn {
			t.Errorf("click return_url = %q, want %q", got, wantReturn)
		}
	})
}
