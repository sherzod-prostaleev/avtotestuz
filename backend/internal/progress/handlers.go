package progress

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/me/saved", h.listSaved)
	r.Post("/me/saved", h.saveQuestion)
	r.Delete("/me/saved/{question_id}", h.unsaveQuestion)
	r.Get("/me/streak", h.getStreak)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return false
	}
	return true
}

func claimsOrUnauthorized(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return auth.Claims{}, false
	}
	return claims, true
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", name+" must be a UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

type savedItemDTO struct {
	QuestionID string    `json:"question_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *Handler) listSaved(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	items, err := h.Svc.ListSaved(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
		return
	}
	out := make([]savedItemDTO, len(items))
	for i, it := range items {
		out[i] = savedItemDTO{QuestionID: it.QuestionID.String(), CreatedAt: it.CreatedAt}
	}
	httpx.Data(w, http.StatusOK, out)
}

type saveQuestionBody struct {
	QuestionID uuid.UUID `json:"question_id"`
}

func (h *Handler) saveQuestion(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	var body saveQuestionBody
	if !decodeBody(w, r, &body) {
		return
	}
	if body.QuestionID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "question_id is required")
		return
	}
	if err := h.Svc.SaveQuestion(r.Context(), claims.ProfileID, body.QuestionID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) unsaveQuestion(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	questionID, ok := parseUUIDParam(w, r, "question_id")
	if !ok {
		return
	}
	if err := h.Svc.UnsaveQuestion(r.Context(), claims.ProfileID, questionID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

type streakDTO struct {
	Current        int     `json:"current"`
	Best           int     `json:"best"`
	TodayDone      int     `json:"today_done"`
	DailyGoal      int     `json:"daily_goal"`
	LastActiveDate *string `json:"last_active_date"`
}

func (h *Handler) getStreak(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	view, err := h.Svc.GetStreak(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
		return
	}
	dto := streakDTO{
		Current:   view.Current,
		Best:      view.Best,
		TodayDone: view.TodayDone,
		DailyGoal: view.DailyGoal,
	}
	if view.LastActiveDate != nil {
		s := view.LastActiveDate.Format("2006-01-02")
		dto.LastActiveDate = &s
	}
	httpx.Data(w, http.StatusOK, dto)
}
