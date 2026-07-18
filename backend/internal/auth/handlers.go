package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/httpx"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/auth/otp/request", h.requestOTP)
	r.Post("/auth/otp/verify", h.verifyOTP)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type otpRequestBody struct {
	Phone string `json:"phone"`
}

type otpRequestResponse struct {
	Channel   string `json:"channel"`
	DebugCode string `json:"debug_code,omitempty"`
}

func (h *Handler) requestOTP(w http.ResponseWriter, r *http.Request) {
	var body otpRequestBody
	if !decodeBody(w, r, &body) {
		return
	}
	res, err := h.Svc.RequestOTP(r.Context(), body.Phone, clientIP(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, otpRequestResponse(res))
}

type otpVerifyBody struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type tokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
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
	httpx.Data(w, http.StatusOK, tokensResponse{AccessToken: res.Access, RefreshToken: res.Refresh})
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

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrRateLimited):
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again later")
	case errors.Is(err, ErrInvalidPhone):
		httpx.Error(w, http.StatusBadRequest, "invalid_phone", "phone number is not a valid Uzbekistan number")
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
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
