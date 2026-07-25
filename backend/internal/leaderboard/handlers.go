package leaderboard

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/leaderboard", h.getLeaderboard)
}

type entryDTO struct {
	Rank  int    `json:"rank"`
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type youDTO struct {
	Rank  *int   `json:"rank"`
	Score int    `json:"score"`
	Name  string `json:"name"`
}

type leaderboardResponse struct {
	Period    string     `json:"period"`
	You       youDTO     `json:"you"`
	Top       []entryDTO `json:"top"`
	AroundYou []entryDTO `json:"around_you"`
}

func (h *Handler) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}

	period := Period(r.URL.Query().Get("period"))
	valid := false
	for _, p := range AllPeriods {
		if p == period {
			valid = true
			break
		}
	}
	if !valid {
		httpx.Error(w, http.StatusBadRequest, "invalid_period", "period must be one of daily, weekly, monthly, alltime")
		return
	}

	res, err := h.Svc.GetLeaderboard(r.Context(), claims.ProfileID, period)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load leaderboard")
		return
	}
	httpx.Data(w, http.StatusOK, toResponse(res))
}

func toEntryDTOs(entries []Entry) []entryDTO {
	out := make([]entryDTO, len(entries))
	for i, e := range entries {
		out[i] = entryDTO{Rank: e.Rank, Name: e.Name, Score: e.Score}
	}
	return out
}

func toResponse(res Result) leaderboardResponse {
	return leaderboardResponse{
		Period:    string(res.Period),
		You:       youDTO{Rank: res.YouRank, Score: res.YouScore, Name: res.YouName},
		Top:       toEntryDTOs(res.Top),
		AroundYou: toEntryDTOs(res.AroundYou),
	}
}
