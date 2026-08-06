package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/embedcfg"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/proxy"
)

// fastEnrollRetry and fastTokenRetry mirror the shape of the production
// schedules but with millisecond backoff, so these tests exercise the same
// retry-then-succeed logic without sleeping through the real multi-second /
// multi-minute production bounds.
var (
	fastEnrollRetry = enrollSchedule{attempts: 4, initial: time.Millisecond, max: 4 * time.Millisecond}
	fastTokenRetry  = tokenSchedule{initial: time.Millisecond, max: 4 * time.Millisecond, steady: 5 * time.Millisecond}
)

// TestStationURLUsesARealFrontendLocale guards against a repeat of the
// review finding where the kiosk opened "/uz/station" — a locale the
// frontend does not define (it only serves uz-Latn, uz-Cyrl and ru) — and
// landed every classroom PC on a 404. The launched path must always carry a
// locale from stationLocales.
func TestStationURLUsesARealFrontendLocale(t *testing.T) {
	for _, locale := range stationLocales {
		got := stationURL("127.0.0.1:17817", locale)
		want := "http://127.0.0.1:17817/" + locale + "/station"
		if got != want {
			t.Fatalf("stationURL(%q) = %q, want %q", locale, got, want)
		}
		if !validLocale(locale) {
			t.Fatalf("validLocale(%q) = false, want true", locale)
		}
	}

	if validLocale("uz") {
		t.Fatal(`validLocale("uz") = true, want false: "uz" is not a real frontend locale`)
	}
}

// TestEmbeddedConfigBeatsFlagDefaults proves a downloaded installer needs no
// arguments: the appended config supplies the code and the URLs, and a flag
// left at its compiled-in default must not silently win over it.
func TestEmbeddedConfigBeatsFlagDefaults(t *testing.T) {
	embedded := embedcfg.Config{
		Code: "AVTO-K7M2-P9XQ", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Org: "avto", Locale: "ru",
	}
	// Flag values are the compiled-in defaults and none was passed.
	got := resolveConfig(embedded, "", "https://drivergo.uz", "https://drivergo.uz", "uz-Latn",
		false, false, false)

	if got.Code != "AVTO-K7M2-P9XQ" {
		t.Fatalf("Code=%q, want the embedded code", got.Code)
	}
	if got.Locale != "ru" {
		t.Fatalf("Locale=%q, want the embedded ru over the uz-Latn default", got.Locale)
	}
	if got.Org != "avto" {
		t.Fatalf("Org=%q", got.Org)
	}
}

// TestFlagsWinWhenNothingIsEmbedded keeps the manual install path working for a
// plain unconfigured build.
func TestFlagsWinWhenNothingIsEmbedded(t *testing.T) {
	got := resolveConfig(embedcfg.Config{}, "AVTO-TYPED-BYHAND",
		"https://drivergo.uz", "https://drivergo.uz", "uz-Latn", false, false, false)

	if got.Code != "AVTO-TYPED-BYHAND" {
		t.Fatalf("Code=%q, want the flag value", got.Code)
	}
	if got.API != "https://drivergo.uz" || got.Locale != "uz-Latn" {
		t.Fatalf("got=%+v, want the flag values untouched", got)
	}
}

// TestExplicitFlagOverridesEmbedded lets an operator point one PC at staging
// without rebuilding an installer for it.
func TestExplicitFlagOverridesEmbedded(t *testing.T) {
	embedded := embedcfg.Config{
		Code: "AVTO-K7M2-P9XQ", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Locale: "uz-Latn",
	}
	got := resolveConfig(embedded, "", "https://staging.example", "https://drivergo.uz", "uz-Latn",
		true /* apiSet */, false, false)

	if got.API != "https://staging.example" {
		t.Fatalf("API=%q, want the explicitly passed flag to win", got.API)
	}
	// Everything not passed still comes from the binary.
	if got.Code != "AVTO-K7M2-P9XQ" {
		t.Fatalf("Code=%q, want the embedded code", got.Code)
	}
}

