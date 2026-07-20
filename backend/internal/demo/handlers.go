package demo

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/demo/question", h.getQuestion)
	r.Post("/demo/answer", h.submitAnswer)
}

// clientIP mirrors internal/auth's helper of the same name — kept as a
// small local copy rather than exporting auth's, since it's five lines and
// this is the only other package that needs it.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	detail, fallback, err := h.Svc.GetQuestion(r.Context(), loc)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "no demo question available")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "demo question query failed")
		return
	}
	httpx.DataMeta(w, http.StatusOK, detail, content.LocaleMeta{Locale: loc, Fallback: fallback})
}

type demoAnswerBody struct {
	QuestionID string `json:"question_id"`
	AnswerID   string `json:"answer_id"`
}

type demoAnswerResponse struct {
	Correct         bool   `json:"correct"`
	CorrectAnswerID string `json:"correct_answer_id"`
}

func (h *Handler) submitAnswer(w http.ResponseWriter, r *http.Request) {
	var body demoAnswerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	questionID, err := uuid.Parse(body.QuestionID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "question_id must be a UUID")
		return
	}
	answerID, err := uuid.Parse(body.AnswerID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "answer_id must be a UUID")
		return
	}

	correct, correctAnswerID, err := h.Svc.SubmitAnswer(r.Context(), clientIP(r), questionID, answerID)
	switch {
	case err == nil:
		httpx.Data(w, http.StatusOK, demoAnswerResponse{Correct: correct, CorrectAnswerID: correctAnswerID.String()})
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "question is not part of the public demo")
	case errors.Is(err, ErrInvalidAnswer):
		httpx.Error(w, http.StatusBadRequest, "invalid_answer", "answer does not belong to question")
	case errors.Is(err, ErrRateLimited):
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again later")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
