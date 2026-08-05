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
// HMAC assertion is valid, or, failing that, an X-Real-IP header only when
// the TCP peer that sent it is inside a trusted proxy network. Its zero
// value ignores all assertion headers and trusted proxies and falls back to
// the connection peer address.
type ClientIPResolver struct {
	secret         []byte
	now            func() time.Time
	trustedProxies []*net.IPNet
}

func NewClientIPResolver(secret []byte) ClientIPResolver {
	return ClientIPResolver{secret: append([]byte(nil), secret...), now: time.Now}
}

// WithTrustedProxies returns a copy of r that additionally trusts an
// X-Real-IP header when the request's TCP peer address falls inside one of
// the given networks (see config.Config.TrustedProxyCIDRs). This is a
// second, independent way to become asserted — the HMAC assertion path
// above is unaffected and, per ResolveAsserted, is always tried first.
func (r ClientIPResolver) WithTrustedProxies(trustedProxies []*net.IPNet) ClientIPResolver {
	r.trustedProxies = trustedProxies
	return r
}

func (r ClientIPResolver) Resolve(req *http.Request) string {
	ip, _ := r.ResolveAsserted(req)
	return ip
}

// ResolveAsserted is Resolve plus provenance: asserted is true only when the
// returned ip came from a signed, freshly-verified assertion header or from
// a trusted proxy's X-Real-IP header; false whenever it fell back to the TCP
// peer address. Callers that want to trust the IP for something stronger
// than telemetry — e.g. keying a rate-limit bucket that must not be shared
// by every caller behind the same unauthenticated proxy hop — must check
// asserted, not just look at whether the string is non-empty.
//
// The two paths are tried in order: the signed HMAC assertion first (it
// proves more — a freshly-signed statement from the BFF, not just "some
// request came from a trusted network hop"), then the trusted-proxy
// X-Real-IP header. This lets a signed assertion always win even when the
// request also happens to arrive from a trusted peer.
func (r ClientIPResolver) ResolveAsserted(req *http.Request) (ip string, asserted bool) {
	fallback := remoteIP(req)
	if ip, ok := r.resolveHMACAssertion(req); ok {
		return ip, true
	}
	if ip, ok := r.resolveTrustedProxyAssertion(req); ok {
		return ip, true
	}
	return fallback, false
}

// resolveHMACAssertion is the original, byte-for-byte-unchanged assertion
// path used by the Next.js BFF for browser traffic (auth.Handler,
// support.Handler). It returns ok=false, not a fallback value, on any
// failure so its caller (ResolveAsserted) can try the trusted-proxy path
// next before giving up.
func (r ClientIPResolver) resolveHMACAssertion(req *http.Request) (string, bool) {
	if len(r.secret) == 0 {
		return "", false
	}

	assertedIP := strings.TrimSpace(req.Header.Get(clientIPHeader))
	timestampText := strings.TrimSpace(req.Header.Get(clientIPTimestampHeader))
	signatureText := strings.TrimSpace(req.Header.Get(clientIPSignatureHeader))
	if assertedIP == "" || timestampText == "" || signatureText == "" || net.ParseIP(assertedIP) == nil {
		return "", false
	}

	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		return "", false
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	assertedAt := time.Unix(timestamp, 0)
	if now.Sub(assertedAt) > clientIPAssertionMaxAge || assertedAt.Sub(now) > clientIPAssertionFuture {
		return "", false
	}

	provided, err := base64.RawURLEncoding.DecodeString(signatureText)
	if err != nil {
		return "", false
	}
	expected := hmac.New(sha256.New, r.secret)
	_, _ = expected.Write([]byte(clientIPSigningPayload(timestampText, assertedIP, req.Method, req.URL.EscapedPath())))
	if !hmac.Equal(provided, expected.Sum(nil)) {
		return "", false
	}

	return assertedIP, true
}

// resolveTrustedProxyAssertion trusts X-Real-IP only when the TCP peer that
// sent this request is inside one of r.trustedProxies. That check is what
// makes this safe: an X-Real-IP set by an arbitrary internet client is
// indistinguishable, byte for byte, from one set by nginx, so the header
// alone proves nothing. The peer address is what closes that gap — see
// config.Config.TrustedProxyCIDRs for the deployment-specific argument for
// why a loopback peer provably came through nginx, which overwrites
// X-Real-IP with $remote_addr on every location that reaches this service.
func (r ClientIPResolver) resolveTrustedProxyAssertion(req *http.Request) (string, bool) {
	if len(r.trustedProxies) == 0 {
		return "", false
	}
	peerIP := net.ParseIP(remoteIP(req))
	if peerIP == nil {
		return "", false
	}
	trusted := false
	for _, network := range r.trustedProxies {
		if network.Contains(peerIP) {
			trusted = true
			break
		}
	}
	if !trusted {
		return "", false
	}
	realIP := strings.TrimSpace(req.Header.Get("X-Real-IP"))
	parsed := net.ParseIP(realIP)
	if parsed == nil {
		return "", false
	}
	return parsed.String(), true
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
