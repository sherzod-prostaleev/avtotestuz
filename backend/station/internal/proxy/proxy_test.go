package proxy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"avtotest.uz/station/internal/proxy"
)

func TestProxyInjectsStationTokenOnAPICalls(t *testing.T) {
	var gotAuth, gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"data":{"ok":true}}`)
	}))
	t.Cleanup(api.Close)

	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "PAGE "+r.URL.Path)
	}))
	t.Cleanup(front.Close)

	h := proxy.New(front.URL, api.URL, func(context.Context) (string, error) {
		return "station-token-123", nil
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	// The browser keeps calling the frontend's proxy path; the agent rewrites
	// it onto the API and attaches the station token.
	resp, err := http.Get(srv.URL + "/api/proxy/me")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if gotAuth != "Bearer station-token-123" {
		t.Fatalf("Authorization=%q, want the station token", gotAuth)
	}
	if gotPath != "/api/v1/me" {
		t.Fatalf("upstream path=%q, want /api/v1/me", gotPath)
	}

	// Everything else is the Next.js app, untouched.
	resp2, err := http.Get(srv.URL + "/uz/station")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	body, _ := io.ReadAll(resp2.Body)
	if string(body) != "PAGE /uz/station" {
		t.Fatalf("body=%q, want the frontend page", body)
	}
}

// Both upstreams sit behind Cloudflare, which routes on the Host header and
// answers 403 when it names a host outside the zone. httputil's default
// director rewrites URL.Scheme and URL.Host but leaves Request.Host alone, so
// without an explicit rewrite every proxied request arrives at the edge
// claiming Host: 127.0.0.1:<agent port> and the classroom sees a Cloudflare
// 403 instead of the kiosk.
func TestProxyRewritesHostForBothUpstreams(t *testing.T) {
	var apiHost string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiHost = r.Host
		_, _ = io.WriteString(w, `{"data":{"ok":true}}`)
	}))
	t.Cleanup(api.Close)

	var frontHost string
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		frontHost = r.Host
		_, _ = io.WriteString(w, "PAGE")
	}))
	t.Cleanup(front.Close)

	h := proxy.New(front.URL, api.URL, func(context.Context) (string, error) {
		return "station-token-123", nil
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/api/proxy/me")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	resp2, err := http.Get(srv.URL + "/uz-Latn/station")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()

	wantAPI := mustHost(t, api.URL)
	if apiHost != wantAPI {
		t.Fatalf("api upstream saw Host=%q, want %q", apiHost, wantAPI)
	}
	wantFront := mustHost(t, front.URL)
	if frontHost != wantFront {
		t.Fatalf("frontend upstream saw Host=%q, want %q -- Cloudflare answers 403 on a mismatched Host", frontHost, wantFront)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestProxyFailsClosedWithoutToken(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(api.Close)

	h := proxy.New("http://127.0.0.1:1", api.URL, func(context.Context) (string, error) {
		return "", context.DeadlineExceeded
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/api/proxy/me")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503 when no station token is available", resp.StatusCode)
	}
}
