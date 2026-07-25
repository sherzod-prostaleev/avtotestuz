package admin

import (
	"context"
	"net/http"
	"strings"

	"avtotest.uz/backend/internal/httpx"
)

type ctxKey int

const (
	claimsKey ctxKey = iota
	permsKey
)

// FromContext returns admin Claims stored by Required.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

// PermsFromContext returns the permission set loaded by Required.
func PermsFromContext(ctx context.Context) []string {
	p, _ := ctx.Value(permsKey).([]string)
	return p
}

func hasPerm(perms []string, want string) bool {
	for _, p := range perms {
		if p == want {
			return true
		}
	}
	return false
}

// Required verifies Bearer admin JWT and loads RBAC permissions.
func Required(secret []byte, store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			claims, err := ParseAccess(secret, token)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired admin token")
				return
			}
			u, err := store.GetUserByID(r.Context(), claims.AdminUserID)
			if err != nil || u.Status != "active" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "admin account inactive")
				return
			}
			perms, err := store.ListPermissions(r.Context(), claims.AdminUserID)
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load permissions")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			ctx = context.WithValue(ctx, permsKey, perms)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission denies unless the admin has the given permission code.
func RequirePermission(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !hasPerm(PermsFromContext(r.Context()), code) {
				httpx.Error(w, http.StatusForbidden, "forbidden", "missing permission: "+code)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
