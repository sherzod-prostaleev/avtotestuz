package supportchat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes learner + admin HTTP/WS endpoints.
type Handler struct {
	Svc *Service
	// AdminIDFromContext resolves the authenticated admin user id.
	// Wired from server to avoid an admin↔supportchat import cycle.
	AdminIDFromContext func(ctx context.Context) (uuid.UUID, bool)
}

func (h *Handler) adminID(r *http.Request) (uuid.UUID, bool) {
	if h.AdminIDFromContext == nil {
		return uuid.Nil, false
	}
	return h.AdminIDFromContext(r.Context())
}

// LearnerRoutes mounts authenticated learner support chat under /api/v1.
func (h *Handler) LearnerRoutes(r chi.Router) {
	r.Get("/me/support/conversation", h.getMyConversation)
	r.Get("/me/support/messages", h.listMyMessages)
	r.Get("/me/support/messages/{id}/attachment", h.downloadMyAttachment)
	r.Post("/me/support/messages", h.postMyMessage)
	r.Post("/me/support/attachments", h.uploadMyAttachment)
	r.Post("/me/support/read", h.markMyRead)
	r.Get("/me/support/unread", h.getMyUnread)
	r.Post("/me/support/ws-ticket", h.mintUserTicket)
}

// LearnerPublicRoutes mounts ticket-redeemed WebSocket (no JWT cookie).
func (h *Handler) LearnerPublicRoutes(r chi.Router) {
	r.Get("/support/ws", h.serveUserWS)
}

// AdminRoutes mounts permission-gated admin chat under /admin/v1.
// Caller must already apply RequirePermission("support.inbox").
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/support/conversations", h.adminListConversations)
	r.Get("/support/conversations/{id}", h.adminGetConversation)
	r.Post("/support/conversations/{id}/messages", h.adminPostMessage)
	r.Post("/support/conversations/{id}/attachments", h.adminUpload)
	r.Get("/support/messages/{id}/attachment", h.adminDownloadAttachment)
	r.Patch("/support/conversations/{id}", h.adminPatchConversation)
	r.Post("/support/ws-ticket", h.mintAdminTicket)
}

// AdminPublicRoutes mounts admin WS (ticket auth).
func (h *Handler) AdminPublicRoutes(r chi.Router) {
	r.Get("/support/ws", h.serveAdminWS)
}

func (h *Handler) getMyConversation(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	conv, err := h.Svc.Store.GetOrCreateConversation(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "conversation failed")
		return
	}
	httpx.Data(w, http.StatusOK, conv)
}

func (h *Handler) listMyMessages(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	conv, err := h.Svc.Store.GetOrCreateConversation(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "conversation failed")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	var before *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		t, err := time.Parse(time.RFC3339Nano, b)
		if err != nil {
			t, err = time.Parse(time.RFC3339, b)
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_before", "invalid before cursor")
			return
		}
		t = t.UTC()
		before = &t
	}
	msgs, err := h.Svc.Store.ListMessages(r.Context(), conv.ID, before, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "messages failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"conversation": conv,
		"items":        withAttachmentURLs(msgs, false),
	})
}

type postMessageBody struct {
	Body           string `json:"body"`
	AttachmentKey  string `json:"attachment_key"`
	AttachmentName string `json:"attachment_name"`
	AttachmentMime string `json:"attachment_mime"`
	AttachmentSize int64  `json:"attachment_size"`
}

func (h *Handler) postMyMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	if h.Svc.Lim.R != nil {
		allowed, err := h.Svc.Lim.Allow(r.Context(), "supportchat:msg:user:"+claims.ProfileID.String(), 60, time.Minute)
		if err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "temporarily_unavailable", "support unavailable")
			return
		}
		if !allowed {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many messages")
			return
		}
	}
	var body postMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	msg, conv, err := h.Svc.PostUserMessage(r.Context(), claims.ProfileID,
		body.Body, body.AttachmentKey, body.AttachmentName, body.AttachmentMime, body.AttachmentSize)
	if err != nil {
		writeMsgErr(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, map[string]any{
		"message": withAttachmentURL(msg, false), "conversation": conv,
	})
}

func (h *Handler) uploadMyAttachment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	if h.Svc.Lim.R != nil {
		allowed, err := h.Svc.Lim.Allow(r.Context(), "supportchat:up:user:"+claims.ProfileID.String(), 20, time.Hour)
		if err != nil || !allowed {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many uploads")
			return
		}
	}
	conv, err := h.Svc.Store.GetOrCreateConversation(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "conversation failed")
		return
	}
	up, err := h.Svc.storeUpload(r, conv.ID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_file", err.Error())
		return
	}
	httpx.Data(w, http.StatusCreated, up)
}

func (h *Handler) markMyRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	conv, err := h.Svc.Store.GetByProfile(r.Context(), claims.ProfileID)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Data(w, http.StatusOK, map[string]any{"unread_user": 0})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "conversation failed")
		return
	}
	conv, err = h.Svc.Store.ClearUnreadUser(r.Context(), conv.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "read failed")
		return
	}
	h.Svc.broadcastConversation(conv)
	httpx.Data(w, http.StatusOK, conv)
}

func (h *Handler) getMyUnread(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	n, err := h.Svc.Store.TotalUnreadUser(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "unread failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]int{"unread": n})
}

func (h *Handler) mintUserTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	tok, exp, err := h.Svc.MintUserTicket(r.Context(), claims.ProfileID)
	if err != nil {
		if err.Error() == "rate_limited" {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many ticket requests")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "ticket mint failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"ticket":         tok,
		"expires_in_sec": exp,
		"ws_url":         h.wsURL(r, "/api/v1/support/ws"),
	})
}

