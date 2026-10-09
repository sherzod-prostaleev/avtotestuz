package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

type setFirstPasswordBody struct {
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

const (
	// setFirstPasswordPerHour bounds attempts per profile: the endpoint
	// hashes a password (bcrypt) and, on success, ends sessions and messages
	// the learner, so a stolen session must not be able to spin it.
	setFirstPasswordPerHour = 5
	// refreshTokenHeader carries the caller's own refresh token from the BFF
	// (which holds it in an HttpOnly cookie), so that session can be the one
	// kept. Server-to-server only; it is never logged.
	refreshTokenHeader = "X-Avtotest-Refresh-Token"
	// passwordNoticeTimeout caps the Telegram call the response waits for.
	passwordNoticeTimeout = 5 * time.Second
)

// setFirstPassword gives an account created through Telegram (no password)
// its first password, with the same policy as registration. Learners only: a
// station's shadow profile never gets one.
//
// It never replaces an existing password — that needs the current one
// (changePassword) or the bot reset — so a stolen session cannot use it to
// take over a password account. On a passwordless account a stolen session
// CAN set one, which would turn a single phished sign-in into access that
// outlives every logout (audit I4). Hence, on success:
//   - every other session of the profile ends; the caller keeps theirs when
//     the BFF names it (refreshTokenHeader), otherwise all end;
//   - the linked Telegram chat is told, so the owner hears of a password they
//     did not set and can take the account back with the bot reset;
//   - attempts are limited per profile.
func (h *Handler) setFirstPassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	profile, err := h.Q.GetProfileByID(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "profile query failed")
		return
	}
	if claims.StationID != uuid.Nil || profile.Kind != "user" {
		httpx.Error(w, http.StatusForbidden, "forbidden", "only a learner account can have a password")
		return
	}
	if allowed, err := h.Lim.Allow(r.Context(), "pwset:profile:"+claims.ProfileID.String(), setFirstPasswordPerHour, time.Hour); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "rate limiter unavailable")
		return
	} else if !allowed {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again later")
		return
	}
	var body setFirstPasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		httpx.Error(w, http.StatusBadRequest, "password_mismatch", "new password confirmation does not match")
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
	if _, err := h.Q.SetProfilePasswordIfUnset(r.Context(), sqlc.SetProfilePasswordIfUnsetParams{
		ID:           claims.ProfileID,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusConflict, "password_already_set", "account already has a password; change it instead")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "password update failed")
		return
	}

	keep := ""
	if raw := strings.TrimSpace(r.Header.Get(refreshTokenHeader)); raw != "" {
		keep = auth.HashToken(raw)
	}
	// The password is already set; a failure here must still be an error, or
	// the page would report success while a stranger's session lives on.
	if _, err := h.Q.DeleteOtherRefreshTokens(r.Context(), sqlc.DeleteOtherRefreshTokensParams{
		ProfileID: claims.ProfileID, KeepTokenHash: keep,
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "session revoke failed")
		return
	}
	if h.PasswordNotices != nil {
		// Detached from the request: a learner who closes the tab right after
		// submitting must still be told.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), passwordNoticeTimeout)
		h.PasswordNotices.FirstPasswordSet(ctx, claims.ProfileID)
		cancel()
	}
	httpx.Data(w, http.StatusOK, map[string]any{"ok": true, "other_sessions_ended": true})
}
