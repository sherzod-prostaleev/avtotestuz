package session

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/content"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

type Handler struct {
	Svc     *Service
	Content *content.Handler
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/sessions", h.startSession)
	r.Post("/sessions/{id}/answers", h.submitAnswer)
	r.Post("/sessions/{id}/finish", h.finishSession)
	r.Get("/sessions/{id}", h.getSession)
	r.Get("/sessions/{id}/questions/{questionID}", h.getSessionQuestion)
	r.Get("/me/practice-allowance", h.practiceAllowance)
	r.Get("/me/mock-eligibility", h.mockEligibility)
	r.Get("/me/sessions", h.listMySessions)
	r.Get("/me/variants", h.listVariantStatuses)
}

// PublicRoutes mounts unauthenticated certificate lookup.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/grand-mock/certificates/{code}", h.getPublicCertificate)
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
	Mode string `json:"mode"`
	// VariantID/CategoryID/SignID accept either a UUID or the human-readable
	// identifier the content API actually exposes for that resource — GET
	// /variants never returns a bilet's UUID, only its `number`
	// (content.VariantListItemDTO), and GET /categories / GET /signs never
	// return a UUID at all, only `code` (content.CategoryDTO/SignDTOs).
	// Plain strings here, not *uuid.UUID, since neither a bilet number
	// ("12") nor a code like "signs"/"3.27" is UUID-shaped and would fail
	// JSON decoding otherwise; resolution to the underlying UUID happens in
	// Service.Resolve{Variant,Category,Sign}ID.
	VariantID  *string `json:"variant_id"`
	CategoryID *string `json:"category_id"`
	SignID     *string `json:"sign_id"`
	// HasImage is a practice selector like CategoryID/SignID, but needs no
	// resolution: it is a plain boolean over question.image_id.
	HasImage *bool `json:"has_image"`
	// Bilet span selector; numbers, not codes, so no resolution is needed.
	VariantFrom int    `json:"variant_from"`
	VariantTo   int    `json:"variant_to"`
	Locale      string `json:"locale"`
	Count       int    `json:"count"`
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

type practiceAllowanceResponse struct {
	Unlimited bool `json:"unlimited"`
	Limit     int  `json:"limit"`
	Used      int  `json:"used"`
	Remaining int  `json:"remaining"`
}

// practiceAllowance reports today's practice budget so the picker can say how
// many questions a chosen size will actually deliver. Without it the UI offers
// sizes the server then silently clamps, which reads as a broken question pool
// rather than as a paywall.
func (h *Handler) practiceAllowance(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	allowance, err := h.Svc.PracticeAllowance(r.Context(), claims.ProfileID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, practiceAllowanceResponse(allowance))
}

type mockEligibilityResponse struct {
	Eligible             bool    `json:"eligible"`
	MasteryPercent       int     `json:"mastery_percent"`
	MinRequiredPercent   int     `json:"min_required_percent"`
	QuestionsStudied     int     `json:"questions_studied"`
	MinRequiredQuestions int     `json:"min_required_questions"`
	IsVIP                bool    `json:"is_vip"`
	Reason               *string `json:"reason"`
}

// mockEligibility reports whether the profile currently qualifies for Grand
// Mock, so GrandMockCard can render its locked/unlocked state without
// guessing. StartSession(mode:"grand_mock") now calls the same
// Svc.MockEligibility, so the card and the server cannot disagree.
func (h *Handler) mockEligibility(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	res, err := h.Svc.MockEligibility(r.Context(), claims.ProfileID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	var reason *string
	if res.Reason != "" {
		reason = &res.Reason
	}
	httpx.Data(w, http.StatusOK, mockEligibilityResponse{
		Eligible:             res.Eligible,
		MasteryPercent:       res.MasteryPercent,
		MinRequiredPercent:   res.MinRequiredPercent,
		QuestionsStudied:     res.QuestionsStudied,
		MinRequiredQuestions: res.MinRequiredQuestions,
		IsVIP:                res.IsVIP,
		Reason:               reason,
	})
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
		id, err := h.Svc.ResolveVariantID(r.Context(), *body.VariantID)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		req.VariantID = id
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
	req.HasImage = body.HasImage
	req.VariantFrom = body.VariantFrom
	req.VariantTo = body.VariantTo

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
	Rating     *int      `json:"rating,omitempty"`
	LatencyMs  *int      `json:"latency_ms,omitempty"`
	SkipFSRS   bool      `json:"skip_fsrs,omitempty"`
}

type submitAnswerResponse struct {
	Recorded        bool                `json:"recorded"`
	Correct         *bool               `json:"correct,omitempty"`
	CorrectAnswerID *string             `json:"correct_answer_id,omitempty"`
	Explanation     *ExplanationPayload `json:"explanation,omitempty"`
	Stopped         bool                `json:"stopped,omitempty"`
	StopReason      string              `json:"stop_reason,omitempty"`
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

	result, err := h.Svc.SubmitAnswer(r.Context(), claims.ProfileID, sessionID, body.QuestionID, body.AnswerID, SubmitAnswerOpts{
		Rating:    body.Rating,
		LatencyMs: body.LatencyMs,
		SkipFSRS:  body.SkipFSRS,
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	resp := submitAnswerResponse{
		Recorded:    result.Recorded,
		Correct:     result.Correct,
		Explanation: result.Explanation,
		Stopped:     result.Stopped,
		StopReason:  result.StopReason,
	}
	if result.CorrectAnswerID != nil {
		s := result.CorrectAnswerID.String()
		resp.CorrectAnswerID = &s
	}
	httpx.Data(w, http.StatusOK, resp)
}

type sessionQuestionDetailResponse struct {
	content.QuestionDetailDTO
	Position        int     `json:"position"`
	Answered        bool    `json:"answered"`
	UserAnswerID    *string `json:"user_answer_id,omitempty"`
	Correct         *bool   `json:"correct,omitempty"`
	CorrectAnswerID *string `json:"correct_answer_id,omitempty"`
}

func (h *Handler) getSessionQuestion(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	questionID, ok := parseUUIDParam(w, r, "questionID")
	if !ok {
		return
	}
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}

	access, err := h.Svc.GetSessionQuestionAccess(r.Context(), claims.ProfileID, sessionID, questionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if h.Content == nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "question content is unavailable")
		return
	}
	detail, fallback, err := h.Content.LoadQuestionDetail(r.Context(), questionID, loc)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "not_found", "question not found")
		} else {
			httpx.Error(w, http.StatusInternalServerError, "internal", "question query failed")
		}
		return
	}
	if !access.ExplanationAllowed {
		detail.Explanation = nil
	}

	response := sessionQuestionDetailResponse{
		QuestionDetailDTO: detail,
		Position:          access.Position,
		Answered:          access.Answered,
		Correct:           access.Correct,
	}
	if access.UserAnswerID != nil {
		value := access.UserAnswerID.String()
		response.UserAnswerID = &value
	}
	if access.CorrectAnswerID != nil {
		value := access.CorrectAnswerID.String()
		response.CorrectAnswerID = &value
	}
	httpx.DataMeta(w, http.StatusOK, response, content.LocaleMeta{Locale: loc, Fallback: fallback})
}

