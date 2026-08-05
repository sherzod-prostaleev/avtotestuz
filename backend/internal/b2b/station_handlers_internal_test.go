package b2b

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testredis"
)

// TestStationRateLimitIdentityDimension drives (*Handler).allow directly with a
// low limit instead of sending hundreds of HTTP requests through a handler
// that hardcodes production limits (40-600/hour). allow's boolean return is
// exactly what each station handler turns into 429: the moment it flips
// false, stationChallenge/stationToken/enrollStation all write
// http.StatusTooManyRequests and return.
func TestStationRateLimitIdentityDimension(t *testing.T) {
	rdb := testredis.New(t)
	h := &Handler{Lim: auth.Limiter{R: rdb}}
	req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
	req.RemoteAddr = "203.0.113.7:54321"

	const limit = 2
	for i := 1; i <= limit; i++ {
		if !h.allow(req, "test-exhaust-id", "station-abc", limit, 1000, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	// The (limit+1)th call is the one a real handler turns into 429.
	if h.allow(req, "test-exhaust-id", "station-abc", limit, 1000, time.Hour) {
		t.Fatalf("call %d/%d: want denied (429), got allowed", limit+1, limit)
	}

	// A different identity must not share the exhausted bucket.
	if !h.allow(req, "test-exhaust-id", "station-xyz", limit, 1000, time.Hour) {
		t.Fatal("different identity was denied by another identity's exhausted bucket")
	}
}

// TestStationRateLimitIPDimension exercises the secondary IP dimension the same
// way, proving it still applies (finding 1 says keep it, not delete it) even
// though identity now carries the primary protection.
func TestStationRateLimitIPDimension(t *testing.T) {
	rdb := testredis.New(t)
	h := &Handler{Lim: auth.Limiter{R: rdb}}

	const limit = 2
	for i := 1; i <= limit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
		req.RemoteAddr = "198.51.100.9:1111"
		// A fresh identity per call so only the IP dimension can deny.
		identity := fmt.Sprintf("station-%d", i)
		if !h.allow(req, "test-exhaust-ip", identity, 1000, limit, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
	req.RemoteAddr = "198.51.100.9:2222"
	if h.allow(req, "test-exhaust-ip", "station-final", 1000, limit, time.Hour) {
		t.Fatalf("call %d/%d: want denied (429) by the IP dimension, got allowed", limit+1, limit)
	}
}

// TestStationRateLimitSkipsEmptyIP covers finding 2: an unresolvable IP must
// not become the shared key "station:<action>:ip:" for every such caller.
// With the IP dimension disabled, two different identities behind the same
// broken address must be limited independently, not lumped together.
func TestStationRateLimitSkipsEmptyIP(t *testing.T) {
	rdb := testredis.New(t)
	h := &Handler{Lim: auth.Limiter{R: rdb}}
	req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
	req.RemoteAddr = "not-an-address" // unparseable -> clientIP returns ""

	if h.clientIP(req) != "" {
		t.Fatalf("clientIP(%q) = %q, want empty", req.RemoteAddr, h.clientIP(req))
	}

	const limit = 2
	for i := 1; i <= limit; i++ {
		if !h.allow(req, "test-empty-ip", "identity-1", limit, 1, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	if h.allow(req, "test-empty-ip", "identity-1", limit, 1, time.Hour) {
		t.Fatal("identity-1's own limit should have denied this call")
	}

	// identity-2 must be unaffected: if the empty IP were used as a literal
	// key, both identities would share one exhausted bucket by now.
	if !h.allow(req, "test-empty-ip", "identity-2", limit, 1, time.Hour) {
		t.Fatal("identity-2 was denied by identity-1's exhausted bucket")
	}
}

// TestStationClientIP covers the three shapes RemoteAddr can take. h is a zero
// value, so h.ClientIPs is a zero-value auth.ClientIPResolver (no secret),
// which always falls back to the TCP peer address -- exactly what
// (*Handler).clientIP is documented to reduce to a bare host, or "" when
// that address is not parseable.
func TestStationClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"host and port", "203.0.113.7:54321", "203.0.113.7"},
		{"ipv6 bracketed host and port", "[::1]:8080", "::1"},
		{"unparseable", "not-an-address", ""},
	}
	h := &Handler{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if got := h.clientIP(req); got != tc.want {
				t.Fatalf("clientIP(%q) = %q, want %q", tc.remoteAddr, got, tc.want)
			}
		})
	}
}
