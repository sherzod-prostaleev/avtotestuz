package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"avtotest.uz/backend/internal/config"
)

func TestMetricsCountsNonProbeRequests(t *testing.T) {
	h, _, _ := New(config.Config{Env: "dev"}, Deps{})

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

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept", "application/json")
	mrec := httptest.NewRecorder()
	h.ServeHTTP(mrec, req)
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

func TestMetricsPrometheusDefault(t *testing.T) {
	h, _, _ := New(config.Config{Env: "dev"}, Deps{})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}

	mrec := httptest.NewRecorder()
	h.ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if mrec.Code != http.StatusOK {
		t.Fatalf("metrics code=%d", mrec.Code)
	}
	ct := mrec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("Content-Type=%q want text/plain prometheus", ct)
	}
	body := mrec.Body.String()
	for _, needle := range []string{
		"avtotest_http_requests_total 1",
		`avtotest_http_requests_by_status_class_total{class="4xx"} 1`,
		`avtotest_http_request_duration_seconds_bucket{le="+Inf"} 1`,
		"avtotest_http_request_duration_seconds_count 1",
		"avtotest_http_request_duration_seconds_sum ",
		"avtotest_uptime_seconds ",
		"# TYPE avtotest_http_requests_total counter",
		"# TYPE avtotest_http_request_duration_seconds histogram",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("body missing %q:\n%s", needle, body)
		}
	}
}

func TestMetricsJSONViaFormatQuery(t *testing.T) {
	h, _, _ := New(config.Config{Env: "dev"}, Deps{})
	mrec := httptest.NewRecorder()
	h.ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics?format=json", nil))
	if mrec.Code != http.StatusOK {
		t.Fatalf("code=%d", mrec.Code)
	}
	if !strings.Contains(mrec.Header().Get("Content-Type"), "json") &&
		!strings.Contains(mrec.Body.String(), `"requests_total"`) {
		t.Fatalf("expected json envelope, got ct=%q body=%s", mrec.Header().Get("Content-Type"), mrec.Body.String())
	}
	if !strings.Contains(mrec.Body.String(), `"requests_total"`) {
		t.Fatalf("body=%s", mrec.Body.String())
	}
}

func TestSkipMetricsPath(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		if !skipMetricsPath(path) {
			t.Errorf("skipMetricsPath(%q)=false want true", path)
		}
	}
	if skipMetricsPath("/api/v1/categories") {
		t.Error("categories should be counted")
	}
}

func TestWantJSONMetrics(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if wantJSONMetrics(req) {
		t.Fatal("default should be prometheus")
	}
	req.Header.Set("Accept", "application/json")
	if !wantJSONMetrics(req) {
		t.Fatal("Accept json should select json")
	}
	req2 := httptest.NewRequest(http.MethodGet, "/metrics?format=json", nil)
	if !wantJSONMetrics(req2) {
		t.Fatal("format=json should select json")
	}
}

func TestMetricsExcludeWebSocketSessionFromDuration(t *testing.T) {
	metrics := NewRequestMetrics()
	handler := metrics.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/ws", nil)
	req.Header.Set("Connection", "keep-alive, Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	metricsRec := httptest.NewRecorder()
	metrics.WritePrometheus(metricsRec)
	body := metricsRec.Body.String()
	if !strings.Contains(body, "avtotest_http_requests_total 1") {
		t.Fatalf("websocket handshake must remain in request counter:\n%s", body)
	}
	if !strings.Contains(body, "avtotest_http_request_duration_seconds_count 0") {
		t.Fatalf("websocket session must not distort request latency:\n%s", body)
	}
}

func TestMetricsDurationHistogramUsesCumulativeBuckets(t *testing.T) {
	metrics := NewRequestMetrics()
	metrics.ObserveDuration(200 * time.Millisecond)
	rec := httptest.NewRecorder()
	metrics.WritePrometheus(rec)
	body := rec.Body.String()
	for _, needle := range []string{
		`avtotest_http_request_duration_seconds_bucket{le="0.1"} 0`,
		`avtotest_http_request_duration_seconds_bucket{le="0.25"} 1`,
		`avtotest_http_request_duration_seconds_bucket{le="5"} 1`,
		`avtotest_http_request_duration_seconds_bucket{le="+Inf"} 1`,
		"avtotest_http_request_duration_seconds_sum 0.200",
		"avtotest_http_request_duration_seconds_count 1",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("histogram missing %q:\n%s", needle, body)
		}
	}
}
