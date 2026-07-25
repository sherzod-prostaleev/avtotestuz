package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/site"
)

func (h *Handler) siteStore() site.Store {
	return site.Store{Pool: h.Pool}
}

func (h *Handler) getCMSContacts(w http.ResponseWriter, r *http.Request) {
	out, err := h.siteStore().GetContacts(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "contacts query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) putCMSContacts(w http.ResponseWriter, r *http.Request) {
	before, err := h.siteStore().GetContacts(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "contacts query failed")
		return
	}
	var body site.Contacts
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	updatedBy := "admin:" + adminID.String()
	after, err := h.siteStore().PutContacts(r.Context(), body, updatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "too long") {
			httpx.Error(w, http.StatusBadRequest, "invalid_field", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to update contacts")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "cms.contacts.put", "site_settings", "contacts",
		contactsAuditMap(before), contactsAuditMap(after),
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, after)
}

func contactsAuditMap(c site.Contacts) map[string]any {
	return map[string]any{
		"phone":        c.Phone,
		"phoneTel":     c.PhoneTel,
		"email":        c.Email,
		"address":      c.Address,
		"hours":        c.Hours,
		"telegram":     c.Telegram,
		"telegramUrl":  c.TelegramURL,
		"instagram":    c.Instagram,
		"instagramUrl": c.InstagramURL,
	}
}
