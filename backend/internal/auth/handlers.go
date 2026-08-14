package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/httpx"
)

type Handler struct {
	Svc         *Service
	ClientIPs   ClientIPResolver
	BotUsername string
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)
	// OTP kept for sandbox/admin tooling; learner UI uses password auth.
	r.Post("/auth/otp/request", h.requestOTP)
	r.Post("/auth/otp/verify", h.verifyOTP)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/password-reset/start", h.passwordResetStart)
	r.Get("/auth/password-reset/status", h.passwordResetStatus)
	r.Post("/auth/password-reset/complete", h.passwordResetComplete)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return false
	}
	return true
}

type registerBody struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginBody struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type otpRequestBody struct {
	Phone string `json:"phone"`
}

type otpRequestResponse struct {
	Channel   string `json:"channel"`
	DebugCode string `json:"debug_code,omitempty"`
}

type otpVerifyBody struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type tokensResponse struct {
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"refresh_token"`
	MustChangePassword bool   `json:"must_change_password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if !decodeBody(w, r, &body) {
		return
	}
	res, err := h.Svc.Register(r.Context(), RegisterInput{
		Phone:    body.Phone,
		Password: body.Password,
		Name:     body.Name,
		IP:       h.ClientIPs.Resolve(r),
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, tokensResponse{
		AccessToken:        res.Access,
		RefreshToken:       res.Refresh,
		MustChangePassword: res.Profile.MustChangePassword,
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if !decodeBody(w, r, &body) {
		return
	}
	res, err := h.Svc.Login(r.Context(), LoginInput{
		Phone:    body.Phone,
		Password: body.Password,
		IP:       h.ClientIPs.Resolve(r),
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, tokensResponse{
		AccessToken:        res.Access,
		RefreshToken:       res.Refresh,
		MustChangePassword: res.Profile.MustChangePassword,
	})
}

func (h *Handler) requestOTP(w http.ResponseWriter, r *http.Request) {
	var body otpRequestBody
	if !decodeBody(w, r, &body) {
		return
	}
	res, err := h.Svc.RequestOTP(r.Context(), body.Phone, h.ClientIPs.Resolve(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, otpRequestResponse(res))
}

func (h *Handler) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var body otpVerifyBody
	if !decodeBody(w, r, &body) {
		return
	}
	res, err := h.Svc.VerifyOTP(r.Context(), body.Phone, body.Code)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, tokensResponse{
		AccessToken:        res.Access,
		RefreshToken:       res.Refresh,
		MustChangePassword: res.Profile.MustChangePassword,
	})
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if !decodeBody(w, r, &body) {
		return
	}
	toks, err := h.Svc.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, tokensResponse{AccessToken: toks.Access, RefreshToken: toks.Refresh})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.Svc.Logout(r.Context(), body.RefreshToken); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "logout failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

type passwordResetStartBody struct {
	Phone string `json:"phone"`
}

func (h *Handler) passwordResetStart(w http.ResponseWriter, r *http.Request) {
	var body passwordResetStartBody
	if !decodeBody(w, r, &body) {
		return
	}
	res, err := h.Svc.StartPasswordReset(r.Context(), body.Phone, h.ClientIPs.Resolve(r), h.BotUsername)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, res)
}

func (h *Handler) passwordResetStatus(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	httpx.Data(w, http.StatusOK, h.Svc.PasswordResetStatus(r.Context(), token))
}

type passwordResetCompleteBody struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) passwordResetComplete(w http.ResponseWriter, r *http.Request) {
	var body passwordResetCompleteBody
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.Svc.CompletePasswordReset(r.Context(), body.Token, body.Password, h.ClientIPs.Resolve(r)); err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOTPDisabled):
		// 404 rather than 4xx-with-detail: OTP is not part of this
		// deployment's sign-in, so the endpoint should look absent instead
		// of advertising a disabled feature to probe.
		httpx.Error(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrRateLimited):
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again later")
	case errors.Is(err, ErrInvalidPhone):
		httpx.Error(w, http.StatusBadRequest, "invalid_phone", "phone number is not a valid Uzbekistan number")
	case errors.Is(err, ErrWeakPassword):
		httpx.Error(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
	case errors.Is(err, ErrPhoneTaken):
		httpx.Error(w, http.StatusConflict, "phone_taken", "phone number is already registered")
	case errors.Is(err, ErrPasswordNotSet):
		httpx.Error(w, http.StatusConflict, "password_not_set", "account has no password; set one to continue")
	case errors.Is(err, ErrInvalidCreds):
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid phone or password")
	case errors.Is(err, ErrAccountBlocked):
		httpx.Error(w, http.StatusForbidden, "account_blocked", "account is blocked")
	case errors.Is(err, ErrExpiredCode):
		httpx.Error(w, http.StatusBadRequest, "expired_code", "code has expired")
	case errors.Is(err, ErrTooManyAttempts):
		httpx.Error(w, http.StatusBadRequest, "too_many_attempts", "too many incorrect attempts")
	case errors.Is(err, ErrInvalidCode):
		httpx.Error(w, http.StatusBadRequest, "invalid_code", "code is incorrect")
	case errors.Is(err, ErrReusedRefresh):
		httpx.Error(w, http.StatusUnauthorized, "refresh_reused", "refresh token reuse detected, all sessions revoked")
	case errors.Is(err, ErrInvalidRefresh):
		httpx.Error(w, http.StatusUnauthorized, "invalid_refresh", "refresh token is invalid or expired")
	case errors.Is(err, ErrTelegramBotUnconfigured):
		httpx.Error(w, http.StatusServiceUnavailable, "telegram_bot_unconfigured", "telegram bot is not configured")
	case errors.Is(err, ErrResetNotVerified):
		httpx.Error(w, http.StatusBadRequest, "reset_not_verified", "confirm the reset in Telegram first")
	case errors.Is(err, ErrResetInvalid):
		httpx.Error(w, http.StatusBadRequest, "invalid_reset_token", "reset link is invalid or expired")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
