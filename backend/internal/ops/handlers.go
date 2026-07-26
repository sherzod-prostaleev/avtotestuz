package ops

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/site"
)

// Handler exposes thin operational controls until M3 Super Admin ships.
type Handler struct {
	Billing billing.Service
	Pool    *pgxpool.Pool
	Token   string
}

// Routes mounts ops endpoints. Call only when Token is non-empty.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/ops/payment-providers", h.requireToken(h.listProviders))
	r.Patch("/ops/payment-providers/{provider}", h.requireToken(h.setProvider))
	r.Get("/ops/site-contacts", h.requireToken(h.getSiteContacts))
	r.Put("/ops/site-contacts", h.requireToken(h.putSiteContacts))
	r.Get("/ops/users", h.requireToken(h.listUsers))
	r.Get("/ops/payments", h.requireToken(h.listPayments))
	r.Get("/ops/audit", h.requireToken(h.listAudit))
	r.Get("/ops/limits", h.requireToken(h.listLimits))
}

func (h *Handler) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(h.Token) == "" {
			httpx.Error(w, http.StatusServiceUnavailable, "ops_disabled", "ops admin token is not configured")
			return
		}
		got := strings.TrimSpace(r.Header.Get("X-Ops-Token"))
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.Token)) != 1 {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid ops token")
			return
		}
		next(w, r)
	}
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	out, err := h.Billing.ListProviderStatuses(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "providers query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type setProviderBody struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) setProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var body setProviderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.Billing.SetProviderEnabled(r.Context(), provider, body.Enabled, "ops")
	if err != nil {
		if strings.Contains(err.Error(), "unknown provider") {
			httpx.Error(w, http.StatusBadRequest, "invalid_provider", "provider must be payme, click, or manual")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to update provider")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) siteStore() site.Store {
	return site.Store{Pool: h.Pool}
}

func (h *Handler) getSiteContacts(w http.ResponseWriter, r *http.Request) {
	out, err := h.siteStore().GetContacts(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "contacts query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) putSiteContacts(w http.ResponseWriter, r *http.Request) {
	site.DecodeAndPut(w, r, h.siteStore(), "ops")
}
