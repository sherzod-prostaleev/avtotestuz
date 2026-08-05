package b2b

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
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

// TestStationRateLimitIPDimensionViaTrustedProxy proves the fix for the
// dormant-IP-dimension gap directly, end to end: the station agent talks to
// the backend directly and never goes through the Next.js BFF that signs
// HMAC assertions, so it can never produce a signedClientIPRequest -- and
// TestStationRateLimitUnassertedIPNeverAppliesIPDimension shows that a bare
// RemoteAddr, which is all nginx's loopback hop gave this service before
// Task 1, never applies the IP dimension. Here the same kind of request
// (no HMAC headers) arrives with a trusted-proxy peer address and an
// X-Real-IP header instead, exactly as nginx now sends it, and the IP
// dimension applies where before it was skipped entirely.
func TestStationRateLimitIPDimensionViaTrustedProxy(t *testing.T) {
	rdb := testredis.New(t)
	_, loopback, err := net.ParseCIDR("127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		Lim:       auth.Limiter{R: rdb},
		ClientIPs: auth.NewClientIPResolver(nil).WithTrustedProxies([]*net.IPNet{loopback}),
	}

	const limit = 2
	for i := 1; i <= limit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
		req.RemoteAddr = "127.0.0.1:54321" // nginx's loopback hop, trusted
		req.Header.Set("X-Real-IP", "198.51.100.9")
		// A fresh identity per call so only the IP dimension can deny.
		identity := fmt.Sprintf("station-%d", i)
		if !h.allow(req, "test-trusted-proxy-ip", identity, 1000, limit, time.Hour) {
			t.Fatalf("call %d/%d: want allowed, got denied", i, limit)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/b2b/stations/challenge", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Real-IP", "198.51.100.9")
	if h.allow(req, "test-trusted-proxy-ip", "station-final", 1000, limit, time.Hour) {
		t.Fatalf("call %d/%d: want denied (429) by the IP dimension via trusted-proxy X-Real-IP, got allowed", limit+1, limit)
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

// TestStationClientIP covers finding 2: clientIP must return the caller's
// address only when h.ClientIPs verified a signed assertion, never as a
// fallback to the TCP peer. h is a zero value here, so h.ClientIPs is a
// zero-value auth.ClientIPResolver (no secret) which never asserts --
// exactly what nginx sends today over its loopback hop to this service --
// so every RemoteAddr shape, however well-formed, must come back "". This
// replaces the pre-fix version of this test, which asserted the opposite
// (that a well-formed unasserted RemoteAddr resolved to its own address):
// that was the uniformly-wrong-last_ip bug, not a behavior to preserve.
func TestStationClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
	}{
		{"host and port", "203.0.113.7:54321"},
		{"ipv6 bracketed host and port", "[::1]:8080"},
		{"unparseable", "not-an-address"},
	}
	h := &Handler{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if got := h.clientIP(req); got != "" {
				t.Fatalf("clientIP(%q) = %q, want \"\" (unasserted must never resolve)", tc.remoteAddr, got)
			}
		})
	}

	// The positive path: a verified assertion is the only way to get a real
	// IP out, and it must still work.
	t.Run("verified assertion resolves", func(t *testing.T) {
		secret := []byte("station-client-ip-test-secret-32-bytes!")
		h := &Handler{ClientIPs: auth.NewClientIPResolver(secret)}
		req := signedClientIPRequest(t, secret, http.MethodPost, "/", "198.51.100.9")
		if got := h.clientIP(req); got != "198.51.100.9" {
			t.Fatalf("clientIP with verified assertion = %q, want %q", got, "198.51.100.9")
		}
	})
}

// testStationOrg inserts a minimal org with an active 2-seat license, enough
// for the enroll/challenge/token flow the failure-bucket test below drives
// through real HTTP handlers. Kept local to this file (rather than reusing
// b2b_test's seatedOrg) because this is a whitebox (package b2b) test file
// and seatedOrg lives in the external test package.
func testStationOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO b2b_org (name) VALUES ('School') RETURNING id`).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note)
		VALUES ($1, 2, now(), now() + interval '30 days', 'test')`, orgID); err != nil {
		t.Fatal(err)
	}
	return orgID
}

