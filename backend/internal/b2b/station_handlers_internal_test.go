package b2b

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testredis"
)

// TestStationRateLimitIdentityDimension drives (*Handler).allow directly with a
// low limit instead of sending hundreds of HTTP requests through a handler
// that hardcodes production limits. allow's boolean return is exactly what
// each station handler turns into 429: the moment it flips false,
// stationChallenge/stationToken/enrollStation all write
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

// signedClientIPRequest builds a request carrying a valid signed
// trusted-proxy IP assertion, mirroring what nginx is expected to send once
// configured (frontend/src/lib/client-ip-assertion.ts is the producer side
// of this same header contract). auth.ClientIPResolver is package-private
// about its fields, so a white-box test in package auth can call the
// unexported signing helper directly (see client_ip_test.go); from package
// b2b the header names and payload format have to be reproduced here to
// build a request that ResolveAsserted will actually verify.
func signedClientIPRequest(t *testing.T, secret []byte, method, path, ip string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := strings.Join([]string{"v1", ts, ip, method, req.URL.EscapedPath()}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	req.Header.Set("X-Avtotest-Client-IP", ip)
	req.Header.Set("X-Avtotest-Client-IP-Timestamp", ts)
	req.Header.Set("X-Avtotest-Client-IP-Signature", base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	return req
}

// TestStationRateLimitIPDimension exercises the secondary IP dimension the
// same way, proving it still applies (finding 1 says close the loophole,
// not delete the dimension) once h.ClientIPs actually verifies a signed
// assertion -- an unasserted RemoteAddr alone must not be enough, which is
// exactly finding 1's bug.
func TestStationRateLimitIPDimension(t *testing.T) {
	rdb := testredis.New(t)
	secret := []byte("station-ip-dimension-test-secret-32-bytes!")
	h := &Handler{Lim: auth.Limiter{R: rdb}, ClientIPs: auth.NewClientIPResolver(secret)}

	const limit = 2
	for i := 1; i <= limit; i++ {
		req := signedClientIPRequest(t, secret, http.MethodPost, "/b2b/stations/challenge", "198.51.100.9")
		// A fresh identity per call so only the IP dimension can deny.
		identity := fmt.Sprintf("station-%d", i)
		if !h.allow(req, "test-exhaust-ip", identity, 1000, limit, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	req := signedClientIPRequest(t, secret, http.MethodPost, "/b2b/stations/challenge", "198.51.100.9")
	if h.allow(req, "test-exhaust-ip", "station-final", 1000, limit, time.Hour) {
		t.Fatalf("call %d/%d: want denied (429) by the IP dimension, got allowed", limit+1, limit)
	}
}

// TestStationRateLimitUnassertedIPNeverAppliesIPDimension covers finding 1
// directly: a RemoteAddr alone (no signed assertion), which is exactly what
// nginx sends today over its loopback hop to this service, must never key
// the IP bucket -- otherwise every station on the platform shares one
// bucket keyed on the proxy's own address. Two different identities behind
// the same unasserted RemoteAddr must be limited independently.
func TestStationRateLimitUnassertedIPNeverAppliesIPDimension(t *testing.T) {
	rdb := testredis.New(t)
	h := &Handler{Lim: auth.Limiter{R: rdb}} // zero-value ClientIPs: never asserts
	req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
	req.RemoteAddr = "198.51.100.9:54321"

	const limit = 2
	for i := 1; i <= limit; i++ {
		if !h.allow(req, "test-unasserted-ip", "identity-1", limit, 1, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	if h.allow(req, "test-unasserted-ip", "identity-1", limit, 1, time.Hour) {
		t.Fatal("identity-1's own limit should have denied this call")
	}

	// identity-2 must be unaffected: if the unasserted RemoteAddr were used
	// as the IP-bucket key (ipLimit=1 above), both identities would share
	// one exhausted bucket by now.
	if !h.allow(req, "test-unasserted-ip", "identity-2", limit, 1, time.Hour) {
		t.Fatal("identity-2 was denied by identity-1's exhausted bucket")
	}
}

// TestStationRateLimitFailsClosedWithNoDimension covers finding 2: empty
// identity and an unasserted IP together must not make allow return true
// having applied no limit at all. Reachable via e.g. enrollStation on
// {"code":""}.
func TestStationRateLimitFailsClosedWithNoDimension(t *testing.T) {
	rdb := testredis.New(t)
	h := &Handler{Lim: auth.Limiter{R: rdb}}
	req := httptest.NewRequest(http.MethodPost, "/b2b/stations/enroll", nil)
	req.RemoteAddr = "198.51.100.20:54321" // present but never asserted

	const limit = 2
	for i := 1; i <= limit; i++ {
		if !h.allow(req, "test-unkeyed", "", limit, limit, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	if h.allow(req, "test-unkeyed", "", limit, limit, time.Hour) {
		t.Fatal("empty identity + unasserted IP must still be bounded by the unkeyed fallback bucket")
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

// TestStationChallengeHandlerRateLimitsSameStation drives the real HTTP
// handler chain end to end (finding 4). Every other rate-limit test in this
// file calls (*Handler).allow directly with synthetic action strings no
// handler actually uses, so a regression that dropped the h.allow call from
// stationChallenge, or passed the wrong action/identity/limit, would go
// completely undetected. challenge needs no Postgres fixture --
// StationAuth.Challenge never touches h.Pool -- so driving it past
// challengeIdentityLimit is cheap.
func TestStationChallengeHandlerRateLimitsSameStation(t *testing.T) {
	rdb := testredis.New(t)
	h := &Handler{Redis: rdb, Secret: []byte("test-secret-that-is-long-enough-000000"), Lim: auth.Limiter{R: rdb}}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	body, err := json.Marshal(map[string]string{"station_id": uuid.New().String()})
	if err != nil {
		t.Fatal(err)
	}

	post := func(t *testing.T) int {
		t.Helper()
		resp, err := http.Post(srv.URL+"/api/v1/b2b/stations/challenge", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	for i := 1; i <= challengeIdentityLimit; i++ {
		if status := post(t); status != http.StatusOK {
			t.Fatalf("request %d/%d: status=%d, want 200", i, challengeIdentityLimit, status)
		}
	}
	if status := post(t); status != http.StatusTooManyRequests {
		t.Fatalf("request %d: status=%d, want 429", challengeIdentityLimit+1, status)
	}
}
