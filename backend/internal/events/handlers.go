package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/events", h.logBatch)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return false
	}
	return true
}

type eventDTO struct {
	Name  string          `json:"name"`
	Props json.RawMessage `json:"props,omitempty"`
	TS    *time.Time      `json:"ts,omitempty"`
}

type logBatchBody struct {
	IdempotencyKey string     `json:"idempotency_key"`
	Events         []eventDTO `json:"events"`
}

func (h *Handler) logBatch(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body logBatchBody
	if !decodeBody(w, r, &body) {
		return
	}

	evs := make([]Event, len(body.Events))
	for i, e := range body.Events {
		evs[i] = Event(e)
	}

	if err := h.Svc.LogBatch(r.Context(), claims.ProfileID, body.IdempotencyKey, evs); err != nil {
		writeEventsError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"ok": true, "count": len(evs)})
}

func writeEventsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "invalid event batch, fields, or idempotency key")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
