package b2b_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	_, body = post(t, "/api/v1/b2b/stations/challenge", map[string]any{"station_id": stationID})
	data, _ = body["data"].(map[string]any)
	nonce2, _ := data["nonce"].(string)
	ts2 := time.Now().Unix()
	sig2 := ed25519.Sign(priv, b2b.SignedMessage(sid, nonce2, ts2))
	resp, _ = post(t, "/api/v1/b2b/stations/token", map[string]any{
		"station_id": stationID, "nonce": nonce2, "ts": ts2,
		"sig":       base64.StdEncoding.EncodeToString(sig2),
		"hwid_hash": "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("hwid mismatch status=%d, want 401", resp.StatusCode)
	}
}
