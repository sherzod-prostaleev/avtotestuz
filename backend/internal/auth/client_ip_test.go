package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"avtotest.uz/backend/internal/redisx"
)

var clientIPTestSecret = []byte("otp-client-ip-test-secret-32-bytes!")

func signedClientIPRequest(t *testing.T, now time.Time, assertedIP string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://backend.test/api/v1/auth/otp/request", nil)
	req.RemoteAddr = "10.0.0.8:43120"
	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, clientIPTestSecret)
	_, _ = mac.Write([]byte(clientIPSigningPayload(timestamp, assertedIP, req.Method, req.URL.EscapedPath())))
	req.Header.Set(clientIPHeader, assertedIP)
	req.Header.Set(clientIPTimestampHeader, timestamp)
	req.Header.Set(clientIPSignatureHeader, base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	return req
}

func TestClientIPResolverAcceptsOnlyFreshAuthenticAssertions(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	resolver := NewClientIPResolver(clientIPTestSecret)
	resolver.now = func() time.Time { return now }

	t.Run("valid", func(t *testing.T) {
		req := signedClientIPRequest(t, now, "198.51.100.10")
		if got := resolver.Resolve(req); got != "198.51.100.10" {
			t.Fatalf("Resolve = %q, want signed client IP", got)
		}
	})

	tests := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{
			name: "unsigned",
			mutate: func(req *http.Request) {
				req.Header.Del(clientIPTimestampHeader)
				req.Header.Del(clientIPSignatureHeader)
			},
		},
		{
			name: "tampered IP",
			mutate: func(req *http.Request) {
				req.Header.Set(clientIPHeader, "198.51.100.99")
			},
		},
		{
			name: "tampered signature",
			mutate: func(req *http.Request) {
				req.Header.Set(clientIPSignatureHeader, "ZmFrZQ")
			},
		},
		{
			name: "expired",
			mutate: func(req *http.Request) {
				stale := now.Add(-clientIPAssertionMaxAge - time.Second)
				staleReq := signedClientIPRequest(t, stale, "198.51.100.10")
				req.Header = staleReq.Header
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := signedClientIPRequest(t, now, "198.51.100.10")
			tt.mutate(req)
			if got := resolver.Resolve(req); got != "10.0.0.8" {
				t.Fatalf("Resolve = %q, want safe RemoteAddr fallback", got)
			}
		})
	}
}

func TestClientIPResolverIgnoresAssertionsWhenNoSharedSecret(t *testing.T) {
	req := signedClientIPRequest(t, time.Now(), "198.51.100.10")
	if got := (ClientIPResolver{}).Resolve(req); got != "10.0.0.8" {
		t.Fatalf("Resolve = %q, want RemoteAddr fallback", got)
	}
}

func TestDistinctSignedClientIPsUseDistinctRateLimitBuckets(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	resolver := NewClientIPResolver(clientIPTestSecret)
	resolver.now = func() time.Time { return now }
	limiter := Limiter{R: redisx.NewTest(t)}
	ctx := context.Background()

	first := resolver.Resolve(signedClientIPRequest(t, now, "198.51.100.10"))
	second := resolver.Resolve(signedClientIPRequest(t, now, "198.51.100.11"))
	if first == second {
		t.Fatalf("resolved IPs unexpectedly share value %q", first)
	}

	for i := 0; i < 20; i++ {
		ok, err := limiter.Allow(ctx, "otp:ip:"+first, 20, time.Hour)
		if err != nil || !ok {
			t.Fatalf("first bucket request %d: ok=%v err=%v", i+1, ok, err)
		}
	}
	ok, err := limiter.Allow(ctx, "otp:ip:"+first, 20, time.Hour)
	if err != nil || ok {
		t.Fatalf("first bucket request 21: ok=%v err=%v, want denied", ok, err)
	}
	ok, err = limiter.Allow(ctx, "otp:ip:"+second, 20, time.Hour)
	if err != nil || !ok {
		t.Fatalf("second client must have an independent bucket: ok=%v err=%v", ok, err)
	}
}
