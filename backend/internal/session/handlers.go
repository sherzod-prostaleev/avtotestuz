package session

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

type Handler struct {
	Svc *Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/sessions", h.startSession)
	r.Post("/sessions/{id}/answers", h.submitAnswer)
	r.Post("/sessions/{id}/finish", h.finishSession)
	r.Get("/sessions/{id}", h.getSession)
	r.Get("/me/sessions", h.listMySessions)
	r.Get("/me/variants", h.listVariantStatuses)
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

type startSessionBody struct {
	Mode      string     `json:"mode"`
	VariantID *uuid.UUID `json:"variant_id"`
	// CategoryID/SignID accept either a UUID or the human-readable `code`
	// returned by GET /categories / GET /signs (content.CategoryDTO/SignDTOs
	// never expose a UUID at all — practice-mode session starts need to
	// work from just the code the content API gave the client). Plain
	// strings here, not *uuid.UUID, since a code like "signs" or "3.27"
	// isn't UUID-shaped and would fail JSON decoding otherwise; resolution
	// to the underlying UUID happens in Service.Resolve{Category,Sign}ID.
	CategoryID *string `json:"category_id"`
	SignID     *string `json:"sign_id"`
	Locale     string  `json:"locale"`
	Count      int     `json:"count"`
}

type startSessionResponse struct {
	ID           string    `json:"id"`
	Mode         string    `json:"mode"`
	QuestionIDs  []string  `json:"question_ids"`
	TimeLimitSec *int      `json:"time_limit_sec"`
	Total        int       `json:"total"`
	StartedAt    time.Time `json:"started_at"`
}

func toStartSessionResponse(v SessionView) startSessionResponse {
	ids := make([]string, len(v.QuestionIDs))
	for i, id := range v.QuestionIDs {
		ids[i] = id.String()
	}
	return startSessionResponse{
		ID:           v.ID.String(),
		Mode:         v.Mode,
		QuestionIDs:  ids,
		TimeLimitSec: v.TimeLimitSec,
		Total:        v.Total,
		StartedAt:    v.StartedAt,
	}
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	var body startSessionBody
	if !decodeBody(w, r, &body) {
		return
	}
	req := StartRequest{
		Mode:   body.Mode,
		Locale: body.Locale,
		Count:  body.Count,
	}
	if body.VariantID != nil {
		req.VariantID = *body.VariantID
	}
	if body.CategoryID != nil {
		id, err := h.Svc.ResolveCategoryID(r.Context(), *body.CategoryID)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		req.CategoryID = id
	}
	if body.SignID != nil {
		id, err := h.Svc.ResolveSignID(r.Context(), *body.SignID)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		req.SignID = id
	}

	view, err := h.Svc.StartSession(r.Context(), claims.ProfileID, req)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, toStartSessionResponse(view))
}

type submitAnswerBody struct {
	QuestionID uuid.UUID `json:"question_id"`
	AnswerID   uuid.UUID `json:"answer_id"`
}

type submitAnswerResponse struct {
	Recorded        bool    `json:"recorded"`
	Correct         *bool   `json:"correct,omitempty"`
	CorrectAnswerID *string `json:"correct_answer_id,omitempty"`
	Stopped         bool    `json:"stopped,omitempty"`
	StopReason      string  `json:"stop_reason,omitempty"`
}

func (h *Handler) submitAnswer(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body submitAnswerBody
	if !decodeBody(w, r, &body) {
		return
	}

	result, err := h.Svc.SubmitAnswer(r.Context(), claims.ProfileID, sessionID, body.QuestionID, body.AnswerID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	resp := submitAnswerResponse{
		Recorded:   result.Recorded,
		Correct:    result.Correct,
		Stopped:    result.Stopped,
		StopReason: result.StopReason,
	}
	if result.CorrectAnswerID != nil {
		s := result.CorrectAnswerID.String()
		resp.CorrectAnswerID = &s
	}
	httpx.Data(w, http.StatusOK, resp)
}

type finishSessionResponse struct {
	Status        string `json:"status"`
	StoppedReason string `json:"stopped_reason"`
	Score         int    `json:"score"`
	Total         int    `json:"total"`
}

func (h *Handler) finishSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	result, err := h.Svc.FinishSession(r.Context(), claims.ProfileID, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, finishSessionResponse(result))
}

