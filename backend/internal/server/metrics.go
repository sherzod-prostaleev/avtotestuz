package server

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"avtotest.uz/backend/internal/httpx"
)

// RequestMetrics is a tiny in-process counter for U-41 — not a Prometheus stack.
// Process-local only; resets on restart. Enough for ops smoke and staging eyeballing.
type RequestMetrics struct {
	startedAt atomic.Value // time.Time
	total     atomic.Uint64
	status2xx atomic.Uint64
	status3xx atomic.Uint64
	status4xx atomic.Uint64
	status5xx atomic.Uint64
	other     atomic.Uint64
}

func NewRequestMetrics() *RequestMetrics {
	m := &RequestMetrics{}
	m.startedAt.Store(time.Now().UTC())
	return m
}

func (m *RequestMetrics) Observe(status int) {
	m.total.Add(1)
	switch {
	case status >= 200 && status < 300:
		m.status2xx.Add(1)
	case status >= 300 && status < 400:
		m.status3xx.Add(1)
	case status >= 400 && status < 500:
		m.status4xx.Add(1)
	case status >= 500 && status < 600:
		m.status5xx.Add(1)
	default:
		m.other.Add(1)
	}
}

func (m *RequestMetrics) Snapshot() map[string]any {
	started, _ := m.startedAt.Load().(time.Time)
	uptime := time.Since(started).Seconds()
	if uptime < 0 {
		uptime = 0
	}
	return map[string]any{
		"uptime_seconds": int64(uptime),
		"requests_total": m.total.Load(),
		"requests_by_status_class": map[string]uint64{
			"2xx":   m.status2xx.Load(),
			"3xx":   m.status3xx.Load(),
			"4xx":   m.status4xx.Load(),
			"5xx":   m.status5xx.Load(),
			"other": m.other.Load(),
		},
	}
}

func (m *RequestMetrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpx.Data(w, http.StatusOK, m.Snapshot())
	}
}

// Middleware counts completed requests. Skips /healthz, /readyz, /metrics so
// probe scrapes do not dominate the counter.
func (m *RequestMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skipMetricsPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		m.Observe(rec.status)
	})
}

func skipMetricsPath(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return strings.HasPrefix(path, "/metrics/")
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
