package auth

import (
	"context"
	"net/http"
	"strings"

	"avtotest.uz/backend/internal/httpx"
)

type ctxKey int

const claimsKey ctxKey = iota

// FromContext retrieves the Claims stored by Required.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

// Required verifies a Bearer access token and stores its Claims in the
// request context; missing, malformed, or expired tokens get 401.
func Required(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			claims, err := ParseAccess(secret, token)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
		})
	}
}
