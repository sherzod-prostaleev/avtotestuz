package server

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"avtotest.uz/backend/internal/httpx"
)

var requestDurationBuckets = [...]time.Duration{
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2500 * time.Millisecond,
	5 * time.Second,
}

var requestDurationBucketLabels = [...]string{"0.1", "0.25", "0.5", "1", "2.5", "5"}

// RequestMetrics exposes low-cardinality, process-local Prometheus counters
// and a latency histogram. Values reset on restart and are not a replacement
// for a durable Prometheus server or multi-instance aggregation.
type RequestMetrics struct {
	startedAt              atomic.Value // time.Time
	total                  atomic.Uint64
	status2xx              atomic.Uint64
	status3xx              atomic.Uint64
	status4xx              atomic.Uint64
	status5xx              atomic.Uint64
	other                  atomic.Uint64
	durationCount          atomic.Uint64
	durationSumNanoseconds atomic.Uint64
	durationBuckets        [len(requestDurationBuckets)]atomic.Uint64
}

func (m *RequestMetrics) ObserveDuration(duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	m.durationCount.Add(1)
	m.durationSumNanoseconds.Add(uint64(duration.Nanoseconds()))
	for i, upperBound := range requestDurationBuckets {
		if duration <= upperBound {
			m.durationBuckets[i].Add(1)
		}
	}
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
		"request_duration_seconds": map[string]any{
			"count": m.durationCount.Load(),
			"sum":   float64(m.durationSumNanoseconds.Load()) / float64(time.Second),
		},
		"requests_by_status_class": map[string]uint64{
			"2xx":   m.status2xx.Load(),
			"3xx":   m.status3xx.Load(),
			"4xx":   m.status4xx.Load(),
			"5xx":   m.status5xx.Load(),
			"other": m.other.Load(),
		},
	}
}

// WritePrometheus writes text exposition (Prometheus 0.0.4) for the counters.
func (m *RequestMetrics) WritePrometheus(w http.ResponseWriter) {
	started, _ := m.startedAt.Load().(time.Time)
	uptime := time.Since(started).Seconds()
	if uptime < 0 {
		uptime = 0
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	var b strings.Builder
	b.WriteString("# HELP avtotest_uptime_seconds Process uptime in seconds since metrics start.\n")
	b.WriteString("# TYPE avtotest_uptime_seconds gauge\n")
	b.WriteString("avtotest_uptime_seconds ")
	b.WriteString(formatFloat(uptime))
	b.WriteByte('\n')
	b.WriteString("# HELP avtotest_http_requests_total HTTP requests observed (probes excluded).\n")
	b.WriteString("# TYPE avtotest_http_requests_total counter\n")
	b.WriteString("avtotest_http_requests_total ")
	b.WriteString(strconv.FormatUint(m.total.Load(), 10))
	b.WriteByte('\n')
	b.WriteString("# HELP avtotest_http_requests_by_status_class_total HTTP requests by status class (probes excluded).\n")
	b.WriteString("# TYPE avtotest_http_requests_by_status_class_total counter\n")
	for _, row := range []struct {
		class string
		n     uint64
	}{
		{"2xx", m.status2xx.Load()},
		{"3xx", m.status3xx.Load()},
		{"4xx", m.status4xx.Load()},
		{"5xx", m.status5xx.Load()},
		{"other", m.other.Load()},
	} {
		b.WriteString(`avtotest_http_requests_by_status_class_total{class="`)
		b.WriteString(row.class)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(row.n, 10))
		b.WriteByte('\n')
	}
	b.WriteString("# HELP avtotest_http_request_duration_seconds End-to-end HTTP request duration (probes and protocol upgrades excluded).\n")
	b.WriteString("# TYPE avtotest_http_request_duration_seconds histogram\n")
	for i, label := range requestDurationBucketLabels {
		b.WriteString(`avtotest_http_request_duration_seconds_bucket{le="`)
		b.WriteString(label)
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(m.durationBuckets[i].Load(), 10))
		b.WriteByte('\n')
	}
	durationCount := m.durationCount.Load()
	b.WriteString(`avtotest_http_request_duration_seconds_bucket{le="+Inf"} `)
	b.WriteString(strconv.FormatUint(durationCount, 10))
	b.WriteByte('\n')
	b.WriteString("avtotest_http_request_duration_seconds_sum ")
	b.WriteString(formatFloat(float64(m.durationSumNanoseconds.Load()) / float64(time.Second)))
	b.WriteByte('\n')
	b.WriteString("avtotest_http_request_duration_seconds_count ")
	b.WriteString(strconv.FormatUint(durationCount, 10))
	b.WriteByte('\n')
	_, _ = w.Write([]byte(b.String()))
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

// Handler serves Prometheus text by default. JSON envelope when the client
// asks for it (`Accept: application/json` or `?format=json`) so FE ops health
// and admin monitoring stay compatible.
func (m *RequestMetrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wantJSONMetrics(r) {
			httpx.Data(w, http.StatusOK, m.Snapshot())
			return
		}
		m.WritePrometheus(w)
	}
}

func wantJSONMetrics(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("format"), "json") {
		return true
	}
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	// Prefer JSON only when explicitly listed (FE ops probe sends application/json).
	// Scrapers typically send text/plain, */*, or openmetrics.
	lower := strings.ToLower(accept)
	return strings.Contains(lower, "application/json")
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
		started := time.Now()
		observeDuration := !isWebSocketUpgrade(r)
		next.ServeHTTP(rec, r)
		m.Observe(rec.status)
		if observeDuration {
			m.ObserveDuration(time.Since(started))
		}
	})
}

func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	for _, value := range r.Header.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
				return true
			}
		}
	}
	return false
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