type answeredQuestionDTO struct {
	QuestionID string `json:"question_id"`
	Position   int    `json:"position"`
	Answered   bool   `json:"answered"`
	Correct    *bool  `json:"correct,omitempty"`
}

type sessionDetailResponse struct {
	ID            string                `json:"id"`
	Mode          string                `json:"mode"`
	Total         int                   `json:"total"`
	Status        string                `json:"status"`
	StoppedReason string                `json:"stopped_reason"`
	Score         *int                  `json:"score,omitempty"`
	StartedAt     time.Time             `json:"started_at"`
	FinishedAt    *time.Time            `json:"finished_at,omitempty"`
	Answers       []answeredQuestionDTO `json:"answers"`
}

func toSessionDetailResponse(d SessionDetail) sessionDetailResponse {
	answers := make([]answeredQuestionDTO, len(d.Answers))
	for i, a := range d.Answers {
		answers[i] = answeredQuestionDTO{
			QuestionID: a.QuestionID.String(),
			Position:   a.Position,
			Answered:   a.Answered,
			Correct:    a.Correct,
		}
	}
	return sessionDetailResponse{
		ID:            d.ID.String(),
		Mode:          d.Mode,
		Total:         d.Total,
		Status:        d.Status,
		StoppedReason: d.StoppedReason,
		Score:         d.Score,
		StartedAt:     d.StartedAt,
		FinishedAt:    d.FinishedAt,
		Answers:       answers,
	}
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	detail, err := h.Svc.GetSession(r.Context(), claims.ProfileID, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, toSessionDetailResponse(detail))
}

type sessionSummaryDTO struct {
	ID         string     `json:"id"`
	Mode       string     `json:"mode"`
	Status     string     `json:"status"`
	Score      *int       `json:"score,omitempty"`
	Total      int        `json:"total"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func (h *Handler) listMySessions(w http.ResponseWriter, r *http.Request) {
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
	summaries, err := h.Svc.ListMySessions(r.Context(), claims.ProfileID, limit)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	out := make([]sessionSummaryDTO, len(summaries))
	for i, s := range summaries {
		out[i] = sessionSummaryDTO{
			ID:         s.ID.String(),
			Mode:       s.Mode,
			Status:     s.Status,
			Score:      s.Score,
			Total:      s.Total,
			StartedAt:  s.StartedAt,
			FinishedAt: s.FinishedAt,
		}
	}
	httpx.Data(w, http.StatusOK, out)
}

type variantStatusDTO struct {
	Number        int32      `json:"number"`
	QuestionCount int        `json:"question_count"`
	Unlocked      bool       `json:"unlocked"`
	BestCorrect   int        `json:"best_correct"`
	Attempts      int        `json:"attempts"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func (h *Handler) listVariantStatuses(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	statuses, err := h.Svc.ListVariantStatuses(r.Context(), claims.ProfileID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	out := make([]variantStatusDTO, len(statuses))
	for i, s := range statuses {
		out[i] = variantStatusDTO(s)
	}
	httpx.Data(w, http.StatusOK, out)
}

func writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "invalid session request")
	case errors.Is(err, ErrDailyLimitReached):
		httpx.Error(w, http.StatusTooManyRequests, "daily_limit_reached", "daily practice limit reached")
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "session not found")
	case errors.Is(err, ErrAlreadyAnswered):
		httpx.Error(w, http.StatusConflict, "already_answered", "question already answered in this session")
	case errors.Is(err, ErrInvalidAnswer):
		httpx.Error(w, http.StatusBadRequest, "invalid_answer", "answer does not belong to question")
	case errors.Is(err, ErrSessionFinished):
		httpx.Error(w, http.StatusConflict, "session_finished", "session already finished")
	case errors.Is(err, ErrRequiresVIP):
		httpx.Error(w, http.StatusPaymentRequired, "vip_required", "active entitlement required")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