type finishSessionResponse struct {
	Status               string `json:"status"`
	StoppedReason        string `json:"stopped_reason"`
	Score                int    `json:"score"`
	Total                int    `json:"total"`
	CertificateShareCode string `json:"certificate_share_code,omitempty"`
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
	QuestionID      string  `json:"question_id"`
	Position        int     `json:"position"`
	Answered        bool    `json:"answered"`
	UserAnswerID    *string `json:"user_answer_id,omitempty"`
	Correct         *bool   `json:"correct,omitempty"`
	CorrectAnswerID *string `json:"correct_answer_id,omitempty"`
}

type sessionDetailResponse struct {
	ID                   string                `json:"id"`
	Mode                 string                `json:"mode"`
	Total                int                   `json:"total"`
	Status               string                `json:"status"`
	StoppedReason        string                `json:"stopped_reason"`
	Score                *int                  `json:"score,omitempty"`
	TimeLimitSec         *int                  `json:"time_limit_sec,omitempty"`
	StartedAt            time.Time             `json:"started_at"`
	FinishedAt           *time.Time            `json:"finished_at,omitempty"`
	Answers              []answeredQuestionDTO `json:"answers"`
	CertificateShareCode string                `json:"certificate_share_code,omitempty"`
}

func toSessionDetailResponse(d SessionDetail) sessionDetailResponse {
	answers := make([]answeredQuestionDTO, len(d.Answers))
	for i, a := range d.Answers {
		answer := answeredQuestionDTO{
			QuestionID: a.QuestionID.String(),
			Position:   a.Position,
			Answered:   a.Answered,
			Correct:    a.Correct,
		}
		if a.UserAnswerID != nil {
			value := a.UserAnswerID.String()
			answer.UserAnswerID = &value
		}
		if a.CorrectAnswerID != nil {
			value := a.CorrectAnswerID.String()
			answer.CorrectAnswerID = &value
		}
		answers[i] = answer
	}
	return sessionDetailResponse{
		ID:                   d.ID.String(),
		Mode:                 d.Mode,
		Total:                d.Total,
		Status:               d.Status,
		StoppedReason:        d.StoppedReason,
		Score:                d.Score,
		TimeLimitSec:         d.TimeLimitSec,
		StartedAt:            d.StartedAt,
		FinishedAt:           d.FinishedAt,
		Answers:              answers,
		CertificateShareCode: d.CertificateShareCode,
	}
}

func (h *Handler) getPublicCertificate(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	cert, err := h.Svc.GetPublicCertificate(r.Context(), code)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"share_code": cert.ShareCode,
		"score":      cert.Score,
		"total":      cert.Total,
		"issued_at":  cert.IssuedAt.UTC().Format(time.RFC3339),
	})
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
	case errors.Is(err, ErrQuestionNotAssigned):
		httpx.Error(w, http.StatusBadRequest, "question_not_assigned", "question is not assigned to this session")
	case errors.Is(err, ErrSessionFinished):
		httpx.Error(w, http.StatusConflict, "session_finished", "session already finished")
	case errors.Is(err, ErrRequiresVIP):
		httpx.Error(w, http.StatusPaymentRequired, "vip_required", "active entitlement required")
	case errors.Is(err, ErrNothingDue):
		httpx.Error(w, http.StatusBadRequest, "nothing_due", "no due reviews right now")
	case errors.Is(err, ErrMockNotEligible):
		// English like every other message here: the client localizes by code
		// (see GET /me/mock-eligibility for the specific reason and numbers).
		httpx.Error(w, http.StatusForbidden, "mock_not_eligible", "grand mock study requirements not met")
	case errors.Is(err, ErrVariantLocked):
		httpx.Error(w, http.StatusForbidden, "variant_locked", "complete the previous variant first")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
}
