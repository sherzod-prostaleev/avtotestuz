// Package devicefp carries a browser/device fingerprint on the request context
// so VIP gates can grant classroom station access without personal entitlement.
package devicefp

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey struct{}

// Header is the learner/admin-facing device fingerprint header.
const Header = "X-Device-Fingerprint"

// MaxLen caps fingerprint length (UUID / opaque token).
const MaxLen = 128

// FromRequest reads and normalizes the fingerprint header.
func FromRequest(r *http.Request) string {
	return Normalize(r.Header.Get(Header))
}

// Normalize trims and length-caps a fingerprint.
func Normalize(fp string) string {
	fp = strings.TrimSpace(fp)
	if len(fp) > MaxLen {
		fp = fp[:MaxLen]
	}
	return fp
}

// WithContext stores fingerprint on ctx.
func WithContext(ctx context.Context, fp string) context.Context {
	fp = Normalize(fp)
	if fp == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, fp)
}

// FromContext returns the fingerprint if present.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// Middleware injects X-Device-Fingerprint into the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fp := FromRequest(r); fp != "" {
			r = r.WithContext(WithContext(r.Context(), fp))
		}
		next.ServeHTTP(w, r)
	})
}
