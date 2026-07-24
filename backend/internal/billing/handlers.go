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
	ClickServiceID    string
	ClickMerchantID   string
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
	r.Post("/billing/promo/validate", h.validatePromo)
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

type validatePromoBody struct {
	Code       string `json:"code"`
	TariffCode string `json:"tariff_code"`
}

func (h *Handler) validatePromo(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body validatePromoBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if body.Code == "" || body.TariffCode == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "code and tariff_code are required")
		return
	}

	res, err := h.Svc.ValidatePromo(r.Context(), claims.ProfileID, body.Code, body.TariffCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrPromoNotFound):
			httpx.Error(w, http.StatusNotFound, "promo_not_found", "promo code not found or inactive")
		case errors.Is(err, ErrPromoExpired):
			httpx.Error(w, http.StatusBadRequest, "promo_expired", "promo code expired")
		case errors.Is(err, ErrPromoNotStarted):
			httpx.Error(w, http.StatusBadRequest, "promo_not_started", "promo code not yet active")
		case errors.Is(err, ErrPromoLimitReached):
			httpx.Error(w, http.StatusBadRequest, "promo_limit_reached", "promo code maximum usage limit reached")
		case errors.Is(err, ErrPromoUserLimitReached):
			httpx.Error(w, http.StatusBadRequest, "promo_user_limit_reached", "per-user promo code limit reached")
		case errors.Is(err, pgx.ErrNoRows):
			httpx.Error(w, http.StatusBadRequest, "invalid_tariff", "unknown or inactive tariff code")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "promo validation failed")
		}
		return
	}
	httpx.Data(w, http.StatusOK, res)
}

type checkoutBody struct {
	TariffCode string `json:"tariff_code"`
	Provider   string `json:"provider"`
	PromoCode  string `json:"promo_code,omitempty"`
}

// checkout starts a checkout (Payme or Click) for the authed profile:
// creates a 'created' payment for the requested tariff and returns its
// checkout URL. Provider defaults to "payme" when omitted.
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
	provider := body.Provider
	if provider == "" {
		provider = "payme"
	}
	if provider != "payme" && provider != "click" {
		httpx.Error(w, http.StatusBadRequest, "invalid_provider", "provider must be payme or click")
		return
	}
	cfg := CheckoutConfig{
		PaymeMerchantID:   h.PaymeMerchantID,
		PaymeCheckoutHost: h.PaymeCheckoutHost,
		ClickServiceID:    h.ClickServiceID,
		ClickMerchantID:   h.ClickMerchantID,
	}
	result, err := h.Svc.StartCheckout(r.Context(), claims.ProfileID, body.TariffCode, provider, cfg, loc, "", body.PromoCode)
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
