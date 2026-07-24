package billing

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

// Handler serves the billing endpoints: public tariff reads plus the
// authed checkout initiate. PaymeMerchantID/PaymeCheckoutHost feed
// BuildPaymeURL for the checkout endpoint; the Payme webhook itself is a
// separate payme.Handler mounted directly by server.go.
type Handler struct {
	Svc               Service
	PaymeMerchantID   string
	PaymeCheckoutHost string
}

// Routes mounts the public, unauthenticated billing endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/tariffs", h.listTariffs)
}

// AuthedRoutes mounts the billing endpoints that require a JWT-authed
// profile. Callers mount this behind auth.Required, e.g.
// bh.AuthedRoutes(api.With(auth.Required(...))).
func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Post("/me/checkout", h.checkout)
}

func (h *Handler) listTariffs(w http.ResponseWriter, r *http.Request) {
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	out, err := h.Svc.ListTariffs(r.Context(), loc)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "tariffs query failed")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpx.Data(w, http.StatusOK, out)
}

type checkoutBody struct {
	TariffCode string `json:"tariff_code"`
}

// checkout starts a Payme checkout for the authed profile: creates a
// 'created' payment for the requested tariff and returns its checkout URL.
func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	result, err := h.Svc.StartCheckout(r.Context(), claims.ProfileID, body.TariffCode, h.PaymeMerchantID, h.PaymeCheckoutHost, loc, "")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusBadRequest, "invalid_tariff", "unknown or inactive tariff code")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "checkout failed")
		return
	}
	httpx.Data(w, http.StatusOK, result)
}
