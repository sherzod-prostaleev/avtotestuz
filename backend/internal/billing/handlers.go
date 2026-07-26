package billing

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	// ManualIngestToken authenticates humo-watcher → POST /internal/manual-pay/ingest.
	ManualIngestToken string
}

// Routes mounts the public, unauthenticated billing endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/tariffs", h.listTariffs)
	r.Get("/billing/providers", h.listProviders)
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.ListProviderStatuses(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "providers query failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, out)
}

// AuthedRoutes mounts the billing endpoints that require a JWT-authed
// profile. Callers mount this behind auth.Required, e.g.
// bh.AuthedRoutes(api.With(auth.Required(...))).
func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Post("/me/checkout", h.checkout)
	r.Get("/me/payments/{id}/manual", h.getManualPayment)
	r.Post("/me/payments/{id}/manual/claim", h.claimManualPayment)
	r.Post("/billing/promo/validate", h.validatePromo)
	r.Get("/me/referral", h.getReferralStats)
	r.Get("/me/referral/activity", h.getReferralActivity)
	r.Get("/me/referral/ledger", h.getReferralLedger)
	r.Post("/me/referral/payout", h.requestReferralPayout)
	r.Post("/referral/apply", h.applyReferral)
}

// InternalRoutes mounts machine-to-machine endpoints (humo-watcher).
func (h *Handler) InternalRoutes(r chi.Router) {
	r.Post("/internal/manual-pay/ingest", h.ingestManualPay)
	r.Get("/internal/manual-pay/tg-credentials", h.getManualTgCredentials)
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
		if !writePromoOrTariffError(w, err) {
			httpx.Error(w, http.StatusInternalServerError, "internal", "promo validation failed")
		}
		return
	}
	httpx.Data(w, http.StatusOK, res)
}

// writePromoOrTariffError writes the client-facing response for a promo
// rejection or an unknown tariff and reports whether it recognized err.
// Shared by validatePromo and checkout: the two used to disagree, with
// checkout collapsing every promo domain error into 500 "checkout failed".
// That mattered beyond UX — a promo limit correctly rejected by the row lock
// looked exactly like a server fault, so the alert for "someone is hammering
// a promo code" was indistinguishable from a real outage, and the on-call
// instinct would be to hunt for a bug in the lock itself.
func writePromoOrTariffError(w http.ResponseWriter, err error) bool {
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
		return false
	}
	return true
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
	if provider != "payme" && provider != "click" && provider != "manual" {
		httpx.Error(w, http.StatusBadRequest, "invalid_provider", "provider must be payme, click, or manual")
		return
	}

	if provider == "manual" {
		info, err := h.Svc.StartManualCheckout(r.Context(), claims.ProfileID, body.TariffCode, body.PromoCode)
		if err != nil {
			if errors.Is(err, ErrProviderDisabled) {
				httpx.Error(w, http.StatusServiceUnavailable, "provider_unavailable",
					"this payment method is temporarily unavailable")
				return
			}
			if errors.Is(err, ErrManualNoCardAvailable) {
				httpx.Error(w, http.StatusServiceUnavailable, "manual_busy",
					"all receiving cards are busy; try again shortly")
				return
			}
			if writeReferralError(w, err) {
				return
			}
			if !writePromoOrTariffError(w, err) {
				httpx.Error(w, http.StatusInternalServerError, "internal", "checkout failed")
			}
			return
		}
		httpx.Data(w, http.StatusOK, CheckoutResult{
			PaymentID: info.PaymentID,
			Manual:    &info,
		})
		return
	}

	cfg := CheckoutConfig{
		PaymeMerchantID:   h.PaymeMerchantID,
		PaymeCheckoutHost: h.PaymeCheckoutHost,
		ClickServiceID:    h.ClickServiceID,
		ClickMerchantID:   h.ClickMerchantID,
	}
	// Server-built return URL: Payme/Click redirect here after payment so the
	// pending page can poll entitlement. Never taken from the client — that
	// would be an open-redirect vector.
	returnURL := h.Svc.checkoutPendingReturnURL(loc)
	result, err := h.Svc.StartCheckout(r.Context(), claims.ProfileID, body.TariffCode, provider, cfg, loc, returnURL, body.PromoCode)
	if err != nil {
		if errors.Is(err, ErrProviderDisabled) {
			httpx.Error(w, http.StatusServiceUnavailable, "provider_unavailable",
				"this payment method is temporarily unavailable")
			return
		}
		if writeReferralError(w, err) {
			return
		}
		if !writePromoOrTariffError(w, err) {
			httpx.Error(w, http.StatusInternalServerError, "internal", "checkout failed")
		}
		return
	}
	httpx.Data(w, http.StatusOK, result)
}

