package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
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

// ProfileStatusReader looks up learner profile status for RejectBanned.
type ProfileStatusReader interface {
	GetProfileByID(ctx context.Context, id uuid.UUID) (sqlc.Profile, error)
}

// RejectBanned must run after Required. Banned profiles get 403 so admin
// block takes effect before the access token naturally expires.
func RejectBanned(q ProfileStatusReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := FromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			profile, err := q.GetProfileByID(r.Context(), claims.ProfileID)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}
			if profile.Status == profileStatusBanned {
				httpx.Error(w, http.StatusForbidden, "account_blocked", "account is blocked")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
