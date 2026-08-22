package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestTrackIdleSurvivesA32BitBuild is a regression test for the bug that took a
// classroom down in total silence.
//
// agentRuntime.lastCall was a plain int64 read through atomic.StoreInt64. On
// 386 -- which every classroom PC runs, because one 386 binary covers 32- and
// 64-bit Windows alike -- the compiler aligns an int64 struct field to 4 bytes
// while a 64-bit atomic requires 8. Sitting behind a sync.RWMutex and a
// pointer, the field landed at offset 28 and the first store panicked with
// "unaligned 64-bit atomic operation". On amd64 it sat at offset 32, so every
// test and every CI job passed.
//
// The shipped binary is linked -H windowsgui, so the panic went to an invalid
// stderr handle and vanished: no window, no browser, no log line after the
// startup banner. A school ran the installer seven times and saw nothing at
// all.
//
// This test only has teeth when the suite is also run with GOARCH=386, which
// is why `make station-check` does exactly that.
func TestTrackIdleSurvivesA32BitBuild(t *testing.T) {
	rt := &agentRuntime{}

	// Panicked here on 386 before the fix, at construction time -- inside
	// main(), between binding the listener and logging "serving".
	h := rt.trackIdle(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// And again on the request path.
	req := httptest.NewRequest(http.MethodGet, "/api/proxy/me", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got := rt.idleFor(); got < 0 || got > time.Minute {
		t.Fatalf("idleFor() = %v, want a small positive duration", got)
	}
}

// TestIdleOnlyCountsProxiedAPICalls keeps the updater's "is a student
// mid-exam?" check honest. The browser fetches static assets on its own
// schedule; counting those would make an empty classroom look busy forever and
// the agent would never take an update.
func TestIdleOnlyCountsProxiedAPICalls(t *testing.T) {
	rt := &agentRuntime{}
	h := rt.trackIdle(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	// Backdate so a non-API request that wrongly counted would be obvious.
	rt.lastCall.Store(time.Now().Add(-time.Hour).UnixNano())

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/_next/static/chunk.js", nil))
	if rt.idleFor() < 30*time.Minute {
		t.Fatal("a static asset reset the idle clock; an empty classroom would never look idle")
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/proxy/me", nil))
	if rt.idleFor() > time.Minute {
		t.Fatal("a proxied API call did not reset the idle clock")
	}
}
