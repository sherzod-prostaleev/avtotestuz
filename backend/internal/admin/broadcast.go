package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/broadcast"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/site"
)

type supportBannerBody struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
	Href    string `json:"href"`
}

type createBroadcastBody struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	ImageURL  string `json:"image_url"`
	ActionURL string `json:"action_url"`
	Audience  string `json:"audience"`
	Channels  string `json:"channels"`
	Confirm   bool   `json:"confirm"`
}

type dryRunBody struct {
	Audience string `json:"audience"`
	Channels string `json:"channels"`
}

func (h *Handler) getSupportBanner(w http.ResponseWriter, r *http.Request) {
	out, err := site.Store{Pool: h.Pool}.GetSupportBanner(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "banner query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) putSupportBanner(w http.ResponseWriter, r *http.Request) {
	var body supportBannerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	store := site.Store{Pool: h.Pool}
	before, _ := store.GetSupportBanner(r.Context())
	after, err := store.PutSupportBanner(r.Context(), site.SupportBanner{
		Enabled: body.Enabled,
		Message: body.Message,
		Href:    body.Href,
	}, "admin:"+adminID.String())
	if err != nil {
		if strings.Contains(err.Error(), "too long") || strings.Contains(err.Error(), "required") {
			httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "banner update failed")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.banner.put", "site_settings", "support_banner",
		before, after, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, after)
}

func (h *Handler) listBroadcastCampaigns(w http.ResponseWriter, r *http.Request) {
	if h.Broadcast == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "broadcast service not wired")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, total, err := h.Broadcast.List(r.Context(), page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "list failed")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, c := range items {
		out = append(out, campaignDTO(c))
	}
	httpx.Data(w, http.StatusOK, map[string]any{"items": out, "total": total})
}

func (h *Handler) getBroadcastCampaign(w http.ResponseWriter, r *http.Request) {
	if h.Broadcast == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "broadcast service not wired")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid campaign id")
		return
	}
	c, err := h.Broadcast.Get(r.Context(), id)
	if errors.Is(err, broadcast.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "campaign not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "get failed")
		return
	}
	httpx.Data(w, http.StatusOK, campaignDTO(c))
}

func (h *Handler) dryRunBroadcast(w http.ResponseWriter, r *http.Request) {
	if h.Broadcast == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "broadcast service not wired")
		return
	}
	var body dryRunBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	counts, err := h.Broadcast.DryRun(r.Context(), body.Audience, body.Channels)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.broadcast.dry_run", "broadcast_campaign", "",
		nil, map[string]any{
			"audience":      body.Audience,
			"channels":      body.Channels,
			"recipients":    counts.Recipients,
			"push_eligible": counts.PushEligible,
		}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, counts)
}

func (h *Handler) createBroadcastCampaign(w http.ResponseWriter, r *http.Request) {
	if h.Broadcast == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "broadcast service not wired")
		return
	}
	var body createBroadcastBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	camp, err := h.Broadcast.Create(r.Context(), broadcast.CreateInput{
		AdminID:        adminID,
		Title:          body.Title,
		Body:           body.Body,
		ImageURL:       body.ImageURL,
		ActionURL:      body.ActionURL,
		Audience:       body.Audience,
		Channels:       body.Channels,
		Confirm:        body.Confirm,
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	})
	switch {
	case errors.Is(err, broadcast.ErrNotConfirm):
		httpx.Error(w, http.StatusBadRequest, "confirm_required", "confirm must be true")
	case errors.Is(err, broadcast.ErrRateLimited):
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many broadcasts; try later")
	case errors.Is(err, broadcast.ErrTooMany):
		httpx.Error(w, http.StatusBadRequest, "audience_too_large", err.Error())
	case err != nil && (strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "must be") ||
		strings.Contains(err.Error(), "too long") ||
		strings.Contains(err.Error(), "not allowed")):
		httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal", "create failed")
	default:
		_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.broadcast.create", "broadcast_campaign", camp.ID.String(),
			nil, campaignDTO(camp), clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
		httpx.Data(w, http.StatusAccepted, campaignDTO(camp))
	}
}

func (h *Handler) cancelBroadcastCampaign(w http.ResponseWriter, r *http.Request) {
	if h.Broadcast == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "broadcast service not wired")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid campaign id")
		return
	}
	camp, err := h.Broadcast.Cancel(r.Context(), id)
	if errors.Is(err, broadcast.ErrInvalidCancel) {
		httpx.Error(w, http.StatusConflict, "not_cancellable", "campaign cannot be cancelled")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "cancel failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.broadcast.cancel", "broadcast_campaign", camp.ID.String(),
		nil, campaignDTO(camp), clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, campaignDTO(camp))
}

func (h *Handler) retractBroadcastCampaign(w http.ResponseWriter, r *http.Request) {
	if h.Broadcast == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "broadcast service not wired")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid campaign id")
		return
	}
	camp, err := h.Broadcast.Retract(r.Context(), id)
	if errors.Is(err, broadcast.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "broadcast not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "retract failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.broadcast.retract", "broadcast_campaign", camp.ID.String(),
		nil, campaignDTO(camp), clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, campaignDTO(camp))
}

func campaignDTO(c sqlc.BroadcastCampaign) map[string]any {
	out := map[string]any{
		"id":                c.ID.String(),
		"created_by_admin":  c.CreatedByAdmin.String(),
		"title":             c.Title,
		"body":              c.Body,
		"image_url":         c.ImageUrl,
		"action_url":        c.ActionUrl,
		"audience":          c.Audience,
		"channels":          c.Channels,
		"status":            c.Status,
		"recipient_total":   c.RecipientTotal,
		"pending_count":     c.PendingCount,
		"sent_count":        c.SentCount,
		"failed_count":      c.FailedCount,
		"push_sent_count":   c.PushSentCount,
		"push_failed_count": c.PushFailedCount,
		"error_summary":     c.ErrorSummary,
		"created_at":        ts(c.CreatedAt),
	}
	if c.IdempotencyKey.Valid {
		out["idempotency_key"] = c.IdempotencyKey.String
	}
	if c.QueuedAt.Valid {
		out["queued_at"] = ts(c.QueuedAt)
	}
	if c.StartedAt.Valid {
		out["started_at"] = ts(c.StartedAt)
	}
	if c.FinishedAt.Valid {
		out["finished_at"] = ts(c.FinishedAt)
	}
	return out
}

func ts(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339Nano)
}
