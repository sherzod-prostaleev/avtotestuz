package arena

import (
	"net/http"
	"testing"
)

func TestWSURLUsesPublicOriginWhenHostIsInternal(t *testing.T) {
	h := &Handler{PublicURL: "https://drivergo.uz"}
	req, err := http.NewRequest(http.MethodPost, "http://api:8080/api/v1/arena/ws-ticket", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := h.wsURL(req)
	want := "wss://drivergo.uz/api/v1/arena/ws"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWSURLKeepsDirectPublicHost(t *testing.T) {
	h := &Handler{PublicURL: "https://drivergo.uz"}
	req, err := http.NewRequest(http.MethodPost, "https://drivergo.uz/api/v1/arena/ws-ticket", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	got := h.wsURL(req)
	want := "wss://drivergo.uz/api/v1/arena/ws"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWSURLLocalhostDirectAPI(t *testing.T) {
	h := &Handler{PublicURL: "http://localhost:3000"}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/arena/ws-ticket", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := h.wsURL(req)
	want := "ws://localhost:8080/api/v1/arena/ws"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestInternalAPIHost(t *testing.T) {
	cases := map[string]bool{
		"api:8080":       true,
		"api":            true,
		"backend":        true,
		"localhost:8080": false,
		"127.0.0.1:8080": false,
		"drivergo.uz":    false,
	}
	for host, want := range cases {
		if got := internalAPIHost(host); got != want {
			t.Fatalf("%q: got %v want %v", host, got, want)
		}
	}
}

func TestOriginPatternsIncludesWWW(t *testing.T) {
	got := originPatterns("https://drivergo.uz")
	if len(got) < 2 || got[0] != "drivergo.uz" || got[1] != "www.drivergo.uz" {
		t.Fatalf("got %#v", got)
	}
}
