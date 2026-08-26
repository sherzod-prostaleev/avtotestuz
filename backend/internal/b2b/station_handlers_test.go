package b2b_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
	"avtotest.uz/backend/internal/testredis"
)

func TestStationEndpointsEndToEnd(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	ctx := context.Background()
	secret := []byte("test-secret-that-is-long-enough-000000")

	store := b2b.Store{Pool: pool}
	orgID := seatedOrg(t, pool, 2)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	h := &b2b.Handler{Pool: pool, Redis: rdb, Secret: secret, Lim: auth.Limiter{R: rdb}}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	post := func(t *testing.T, path string, body any) (*http.Response, map[string]any) {
		t.Helper()
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"

	resp, body := post(t, "/api/v1/b2b/stations/enroll", map[string]any{
		"code":          code.Code,
		"public_key":    base64.StdEncoding.EncodeToString(pub),
		"hwid_hash":     hwid,
		"label":         "PC-1",
		"agent_version": "1.0.0",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enroll status=%d body=%v", resp.StatusCode, body)
	}
	data, _ := body["data"].(map[string]any)
	stationID, _ := data["station_id"].(string)
	if stationID == "" {
		t.Fatalf("no station_id in %v", body)
	}

	resp, body = post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("challenge status=%d body=%v", resp.StatusCode, body)
	}
	data, _ = body["data"].(map[string]any)
	nonce, _ := data["nonce"].(string)
	if nonce == "" {
		t.Fatalf("no nonce in %v", body)
	}

	sid, err := uuid.Parse(stationID)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Unix()
	sig := ed25519.Sign(priv, b2b.SignedMessage(sid, nonce, ts))

	resp, body = post(t, "/api/v1/b2b/stations/token", map[string]any{
		"station_id": stationID,
		"nonce":      nonce,
		"ts":         ts,
		"sig":        base64.StdEncoding.EncodeToString(sig),
		"hwid_hash":  hwid,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token status=%d body=%v", resp.StatusCode, body)
	}
	data, _ = body["data"].(map[string]any)
	accessToken, _ := data["access_token"].(string)
	claims, err := auth.ParseAccess(secret, accessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.StationID != sid {
		t.Fatalf("claims.StationID=%v, want %v", claims.StationID, sid)
	}

	// A wrong hwid on an otherwise valid flow is a flat 401.
	resp, body = post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("challenge (hwid case) status=%d body=%v", resp.StatusCode, body)
	}
	data, _ = body["data"].(map[string]any)
	nonce2, _ := data["nonce"].(string)
	if nonce2 == "" {
		t.Fatalf("no nonce in %v", body)
	}
	ts2 := time.Now().Unix()
	sig2 := ed25519.Sign(priv, b2b.SignedMessage(sid, nonce2, ts2))
	resp, hwidBody := post(t, "/api/v1/b2b/stations/token", map[string]any{
		"station_id": stationID, "nonce": nonce2, "ts": ts2,
		"sig":       base64.StdEncoding.EncodeToString(sig2),
		"hwid_hash": "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("hwid mismatch status=%d, want 401", resp.StatusCode)
	}

	// The property being defended is that every station-auth failure reason
	// is indistinguishable to the caller, not just that they all return 401.
	// A corrupted signature on a fresh challenge must produce the exact same
	// error.code and error.message as the hwid mismatch above -- a
	// status-only check on one reason cannot catch a regression that leaks
	// which reason it was through the body.
	resp, body = post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("challenge (sig case) status=%d body=%v", resp.StatusCode, body)
	}
	data, _ = body["data"].(map[string]any)
	nonce3, _ := data["nonce"].(string)
	if nonce3 == "" {
		t.Fatalf("no nonce in %v", body)
	}
	ts3 := time.Now().Unix()
	sig3 := ed25519.Sign(priv, b2b.SignedMessage(sid, nonce3, ts3))
	sig3[0] ^= 0xFF // corrupt the signature; hwid stays correct
	resp, sigBody := post(t, "/api/v1/b2b/stations/token", map[string]any{
		"station_id": stationID, "nonce": nonce3, "ts": ts3,
		"sig":       base64.StdEncoding.EncodeToString(sig3),
		"hwid_hash": hwid,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("corrupted signature status=%d, want 401", resp.StatusCode)
	}

	hwidErr, _ := hwidBody["error"].(map[string]any)
	sigErr, _ := sigBody["error"].(map[string]any)
	if hwidErr["code"] == nil || sigErr["code"] == nil {
		t.Fatalf("missing error body: hwid=%v sig=%v", hwidBody, sigBody)
	}
	if hwidErr["code"] != sigErr["code"] || hwidErr["message"] != sigErr["message"] {
		t.Fatalf("station-auth failure reasons are distinguishable: hwid=%v sig=%v", hwidErr, sigErr)
	}
}

// TestEnrollRateLimitAllowsAFullLicenceRollout is the regression test for the
// incident this fixes: an installer key is idempotent and licence-lived (see
// OpenInstallerKey), so every PC in a school's rollout hits /enroll with the
// exact same code -- and the enroll rate limit is keyed on that code, not on
// the PC. Before this fix enrollIdentityLimit was 20, a value left over from
// when a code was a disposable 2-hour window a teacher could reopen at will.
// A 30-seat school would enrol PCs 1-20, then PC 21 would get 429, the agent
// would burn its retries and log.Fatalf, and there was no way to recover
// short of waiting an hour or rotating the key -- which kills the copies
// already handed to the rest of the classroom. Driven through the real HTTP
// handler, with the real (unmocked) constant, because that is where the
// limiter actually lives; the store has no rate limiting of its own.
func TestEnrollRateLimitAllowsAFullLicenceRollout(t *testing.T) {
	const seats = 30 // more than the old enrollIdentityLimit of 20

	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	ctx := context.Background()
	secret := []byte("test-secret-that-is-long-enough-000000")

	store := b2b.Store{Pool: pool}
	orgID := seatedOrg(t, pool, seats)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	if code.MaxUses < seats {
		t.Fatalf("max_uses=%d, want at least %d so the code cannot be what stops the %d-PC rollout", code.MaxUses, seats, seats)
	}

	h := &b2b.Handler{Pool: pool, Redis: rdb, Secret: secret, Lim: auth.Limiter{R: rdb}}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	for i := 0; i < seats; i++ {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(map[string]any{
			"code":          code.Code,
			"public_key":    base64.StdEncoding.EncodeToString(pub),
			"hwid_hash":     testHWID(fmt.Sprintf("rate-limit-rollout-%d", i)),
			"label":         fmt.Sprintf("PC-%d", i+1),
			"agent_version": "1.0.0",
		})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(srv.URL+"/api/v1/b2b/stations/enroll", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PC %d/%d: status=%d body=%s (a same-code rollout must not trip the enroll rate limit before every seat is used)",
				i+1, seats, resp.StatusCode, respBody)
		}
	}
}
