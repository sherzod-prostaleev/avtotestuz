package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestTariffsEndpoint(t *testing.T) {
	pool := testdb.New(t)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tariff (code, days, price_uzs, old_price_uzs, badge, sort_order, active) VALUES
			('t7', 7, 24900, 34900, NULL, 1, true);
		INSERT INTO tariff_translation (tariff_id, locale, name, description)
		SELECT t.id, 'uz-Latn'::locale_code, 'Seven', 'desc7' FROM tariff t WHERE t.code = 't7';`)
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
	if len(body.Data) != 1 || body.Data[0].Code != "t7" || body.Data[0].PricePerDayUZS != 3557 {
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
