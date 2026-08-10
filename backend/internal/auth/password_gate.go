package auth

import (
	"net/http"
	"strings"

	"avtotest.uz/backend/internal/httpx"
)

// passwordChangeAllowedPaths may be called while must_change_password is set.
// Everything else under learner auth returns password_change_required.
func passwordChangeAllowed(r *http.Request) bool {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && isExactMe(path):
		return true
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/me/password"):
		return true
	default:
		return false
	}
}

func isExactMe(path string) bool {
	return path == "/me" || path == "/api/v1/me" || strings.HasSuffix(path, "/me") && !strings.Contains(path, "/me/")
}

// RequirePasswordChanged must run after Required (+ RejectBanned). When the
// profile has must_change_password, only GET /me and POST /me/password proceed.
func RequirePasswordChanged(q ProfileStatusReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if passwordChangeAllowed(r) {
				next.ServeHTTP(w, r)
				return
			}
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
			if profile.MustChangePassword {
				httpx.Error(w, http.StatusForbidden, "password_change_required", "password change required before continuing")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
