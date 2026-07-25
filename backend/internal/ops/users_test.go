package ops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestListUsersMasked(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	q := sqlc.New(pool)
	profileID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO profile (id, phone, name) VALUES ($1, $2, $3)`,
		profileID, "+998901112233", "Ops User"); err != nil {
		t.Fatal(err)
	}

	h := &Handler{
		Billing: billing.Service{Q: q, Pool: pool},
		Pool:    pool,
		Token:   "ops-secret-token",
	}
	r := chi.NewRouter()
	h.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/ops/users", nil)
	req.Header.Set("X-Ops-Token", "ops-secret-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data []UserRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("len=%d", len(env.Data))
	}
	if env.Data[0].PhoneMasked == "+998901112233" || !containsStars(env.Data[0].PhoneMasked) {
		t.Fatalf("phone not masked: %q", env.Data[0].PhoneMasked)
	}
	if env.Data[0].Name != "Ops User" {
		t.Fatalf("name=%q", env.Data[0].Name)
	}
}

func containsStars(s string) bool {
	return len(s) >= 3 && (s[0] == '+' || s[0] == '9' || s[0] == '*') && (containsRune(s, '*'))
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

func TestMaskPhone(t *testing.T) {
	got := maskPhone("+998901112233")
	if got != "+998***2233" {
		t.Fatalf("mask=%q", got)
	}
}
