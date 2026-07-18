package explanation

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/explanations/feedback", h.feedback)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return false
	}
	return true
}

type feedbackBody struct {
	QuestionID uuid.UUID `json:"question_id"`
	Helpful    bool      `json:"helpful"`
}

func (h *Handler) feedback(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body feedbackBody
	if !decodeBody(w, r, &body) {
		return
	}

	if err := h.Svc.RecordFeedback(r.Context(), claims.ProfileID, body.QuestionID, body.Helpful); err != nil {
		writeExplanationError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeExplanationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "explanation not found")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
