package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/keystore"
)

const throttleHWID = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"

// enrolledAgent returns an agent already bound to stationID against srv, so a
// test can go straight to token behaviour without re-testing enrollment.
func enrolledAgent(t *testing.T, baseURL string) *agent.Agent {
	t.Helper()
	dir := t.TempDir()
	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{
		APIBase:  baseURL,
		StateDir: dir,
		Keys:     ks,
		HWID:     throttleHWID,
		Version:  "test",
	}
	if err := a.Enroll(context.Background(), "AVTO-TEST-CODE", "PC-1", "Test Avtomaktab"); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestTokenStopsHammeringAfterA429 is the regression for the request storm at
// Avtomotohavaskorlar BUXORO, 2026-08-26.
//
// The kiosk's local proxy asks for a token on every proxied API request, and
// the kiosk page re-probes /me for as long as it is unhappy. Because a failed
// token was not remembered, each of those went back on the wire as a fresh
// challenge+token pair: 55 PCs behind one NAT'd IP produced 7,030 rate-limited
// requests in two hours against a budget of 60 a minute, and the limiter they
// had tripped could never drain.
//
// One attempt per cooldown, no matter how many callers ask, is the property
// that makes the storm impossible.
func TestTokenStopsHammeringAfterA429(t *testing.T) {
	var challenges, tokens int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/enroll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"station_id": "11111111-2222-3333-4444-555555555555",
					"org_id":     "11111111-2222-3333-4444-555555555555",
					"label":      "PC-1",
				},
			})
		case "/api/v1/b2b/stations/challenge":
			atomic.AddInt64(&challenges, 1)
			// Exactly what nginx's limit_req writes: a 429 with an HTML body
			// and no error envelope for the agent to read a code out of.
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("<html><head><title>429 Too Many Requests</title></head></html>"))
		case "/api/v1/b2b/stations/token":
			atomic.AddInt64(&tokens, 1)
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	a := enrolledAgent(t, srv.URL)
	ctx := context.Background()

	// A kiosk page loading thirty question images, all inside the cooldown.
	const callers = 30
	var lastErr error
	for i := 0; i < callers; i++ {
		if _, err := a.Token(ctx); err == nil {
			t.Fatalf("call %d: token succeeded against a 429-only server", i+1)
		} else {
			lastErr = err
		}
	}

	if got := atomic.LoadInt64(&challenges); got != 1 {
		t.Fatalf("challenge calls=%d after %d Token() calls, want exactly 1", got, callers)
	}
	if got := atomic.LoadInt64(&tokens); got != 0 {
		t.Fatalf("token calls=%d, want 0: the challenge never succeeded", got)
	}

	// The cached error must stay the real one, or every caller that matches on
	// it (diagnose, the status store, connect.go) starts seeing something else.
	var apiErr *agent.APIError
	if !errors.As(lastErr, &apiErr) {
		t.Fatalf("err=%v (%T), want *agent.APIError", lastErr, lastErr)
	}
	if apiErr.Status != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want 429", apiErr.Status)
	}
	if apiErr.Code != "" {
		t.Fatalf("code=%q, want empty: nginx sends no envelope", apiErr.Code)
	}
}

// TestReEnrollmentDoesNotInheritTheOldStationsFailure covers the identity
// change: a cached rejection belongs to the station that earned it, and an
// operator who has just run -reenroll must not be shown it again.
func TestReEnrollmentDoesNotInheritTheOldStationsFailure(t *testing.T) {
	var refuse atomic.Bool
	refuse.Store(true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/enroll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"station_id": "11111111-2222-3333-4444-555555555555",
					"org_id":     "11111111-2222-3333-4444-555555555555",
					"label":      "PC-1",
				},
			})
		case "/api/v1/b2b/stations/challenge":
			if refuse.Load() {
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

	a := enrolledAgent(t, srv.URL)
	ctx := context.Background()

	if _, err := a.Token(ctx); err == nil {
		t.Fatal("token succeeded against a 429-only server")
	}

	// -reenroll, then a server that now answers. A 429 cooldown is 60s, far
	// longer than any test would wait, so if the reset did not clear it this
	// call would still be served the old error.
	if err := a.ResetEnrollment(); err != nil {
		t.Fatal(err)
	}
	refuse.Store(false)
	if err := a.Enroll(ctx, "AVTO-TEST-CODE", "PC-1", "Test Avtomaktab"); err != nil {
		t.Fatal(err)
	}

	tok, err := a.Token(ctx)
	if err != nil {
		t.Fatalf("re-enrolled station still served the old failure: %v", err)
	}
	if tok != "tok-abc" {
		t.Fatalf("token=%q, want tok-abc", tok)
	}
}
