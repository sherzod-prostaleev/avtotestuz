package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestListLimits(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	h := &Handler{
		Billing: billing.Service{Q: q, Pool: pool},
		Pool:    pool,
		Token:   "ops-secret-token",
	}
	r := chi.NewRouter()
	h.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/ops/limits", nil)
	req.Header.Set("X-Ops-Token", "ops-secret-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data []LimitRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) < 4 {
		t.Fatalf("limits=%d want seeded rows", len(env.Data))
	}
	found := false
	for _, row := range env.Data {
		if row.Key == "daily_practice_questions" {
			found = true
			if row.FreeValue <= 0 {
				t.Fatalf("daily_practice free=%d", row.FreeValue)
			}
		}
	}
	if !found {
		t.Fatal("missing daily_practice_questions")
	}

	bad := httptest.NewRequest(http.MethodGet, "/ops/limits", nil)
	bw := httptest.NewRecorder()
	r.ServeHTTP(bw, bad)
	if bw.Code != http.StatusUnauthorized {
		t.Fatalf("no token status=%d", bw.Code)
	}
}
