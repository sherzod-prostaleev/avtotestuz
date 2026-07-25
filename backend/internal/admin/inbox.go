package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/support"
)

func (h *Handler) supportStore() support.Store {
	return support.Store{Pool: h.Pool}
}

func (h *Handler) listSupportTickets(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	status := r.URL.Query().Get("status")
	q := r.URL.Query().Get("q")
	out, err := h.supportStore().List(r.Context(), status, q, page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "tickets query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getSupportTicket(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid ticket id")
		return
	}
	out, err := h.supportStore().Get(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "ticket not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "ticket query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type patchTicketBody struct {
	Status    string  `json:"status"`
	AdminNote *string `json:"admin_note"`
}

func (h *Handler) patchSupportTicket(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid ticket id")
		return
	}
	var body patchTicketBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	before, err := h.supportStore().Get(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "ticket not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "ticket query failed")
		return
	}
	after, err := h.supportStore().UpdateStatus(r.Context(), id, body.Status, body.AdminNote)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "ticket not found")
			return
		}
		if strings.Contains(err.Error(), "invalid status") || strings.Contains(err.Error(), "too long") {
			httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "ticket update failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "support.ticket.patch", "support_ticket", id.String(),
		map[string]any{"status": before.Status, "admin_note": before.AdminNote},
		map[string]any{"status": after.Status, "admin_note": after.AdminNote},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, after)
}
