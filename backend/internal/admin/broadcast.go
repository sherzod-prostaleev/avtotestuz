package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/push"
	"avtotest.uz/backend/internal/site"
)

type supportBannerBody struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
	Href    string `json:"href"`
}

type webPushBroadcastBody struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	DryRun bool   `json:"dry_run"`
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

func (h *Handler) postWebPushBroadcast(w http.ResponseWriter, r *http.Request) {
	if h.Push == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "web_push_unconfigured",
			"web push is not wired on this admin handler")
		return
	}
	var body webPushBroadcastBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "body is required")
		return
	}
	if body.Title == "" {
		body.Title = "Driver Go"
	}
	res, err := h.Push.BroadcastSupport(r.Context(), push.BroadcastOpts{
		Title:  body.Title,
		Body:   body.Body,
		URL:    body.URL,
		DryRun: body.DryRun,
	})
	if err != nil {
		if err == push.ErrUnconfigured {
			httpx.Error(w, http.StatusServiceUnavailable, "web_push_unconfigured",
				"web push VAPID keys are not configured")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "broadcast failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.broadcast.webpush", "push_broadcast", "",
		nil, map[string]any{
			"title":      body.Title,
			"body":       body.Body,
			"url":        body.URL,
			"dry_run":    body.DryRun,
			"recipients": res.Recipients,
			"notified":   res.Notified,
			"deliveries": res.Deliveries,
			"errors":     res.Errors,
		}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, res)
}
