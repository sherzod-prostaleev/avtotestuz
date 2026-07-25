package server

import (
	"net/http/httptest"
	"testing"

	"avtotest.uz/backend/internal/config"
)

func TestHealthz(t *testing.T) {
	h, _ := New(config.Config{Env: "dev"}, Deps{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