func (h *Handler) adminListConversations(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, total, err := h.Svc.Store.ListConversations(r.Context(),
		r.URL.Query().Get("status"), r.URL.Query().Get("q"), page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "list failed")
		return
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "limit": limit, "total": total,
	})
}

func (h *Handler) adminGetConversation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid conversation id")
		return
	}
	conv, err := h.Svc.Store.GetByID(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "conversation not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "conversation failed")
		return
	}
	learner, err := h.Svc.Store.GetLearnerSummary(r.Context(), conv.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "learner failed")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	var before *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		t, err := time.Parse(time.RFC3339Nano, b)
		if err != nil {
			t, err = time.Parse(time.RFC3339, b)
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_before", "invalid before cursor")
			return
		}
		t = t.UTC()
		before = &t
	}
	msgs, err := h.Svc.Store.ListMessages(r.Context(), conv.ID, before, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "messages failed")
		return
	}
	// Opening a thread clears admin unread.
	if conv.UnreadAdmin > 0 {
		if c2, err := h.Svc.Store.ClearUnreadAdmin(r.Context(), conv.ID); err == nil {
			conv = c2
			h.Svc.broadcastConversation(conv)
		}
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"conversation": conv,
		"learner":      learner,
		"items":        withAttachmentURLs(msgs, true),
	})
}

func (h *Handler) adminPostMessage(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.adminID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid conversation id")
		return
	}
	if h.Svc.Lim.R != nil {
		allowed, err := h.Svc.Lim.Allow(r.Context(), "supportchat:msg:admin:"+adminID.String(), 120, time.Minute)
		if err != nil || !allowed {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many messages")
			return
		}
	}
	var body postMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	msg, conv, err := h.Svc.PostAdminMessage(r.Context(), adminID, id,
		body.Body, body.AttachmentKey, body.AttachmentName, body.AttachmentMime, body.AttachmentSize)
	if err != nil {
		if err == ErrNotFound {
			httpx.Error(w, http.StatusNotFound, "not_found", "conversation not found")
			return
		}
		writeMsgErr(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, map[string]any{
		"message": withAttachmentURL(msg, true), "conversation": conv,
	})
}

func (h *Handler) adminUpload(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.adminID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid conversation id")
		return
	}
	if _, err := h.Svc.Store.GetByID(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	if h.Svc.Lim.R != nil {
		allowed, err := h.Svc.Lim.Allow(r.Context(), "supportchat:up:admin:"+adminID.String(), 40, time.Hour)
		if err != nil || !allowed {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many uploads")
			return
		}
	}
	up, err := h.Svc.storeUpload(r, id)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_file", err.Error())
		return
	}
	httpx.Data(w, http.StatusCreated, up)
}

type patchConvBody struct {
	Status string `json:"status"`
}

func (h *Handler) adminPatchConversation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid conversation id")
		return
	}
	var body patchConvBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	conv, err := h.Svc.Store.SetStatus(r.Context(), id, body.Status)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "conversation not found")
			return
		}
		if strings.Contains(err.Error(), "invalid status") {
			httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "patch failed")
		return
	}
	h.Svc.broadcastConversation(conv)
	httpx.Data(w, http.StatusOK, conv)
}

func (h *Handler) mintAdminTicket(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.adminID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	tok, exp, err := h.Svc.MintAdminTicket(r.Context(), adminID)
	if err != nil {
		if err.Error() == "rate_limited" {
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many ticket requests")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "ticket mint failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"ticket":         tok,
		"expires_in_sec": exp,
		"ws_url":         h.wsURL(r, "/admin/v1/support/ws"),
	})
}

func writeMsgErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrBadAttachment) {
		httpx.Error(w, http.StatusBadRequest, "invalid_attachment", "attachment_key not owned by conversation")
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "required") || strings.Contains(msg, "too long") ||
		strings.Contains(msg, "invalid") {
		httpx.Error(w, http.StatusBadRequest, "invalid_field", msg)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal", "message failed")
}

func (h *Handler) downloadMyAttachment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	msgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid message id")
		return
	}
	msg, err := h.Svc.Store.GetMessage(r.Context(), msgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "message not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "message failed")
		return
	}
	conv, err := h.Svc.Store.GetByID(r.Context(), msg.ConversationID)
	if err != nil || conv.ProfileID != claims.ProfileID {
		httpx.Error(w, http.StatusNotFound, "not_found", "message not found")
		return
	}
	h.writeAttachment(w, r, msg)
}

func (h *Handler) adminDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	msgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid message id")
		return
	}
	msg, err := h.Svc.Store.GetMessage(r.Context(), msgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "message not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "message failed")
		return
	}
	h.writeAttachment(w, r, msg)
}

func (h *Handler) writeAttachment(w http.ResponseWriter, r *http.Request, msg Message) {
	if strings.TrimSpace(msg.AttachmentKey) == "" {
		httpx.Error(w, http.StatusNotFound, "not_found", "no attachment")
		return
	}
	if h.Svc.Blobs == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "uploads unavailable")
		return
	}
	data, ct, err := h.Svc.Blobs.Get(r.Context(), msg.AttachmentKey)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "attachment missing")
		return
	}
	if ct == "" {
		ct = msg.AttachmentMime
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	name := msg.AttachmentName
	if name == "" {
		name = "attachment"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "inline; filename=\""+strings.ReplaceAll(name, "\"", "")+"\"")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