// testStationHWID turns a seed string into a deterministic 64-character
// lowercase sha256 hex digest, the shape EnrollInput.validate requires of
// hwid_hash (mirrors b2b_test's testHWID, unreachable from this package).
func testStationHWID(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// TestStationTokenFailureBucketClearsOnSuccess covers the residual finding
// from task 7's review: tokenFailedIdentityLimit only ever grew, so a
// station that failed a handful of times for an innocent reason -- clock
// drift, an expired nonce, a restart mid-handshake -- and then started
// succeeding again stayed penalized against the same threshold for the rest
// of the hour regardless. stationToken now resets failKey on a successful
// issue; this drives the bucket partway up, forces one success, then proves
// a full run of tokenFailedIdentityLimit fresh failures is accepted (never
// 429) afterward -- which is only possible if the earlier failures were
// actually cleared, not merely stopped from growing further.
func TestStationTokenFailureBucketClearsOnSuccess(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	ctx := context.Background()
	secret := []byte("test-secret-that-is-long-enough-000000")

	store := Store{Pool: pool}
	orgID := testStationOrg(t, pool)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{Pool: pool, Redis: rdb, Secret: secret, Lim: auth.Limiter{R: rdb}}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	post := func(t *testing.T, path string, body any) (int, map[string]any) {
		t.Helper()
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hwid := testStationHWID("hw-clear-on-success")

	status, body := post(t, "/api/v1/b2b/stations/enroll", map[string]any{
		"code":          code.Code,
		"public_key":    base64.StdEncoding.EncodeToString(pub),
		"hwid_hash":     hwid,
		"label":         "PC-1",
		"agent_version": "1.0.0",
	})
	if status != http.StatusOK {
		t.Fatalf("enroll status=%d body=%v", status, body)
	}
	data, _ := body["data"].(map[string]any)
	stationID, _ := data["station_id"].(string)
	sid, err := uuid.Parse(stationID)
	if err != nil {
		t.Fatal(err)
	}

	// attempt gets a fresh nonce and answers it -- with a bad signature if
	// bad is true, otherwise a genuine one -- and returns the token call's
	// status code.
	attempt := func(t *testing.T, bad bool) int {
		t.Helper()
		status, cbody := post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
		if status != http.StatusOK {
			t.Fatalf("challenge status=%d body=%v", status, cbody)
		}
		cdata, _ := cbody["data"].(map[string]any)
		nonce, _ := cdata["nonce"].(string)
		if nonce == "" {
			t.Fatalf("no nonce in %v", cbody)
		}
		ts := time.Now().Unix()
		sig := ed25519.Sign(priv, SignedMessage(sid, nonce, ts))
		if bad {
			sig[0] ^= 0xFF
		}
		status, _ = post(t, "/api/v1/b2b/stations/token", map[string]any{
			"station_id": stationID, "nonce": nonce, "ts": ts,
			"sig": base64.StdEncoding.EncodeToString(sig), "hwid_hash": hwid,
		})
		return status
	}

	// Drive the failure bucket partway up -- well short of the limit, so
	// this alone proves nothing about resets; it just establishes there is
	// something in the bucket for the reset to clear.
	const partial = 10
	for i := 0; i < partial; i++ {
		if status := attempt(t, true); status != http.StatusUnauthorized {
			t.Fatalf("pre-success failed attempt %d/%d: status=%d, want 401", i+1, partial, status)
		}
	}

	// One genuine success.
	if status := attempt(t, false); status != http.StatusOK {
		t.Fatalf("recovery attempt: status=%d, want 200", status)
	}

	// If the success had not cleared failKey, it would still sit at
	// `partial`, and this loop -- tokenFailedIdentityLimit more failures --
	// would trip 429 with `partial` attempts still to go. Every one of these
	// landing as 401 is only possible if the bucket actually reset to zero.
	for i := 0; i < tokenFailedIdentityLimit; i++ {
		if status := attempt(t, true); status != http.StatusUnauthorized {
			t.Fatalf("post-success failed attempt %d/%d: status=%d, want 401 (bucket did not reset)",
				i+1, tokenFailedIdentityLimit, status)
		}
	}
	if status := attempt(t, true); status != http.StatusTooManyRequests {
		t.Fatalf("attempt %d after the post-success run: status=%d, want 429 (limit not enforced)",
			tokenFailedIdentityLimit+1, status)
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
