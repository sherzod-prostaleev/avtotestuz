package proxy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