// TestEnrollWithRetrySucceedsAfterTransientFailures drives Enroll against a
// fake backend that refuses the first two attempts (simulating the
// cold-network window on a first-boot GPO rollout) and only then accepts —
// enrollWithRetry must ride that out and return success, not give up early.
func TestEnrollWithRetrySucceedsAfterTransientFailures(t *testing.T) {
	dir := t.TempDir()
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/enroll":
			if atomic.AddInt32(&attempts, 1) <= 2 {
				http.Error(w, "connection refused (simulated)", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"station_id": "s1", "org_id": "o1", "label": "PC-1"},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{APIBase: srv.URL, StateDir: dir, Keys: ks, HWID: "hwid", Version: "test"}

	if err := enrollWithRetry(context.Background(), a, "AVTO-TEST-CODE", "PC-1", fastEnrollRetry); err != nil {
		t.Fatalf("enrollWithRetry() = %v, want success after transient failures", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("enroll was attempted %d times, want 3 (2 failures + 1 success)", got)
	}
}

// TestEnrollWithRetryGivesUpAfterScheduleExhausted covers a backend that
// never answers within the schedule: enrollWithRetry must return the last
// error rather than retry forever, since a one-time code left unresolved
// deserves a fatal message to the operator, not a silent hang.
func TestEnrollWithRetryGivesUpAfterScheduleExhausted(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down (simulated)", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{APIBase: srv.URL, StateDir: dir, Keys: ks, HWID: "hwid", Version: "test"}

	if err := enrollWithRetry(context.Background(), a, "AVTO-TEST-CODE", "PC-1", fastEnrollRetry); err == nil {
		t.Fatal("enrollWithRetry() = nil, want an error once the schedule is exhausted")
	}
}

// TestKeepTokenWarmRecoversAndServesProxyTraffic is the enrolled-but-
// unreachable case from the review: a station that is already enrolled but
// whose first few token fetches fail must not exit, and once the backend
// starts answering, the proxy it is already serving must start returning
// real responses with no restart of anything.
func TestKeepTokenWarmRecoversAndServesProxyTraffic(t *testing.T) {
	dir := t.TempDir()
	const stationID = "11111111-2222-3333-4444-555555555555"
	var challenges int32

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/enroll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"station_id": stationID, "org_id": "o1", "label": "PC-1"},
			})
		case "/api/v1/b2b/stations/challenge":
			// The first three challenge calls simulate the backend being
			// unreachable at boot; only after that does it start answering.
			if atomic.AddInt32(&challenges, 1) <= 3 {
				http.Error(w, "unreachable (simulated)", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"nonce": "n1"}})
		case "/api/v1/b2b/stations/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"access_token": "tok-abc", "expires_in": 900},
			})
		case "/api/v1/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ok": true}})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(api.Close)

	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(front.Close)

	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{APIBase: api.URL, StateDir: dir, Keys: ks, HWID: "hwid", Version: "test"}
	// Enroll first so this test targets the "already enrolled" path (the
	// review finding), not the enrollment flow covered by the tests above.
	if err := a.Enroll(context.Background(), "AVTO-TEST-CODE", "PC-1"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go keepTokenWarm(ctx, a, fastTokenRetry)

	handler := proxy.New(front.URL, api.URL, a.Token)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// Immediately after start, the token is not yet live: the proxy must
	// fail closed, not error out or hang.
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(srv.URL + "/api/proxy/me")
		if err != nil {
			t.Fatal(err)
		}
		status := resp.StatusCode
		_ = resp.Body.Close()
		if status == http.StatusOK {
			break // keepTokenWarm recovered; the proxy is serving normally again.
		}
		if status != http.StatusServiceUnavailable {
			t.Fatalf("unexpected proxy status %d while waiting for recovery", status)
		}
		if time.Now().After(deadline) {
			t.Fatal("proxy never recovered after the backend started answering")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
