package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/httpx"
)

func (h *Handler) listReferralPayouts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = int32(n)
		}
	}
	out, err := h.Svc.Store.ListReferralPayouts(r.Context(), status, limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "list payouts failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type payoutActionBody struct {
	Note string `json:"note"`
}

func (h *Handler) markReferralPayoutPaid(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing admin")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid payout id")
		return
	}
	var body payoutActionBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.Svc.Store.MarkReferralPayoutPaid(r.Context(), id, claims.AdminUserID, body.Note); err != nil {
		if errors.Is(err, ErrReferralPayoutNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "payout not found")
			return
		}
		if errors.Is(err, ErrReferralPayoutNotPending) {
			httpx.Error(w, http.StatusConflict, "not_pending", "payout is not pending")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "mark paid failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) rejectReferralPayout(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing admin")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid payout id")
		return
	}
	var body payoutActionBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.Svc.Store.RejectReferralPayout(r.Context(), id, claims.AdminUserID, body.Note); err != nil {
		if errors.Is(err, ErrReferralPayoutNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "payout not found")
			return
		}
		if errors.Is(err, ErrReferralPayoutNotPending) {
			httpx.Error(w, http.StatusConflict, "not_pending", "payout is not pending")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "reject failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) getUserReferral(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	out, err := h.Svc.Store.GetUserReferralAdmin(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrReferralPayoutNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "referral stats failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type patchReferralRateBody struct {
	Percent int32 `json:"percent"`
}

func (h *Handler) patchUserReferralRate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	var body patchReferralRateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON")
		return
	}
	percent, err := h.Svc.Store.UpdateUserReferralCommissionPercent(r.Context(), id, body.Percent)
	if err != nil {
		if errors.Is(err, ErrReferralPayoutNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "invalid_percent", err.Error())
		return
	}
	httpx.Data(w, http.StatusOK, map[string]int32{"commission_percent": percent})
}

type setReferralBalanceBody struct {
	BalanceUzs int64  `json:"balance_uzs"`
	Note       string `json:"note"`
}

func (h *Handler) putUserReferralBalance(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing admin")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	var body setReferralBalanceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON")
		return
	}
	bal, err := h.Svc.Store.SetUserReferralBalance(r.Context(), id, claims.AdminUserID, body.BalanceUzs, body.Note)
	if err != nil {
		if errors.Is(err, ErrReferralPayoutNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "invalid_balance", err.Error())
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "referral.balance.set", "profile", id.String(),
		nil, map[string]any{"balance_uzs": bal, "note": body.Note},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]int64{"balance_uzs": bal})
}

func (h *Handler) getUserReferralLedger(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	entries, err := h.Svc.Store.ListUserReferralLedger(r.Context(), id, 50)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "ledger failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"entries": entries})
}
