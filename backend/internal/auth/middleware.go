package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/stationctx"
)

type ctxKey int

const (
	claimsKey ctxKey = iota
	profileKey
)

// FromContext retrieves the Claims stored by Required.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

// ProfileFromContext retrieves the sqlc.Profile loaded by RejectBanned.
func ProfileFromContext(ctx context.Context) (sqlc.Profile, bool) {
	p, ok := ctx.Value(profileKey).(sqlc.Profile)
	return p, ok
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
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			ctx = stationctx.WithContext(ctx, claims.StationID)
			next.ServeHTTP(w, r.WithContext(ctx))
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
				if errors.Is(err, pgx.ErrNoRows) {
					httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
					return
				}
				httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "service temporarily unavailable")
				return
			}
			if profile.Status == profileStatusBanned {
				httpx.Error(w, http.StatusForbidden, "account_blocked", "account is blocked")
				return
			}
			ctx := context.WithValue(r.Context(), profileKey, profile)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
