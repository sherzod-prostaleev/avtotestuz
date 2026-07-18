package learning

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes the learning engine (FSRS scheduling, mastery, stats)
// over HTTP.
type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/learn/next", h.nextDue)
	r.Post("/learn/review", h.review)
	r.Get("/me/stats", h.stats)
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

func (h *Handler) nextDue(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	limit := 0
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "limit must be an integer")
			return
		}
		limit = n
	}
	ids, err := h.Svc.NextDue(r.Context(), claims.ProfileID, limit)
	if err != nil {
		writeLearningError(w, err)
		return
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	httpx.Data(w, http.StatusOK, out)
}

type reviewBody struct {
	QuestionID uuid.UUID `json:"question_id"`
	Rating     int       `json:"rating"`
}

type reviewResponse struct {
	Stability  float64 `json:"stability"`
	Difficulty float64 `json:"difficulty"`
	DueAt      string  `json:"due_at"`
	Reps       int     `json:"reps"`
	Lapses     int     `json:"lapses"`
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	var body reviewBody
	if !decodeBody(w, r, &body) {
		return
	}
	card, err := h.Svc.RecordReview(r.Context(), claims.ProfileID, body.QuestionID, Rating(body.Rating))
	if err != nil {
		writeLearningError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, reviewResponse{
		Stability:  card.Stability,
		Difficulty: card.Difficulty,
		DueAt:      card.DueAt.Format(time.RFC3339),
		Reps:       card.Reps,
		Lapses:     card.Lapses,
	})
}

type categoryStatDTO struct {
	CategoryCode string  `json:"category_code"`
	Mastery      float64 `json:"mastery"`
	Seen         int     `json:"seen"`
	Correct      int     `json:"correct"`
}

type statsResponse struct {
	Categories   []categoryStatDTO `json:"categories"`
	ReadinessPct int               `json:"readiness_pct"`
	DueCount     int               `json:"due_count"`
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	st, err := h.Svc.Stats(r.Context(), claims.ProfileID)
	if err != nil {
		writeLearningError(w, err)
		return
	}
	cats := make([]categoryStatDTO, len(st.Categories))
	for i, c := range st.Categories {
		cats[i] = categoryStatDTO(c)
	}
	httpx.Data(w, http.StatusOK, statsResponse{
		Categories:   cats,
		ReadinessPct: st.ReadinessPct,
		DueCount:     st.DueCount,
	})
}

func writeLearningError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRating):
		httpx.Error(w, http.StatusBadRequest, "invalid_rating", "rating must be 1-4")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
