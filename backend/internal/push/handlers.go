package push

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes authenticated /me/push routes.
type Handler struct {
	Svc *Service
}

func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Get("/me/push", h.getStatus)
	r.Post("/me/push/subscribe", h.subscribe)
	r.Delete("/me/push/subscribe", h.unsubscribe)
	r.Post("/me/push/test", h.testPush)
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	status, err := h.Svc.Status(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load push status")
		return
	}
	httpx.Data(w, http.StatusOK, status)
}

type subscribeBody struct {
	Endpoint  string `json:"endpoint"`
	UserAgent string `json:"user_agent"`
	Keys      struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (h *Handler) subscribe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body subscribeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	err := h.Svc.Subscribe(r.Context(), claims.ProfileID, SubscribeInput{
		Endpoint:  body.Endpoint,
		P256dh:    body.Keys.P256dh,
		Auth:      body.Keys.Auth,
		UserAgent: body.UserAgent,
	})
	switch {
	case errors.Is(err, ErrUnconfigured):
		httpx.Error(w, http.StatusServiceUnavailable, "web_push_unconfigured",
			"web push VAPID keys are not configured")
	case errors.Is(err, ErrBadEndpoint), errors.Is(err, ErrBadKeys):
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to save subscription")
	default:
		httpx.Data(w, http.StatusOK, map[string]any{"ok": true})
	}
}

type unsubscribeBody struct {
	Endpoint string `json:"endpoint"`
}

func (h *Handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body unsubscribeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	err := h.Svc.Unsubscribe(r.Context(), claims.ProfileID, body.Endpoint)
	switch {
	case errors.Is(err, ErrBadEndpoint):
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to remove subscription")
	default:
		httpx.Data(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func (h *Handler) testPush(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	sent, err := h.Svc.SendTest(r.Context(), claims.ProfileID)
	switch {
	case errors.Is(err, ErrUnconfigured):
		httpx.Error(w, http.StatusServiceUnavailable, "web_push_unconfigured",
			"web push VAPID keys are not configured")
	case errors.Is(err, ErrNoSubs):
		httpx.Error(w, http.StatusConflict, "no_subscription",
			"enable push on this device first")
	case err != nil:
		httpx.Error(w, http.StatusBadGateway, "delivery_failed", err.Error())
	default:
		httpx.Data(w, http.StatusOK, map[string]any{"sent": sent})
	}
}
