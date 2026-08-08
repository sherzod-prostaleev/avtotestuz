package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"avtotest.uz/backend/internal/config"
)

func TestHealthz(t *testing.T) {
	h, _, _ := New(config.Config{Env: "dev"}, Deps{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzSkippedDeps(t *testing.T) {
	h, _, _ := New(config.Config{Env: "dev"}, Deps{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/readyz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if env.Data.Status != "ok" {
		t.Fatalf("status=%q want ok", env.Data.Status)
	}
	if env.Data.Checks["postgres"] != "skipped" || env.Data.Checks["redis"] != "skipped" {
		t.Fatalf("checks=%v want postgres+redis skipped", env.Data.Checks)
	}
}
