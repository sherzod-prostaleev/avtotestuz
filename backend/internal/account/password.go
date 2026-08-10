package account

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
)

type changePasswordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// changePassword lets the authenticated learner replace their own password.
// Profile id always comes from the access token (no IDOR via body/URL).
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body changePasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		httpx.Error(w, http.StatusBadRequest, "password_mismatch", "new password confirmation does not match")
		return
	}

	profile, err := h.Q.GetProfileByID(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "profile query failed")
		return
	}
	if !profile.PasswordHash.Valid || profile.PasswordHash.String == "" {
		httpx.Error(w, http.StatusConflict, "password_not_set", "account has no password; set one to continue")
		return
	}
	if !auth.CheckPassword(profile.PasswordHash.String, body.CurrentPassword) {
		httpx.Error(w, http.StatusUnauthorized, "invalid_current_password", "current password is incorrect")
		return
	}
	if body.CurrentPassword == body.NewPassword {
		httpx.Error(w, http.StatusBadRequest, "password_unchanged", "new password must differ from current password")
		return
	}

	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrWeakPassword) {
			httpx.Error(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "password update failed")
		return
	}

	updated, err := h.Q.SetProfilePassword(r.Context(), sqlc.SetProfilePasswordParams{
		ID:                 claims.ProfileID,
		PasswordHash:       pgtype.Text{String: hash, Valid: true},
		MustChangePassword: false,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "password update failed")
		return
	}

	// Invalidate every refresh session after a credential change. The current
	// access token remains valid until its short TTL; refresh forces re-login.
	if err := h.Q.RevokeAllRefreshTokens(r.Context(), claims.ProfileID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "session revoke failed")
		return
	}

	httpx.Data(w, http.StatusOK, map[string]any{
		"ok":                   true,
		"must_change_password": updated.MustChangePassword,
		"sessions_revoked":     true,
	})
}
