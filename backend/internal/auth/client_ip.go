package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	clientIPHeader           = "X-Avtotest-Client-IP"
	clientIPTimestampHeader  = "X-Avtotest-Client-IP-Timestamp"
	clientIPSignatureHeader  = "X-Avtotest-Client-IP-Signature"
	clientIPAssertionVersion = "v1"
	clientIPAssertionMaxAge  = 60 * time.Second
	clientIPAssertionFuture  = 5 * time.Second
)

// ClientIPResolver accepts a BFF-provided client IP only when its short-lived
// HMAC assertion is valid. Its zero value ignores all assertion headers and
// falls back to the connection peer address.
type ClientIPResolver struct {
	secret []byte
	now    func() time.Time
}

func NewClientIPResolver(secret []byte) ClientIPResolver {
	return ClientIPResolver{secret: append([]byte(nil), secret...), now: time.Now}
}

func (r ClientIPResolver) Resolve(req *http.Request) string {
	ip, _ := r.ResolveAsserted(req)
	return ip
}

// ResolveAsserted is Resolve plus provenance: asserted is true only when the
// returned ip came from a signed, freshly-verified assertion header, false
// whenever it fell back to the TCP peer address (no secret configured, no
// assertion headers present, or the assertion failed to verify). Callers
// that want to trust the IP for something stronger than telemetry — e.g.
// keying a rate-limit bucket that must not be shared by every caller behind
// the same unauthenticated proxy hop — must check asserted, not just look at
// whether the string is non-empty.
func (r ClientIPResolver) ResolveAsserted(req *http.Request) (ip string, asserted bool) {
	fallback := remoteIP(req)
	if len(r.secret) == 0 {
		return fallback, false
	}

	assertedIP := strings.TrimSpace(req.Header.Get(clientIPHeader))
	timestampText := strings.TrimSpace(req.Header.Get(clientIPTimestampHeader))
	signatureText := strings.TrimSpace(req.Header.Get(clientIPSignatureHeader))
	if assertedIP == "" || timestampText == "" || signatureText == "" || net.ParseIP(assertedIP) == nil {
		return fallback, false
	}

	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		return fallback, false
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	assertedAt := time.Unix(timestamp, 0)
	if now.Sub(assertedAt) > clientIPAssertionMaxAge || assertedAt.Sub(now) > clientIPAssertionFuture {
		return fallback, false
	}

	provided, err := base64.RawURLEncoding.DecodeString(signatureText)
	if err != nil {
		return fallback, false
	}
	expected := hmac.New(sha256.New, r.secret)
	_, _ = expected.Write([]byte(clientIPSigningPayload(timestampText, assertedIP, req.Method, req.URL.EscapedPath())))
	if !hmac.Equal(provided, expected.Sum(nil)) {
		return fallback, false
	}

	return assertedIP, true
}

func clientIPSigningPayload(timestamp, ip, method, path string) string {
	return strings.Join([]string{clientIPAssertionVersion, timestamp, ip, method, path}, "\n")
}

func remoteIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}
