package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	if env.Data.Checks["postgres"] != "skipped" || env.Data.Checks["redis"] != "skipped" || env.Data.Checks["object_storage"] != "skipped" {
		t.Fatalf("checks=%v want postgres+redis+object_storage skipped", env.Data.Checks)
	}
}

type readinessBlob struct{ err error }

func (s readinessBlob) Put(context.Context, string, string, []byte) error { return s.err }
func (s readinessBlob) Get(context.Context, string) ([]byte, string, error) {
	return nil, "", s.err
}
func (s readinessBlob) Health(context.Context) error { return s.err }

func TestReadyzFailsWhenRequiredObjectStorageIsDown(t *testing.T) {
	h := readinessHandler(nil, nil, readinessBlob{err: errors.New("down")}, nil, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzAcceptsHealthyRequiredObjectStorage(t *testing.T) {
	h := readinessHandler(nil, nil, readinessBlob{}, nil, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
