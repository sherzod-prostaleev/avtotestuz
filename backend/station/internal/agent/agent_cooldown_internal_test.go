package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"avtotest.uz/station/internal/keystore"
)

// These live inside the package so the cooldown can be expired by hand. The
// alternative -- sleeping out tokenFailTTL, or exposing a setter that only
// tests would ever call -- either slows the suite down or puts test scaffolding
// in the agent's public surface.

func cooldownAgent(t *testing.T, baseURL string) *Agent {
	t.Helper()
	dir := t.TempDir()
	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &Agent{
		APIBase:  baseURL,
		StateDir: dir,
		Keys:     ks,
		HWID:     "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900",
		Version:  "test",
	}
	// Stand the identity up directly: enrollment is covered elsewhere and is
	// not what these tests are about.
	if _, err := ks.Load(); err != nil {
		t.Fatal(err)
	}
	a.state = State{StationID: "11111111-2222-3333-4444-555555555555", Label: "PC-1"}
	a.loaded = true
	if err := a.saveState(); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestTokenRetriesOnceTheCooldownExpires is the other half of the negative
// cache: it must suppress a storm without becoming a storm-shaped outage. The
// 43 stations reactivated after the 2026-08-26 incident come back with nobody
// touching them only because the agent returns to a server that has started
// working again.
func TestTokenRetriesOnceTheCooldownExpires(t *testing.T) {
	var challenges int64
	var failing atomic.Bool
	failing.Store(true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/challenge":
			atomic.AddInt64(&challenges, 1)
			if failing.Load() {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"nonce": "test-nonce", "expires_in": 60},
			})
		case "/api/v1/b2b/stations/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"access_token": "tok-abc", "expires_in": 900},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	a := cooldownAgent(t, srv.URL)
	ctx := context.Background()

	if _, err := a.Token(ctx); err == nil {
		t.Fatal("token succeeded against a 429-only server")
	}
	if got := atomic.LoadInt64(&challenges); got != 1 {
		t.Fatalf("challenges=%d, want 1", got)
	}
	// Still inside the cooldown: served from cache, no traffic.
	if _, err := a.Token(ctx); err == nil {
		t.Fatal("token succeeded from cache")
	}
	if got := atomic.LoadInt64(&challenges); got != 1 {
		t.Fatalf("challenges=%d after a cached call, want still 1", got)
	}

	// Cooldown over, server healthy again.
	a.mu.Lock()
	a.tokenErrTill = time.Now().Add(-time.Second)
	a.mu.Unlock()
	failing.Store(false)

	tok, err := a.Token(ctx)
	if err != nil {
		t.Fatalf("token still failing after the cooldown expired: %v", err)
	}
	if tok != "tok-abc" {
		t.Fatalf("token=%q, want tok-abc", tok)
	}
	if got := atomic.LoadInt64(&challenges); got != 2 {
		t.Fatalf("challenges=%d, want 2: one refused, one that got through", got)
	}

	// Success clears the cached failure, so the next blip is judged fresh
	// rather than answered with a stale error.
	a.mu.Lock()
	stale := a.tokenErr
	a.mu.Unlock()
	if stale != nil {
		t.Fatalf("tokenErr=%v after a success, want nil", stale)
	}
}

// TestThrottlingCoolsDownLongerThanAnOrdinaryFailure pins the two-speed
// behaviour. A few seconds of network coming up must not cost a classroom a
// minute, and a 429 -- the server saying outright that the caller is going too
// fast -- must not be retried on the same few-second cadence that caused it.
func TestThrottlingCoolsDownLongerThanAnOrdinaryFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want time.Duration
	}{
		{"nginx 429 with no envelope", &APIError{Path: "/token", Status: http.StatusTooManyRequests}, tokenThrottledTTL},
		{"api envelope rate_limited", &APIError{Path: "/token", Status: http.StatusTooManyRequests, Code: "rate_limited"}, tokenThrottledTTL},
		{"backend down", &APIError{Path: "/token", Status: http.StatusBadGateway}, tokenFailTTL},
		{"station revoked", &APIError{Path: "/token", Status: http.StatusUnauthorized, Code: "station_unauthorized"}, tokenFailTTL},
		{"network refused", errors.New("dial tcp: connection refused"), tokenFailTTL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tokenFailCooldown(tc.err); got != tc.want {
				t.Fatalf("cooldown=%v, want %v", got, tc.want)
			}
		})
	}
	if tokenThrottledTTL <= tokenFailTTL {
		t.Fatalf("throttled cooldown %v must exceed the ordinary one %v", tokenThrottledTTL, tokenFailTTL)
	}
}
