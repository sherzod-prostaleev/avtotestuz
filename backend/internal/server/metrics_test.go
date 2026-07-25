package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"avtotest.uz/backend/internal/config"
)

func TestMetricsCountsNonProbeRequests(t *testing.T) {
	h, _ := New(config.Config{Env: "dev"}, Deps{})

	// Probe paths must not inflate counters.
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s code=%d", path, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing route code=%d want 404", rec.Code)
	}

	mrec := httptest.NewRecorder()
	h.ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if mrec.Code != http.StatusOK {
		t.Fatalf("metrics code=%d body=%s", mrec.Code, mrec.Body.String())
	}
	var env struct {
		Data struct {
			RequestsTotal uint64 `json:"requests_total"`
			ByStatus      struct {
				S4xx uint64 `json:"4xx"`
			} `json:"requests_by_status_class"`
			Uptime int64 `json:"uptime_seconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(mrec.Body.Bytes(), &env); err != nil {
		t.Fatalf("json: %v body=%s", err, mrec.Body.String())
	}
	if env.Data.RequestsTotal != 1 {
		t.Fatalf("requests_total=%d want 1 (probes excluded)", env.Data.RequestsTotal)
	}
	if env.Data.ByStatus.S4xx != 1 {
		t.Fatalf("4xx=%d want 1", env.Data.ByStatus.S4xx)
	}
	if env.Data.Uptime < 0 {
		t.Fatalf("uptime_seconds=%d", env.Data.Uptime)
	}
}

func TestSkipMetricsPath(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		if !skipMetricsPath(path) {
			t.Errorf("skipMetricsPath(%q)=false want true", path)
		}
	}
	if skipMetricsPath("/api/v1/me") {
		t.Fatal("skipMetricsPath(/api/v1/me)=true want false")
	}
}