func (h *Handler) getManualPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "payment id required")
		return
	}
	out, err := h.Svc.GetManualPaymentStatus(r.Context(), claims.ProfileID, id)
	if err != nil {
		if errors.Is(err, ErrManualNotFound) || errors.Is(err, ErrManualWrongOwner) {
			httpx.Error(w, http.StatusNotFound, "not_found", "manual payment not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load manual payment")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) claimManualPayment(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "payment id required")
		return
	}
	out, err := h.Svc.ClaimManualPayment(r.Context(), claims.ProfileID, id)
	if err != nil {
		if errors.Is(err, ErrManualNotFound) || errors.Is(err, ErrManualWrongOwner) {
			httpx.Error(w, http.StatusNotFound, "not_found", "manual payment not found")
			return
		}
		if errors.Is(err, ErrManualAlreadyDone) {
			httpx.Error(w, http.StatusConflict, "already_finalized", "payment already finalized")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "claim failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type ingestManualBody struct {
	RawText       string `json:"raw_text"`
	TelegramMsgID int64  `json:"telegram_msg_id"`
}

func (h *Handler) requireManualIngestAuth(w http.ResponseWriter, r *http.Request) bool {
	if h.ManualIngestToken == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "manual ingest not configured")
		return false
	}
	got := r.Header.Get("X-Internal-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.ManualIngestToken)) != 1 {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return false
	}
	return true
}

func (h *Handler) ingestManualPay(w http.ResponseWriter, r *http.Request) {
	if !h.requireManualIngestAuth(w, r) {
		return
	}
	var body ingestManualBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RawText == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "raw_text required")
		return
	}
	out, err := h.Svc.IngestHumoPush(r.Context(), body.RawText, body.TelegramMsgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "ingest failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getManualTgCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.requireManualIngestAuth(w, r) {
		return
	}
	out, err := h.Svc.GetManualTgCredentialsPlain(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "credentials unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, out)
}

type applyReferralBody struct {
	Code string `json:"code"`
}

func (h *Handler) getReferralStats(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	stats, err := h.Svc.GetReferralStats(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to get referral stats")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, stats)
}

func (h *Handler) applyReferral(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	var body applyReferralBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "JSON payload expected")
		return
	}
	err := h.Svc.ApplyReferralCode(r.Context(), claims.ProfileID, body.Code)
	if err != nil {
		if !writeReferralError(w, err) {
			httpx.Error(w, http.StatusInternalServerError, "internal", "failed to apply referral code")
		}
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"success": true})
}

func writeReferralError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, ErrReferralNotFound):
		httpx.Error(w, http.StatusBadRequest, "referral_not_found", "referral code not found")
	case errors.Is(err, ErrReferralSelf):
		httpx.Error(w, http.StatusBadRequest, "referral_self", "cannot apply your own referral code")
	case errors.Is(err, ErrReferralAlreadyApplied):
		httpx.Error(w, http.StatusBadRequest, "referral_already_applied", "referral code already applied")
	case errors.Is(err, ErrReferralNotEligiblePaid):
		httpx.Error(w, http.StatusBadRequest, "referral_not_eligible_paid", "account already has a paid payment")
	case errors.Is(err, ErrReferralWindowClosed):
		httpx.Error(w, http.StatusBadRequest, "referral_window_closed", "referral attach window has closed")
	default:
		return false
	}
	return true
}

func (h *Handler) getReferralActivity(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	out, err := h.Svc.GetReferralActivity(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load referral activity")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getReferralLedger(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	entries, err := h.Svc.ListReferralLedger(r.Context(), claims.ProfileID, 50)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to load ledger")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"entries": entries})
}

type payoutBody struct {
	AmountUzs   int64  `json:"amount_uzs"`
	CardNumber  string `json:"card_number"`
	CardNetwork string `json:"card_network"`
}

func (h *Handler) requestReferralPayout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	var body payoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "JSON payload expected")
		return
	}
	res, err := h.Svc.RequestReferralPayout(r.Context(), claims.ProfileID, body.AmountUzs, body.CardNumber, body.CardNetwork)
	if err != nil {
		switch {
		case errors.Is(err, ErrPayoutInvalidAmount):
			httpx.Error(w, http.StatusBadRequest, "payout_invalid_amount", "amount must be positive")
		case errors.Is(err, ErrPayoutInvalidCard):
			httpx.Error(w, http.StatusBadRequest, "payout_invalid_card", "invalid card number")
		case errors.Is(err, ErrPayoutInvalidNetwork):
			httpx.Error(w, http.StatusBadRequest, "payout_invalid_network", "card_network must be uzcard or humo")
		case errors.Is(err, ErrPayoutInsufficientBalance):
			httpx.Error(w, http.StatusBadRequest, "payout_insufficient_balance", "insufficient referral balance")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "payout request failed")
		}
		return
	}
	httpx.Data(w, http.StatusOK, res)
}
