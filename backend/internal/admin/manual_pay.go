package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
)

type manualPayCardAdminRow struct {
	ID         uuid.UUID `json:"id"`
	Network    string    `json:"network"`
	PanMasked  string    `json:"pan_masked"`
	PanLast4   string    `json:"pan_last4"`
	HolderName string    `json:"holder_name"`
	SortOrder  int32     `json:"sort_order"`
	Enabled    bool      `json:"enabled"`
}

func manualPayCardAdminDTO(row sqlc.ManualPayCard) manualPayCardAdminRow {
	return manualPayCardAdminRow{
		ID: row.ID, Network: row.Network,
		PanMasked: billing.MaskCardNumber(row.PanLast4), PanLast4: row.PanLast4,
		HolderName: row.HolderName, SortOrder: row.SortOrder, Enabled: row.Enabled,
	}
}

func (h *Handler) listManualPayCards(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Billing.Q.ListManualPayCards(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "list cards failed")
		return
	}
	out := make([]manualPayCardAdminRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, manualPayCardAdminDTO(row))
	}
	httpx.Data(w, http.StatusOK, out)
}

type manualCardBody struct {
	PanFull    string `json:"pan_full"`
	HolderName string `json:"holder_name"`
	SortOrder  int32  `json:"sort_order"`
	Enabled    *bool  `json:"enabled"`
}

func (h *Handler) createManualPayCard(w http.ResponseWriter, r *http.Request) {
	var body manualCardBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "JSON expected")
		return
	}
	full, last4, err := h.Billing.EncryptPAN(body.PanFull)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_card", "card number must be 16–19 digits")
		return
	}
	if strings.TrimSpace(body.HolderName) == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_name", "holder_name required")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	row, err := h.Billing.Q.InsertManualPayCard(r.Context(), sqlc.InsertManualPayCardParams{
		Network:    "humo",
		PanFull:    full,
		PanLast4:   last4,
		HolderName: strings.TrimSpace(body.HolderName),
		SortOrder:  body.SortOrder,
		Enabled:    enabled,
	})
	if err != nil {
		if strings.Contains(err.Error(), "manual_pay_card_pan_last4") || strings.Contains(err.Error(), "duplicate key") {
			httpx.Error(w, http.StatusConflict, "duplicate_card", "card with this last4 already exists")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "create card failed")
		return
	}
	claims, _ := FromContext(r.Context())
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.card.create", "manual_pay_card", row.ID.String(),
		nil, nil, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusCreated, manualPayCardAdminDTO(row))
}

func (h *Handler) updateManualPayCard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "card id required")
		return
	}
	var body manualCardBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "JSON expected")
		return
	}
	existing, err := h.Billing.Q.GetManualPayCard(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	full, last4 := existing.PanFull, existing.PanLast4
	if strings.TrimSpace(body.PanFull) != "" {
		full, last4, err = h.Billing.EncryptPAN(body.PanFull)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_card", "card number must be 16–19 digits")
			return
		}
	}
	holder := existing.HolderName
	if strings.TrimSpace(body.HolderName) != "" {
		holder = strings.TrimSpace(body.HolderName)
	}
	enabled := existing.Enabled
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	sortOrder := body.SortOrder
	if body.SortOrder == 0 {
		sortOrder = existing.SortOrder
	}
	row, err := h.Billing.Q.UpdateManualPayCard(r.Context(), sqlc.UpdateManualPayCardParams{
		ID:         id,
		PanFull:    full,
		PanLast4:   last4,
		HolderName: holder,
		SortOrder:  sortOrder,
		Enabled:    enabled,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "update card failed")
		return
	}
	claims, _ := FromContext(r.Context())
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.card.update", "manual_pay_card", id.String(),
		nil, nil, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, manualPayCardAdminDTO(row))
}

func (h *Handler) revealManualPayCard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "card id required")
		return
	}
	row, err := h.Billing.Q.GetManualPayCard(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	pan, err := h.Billing.DecryptPAN(row.PanFull)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "card decrypt failed")
		return
	}
	claims, _ := FromContext(r.Context())
	if err := h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.card.reveal", "manual_pay_card", id.String(),
		nil, map[string]any{"pan_last4": row.PanLast4}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context())); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "audit write failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, map[string]string{"pan_full": pan})
}

// retireManualPayCard preserves the card and every historical assignment. A
// retired card is excluded from new checkout allocation by enabled=false.
func (h *Handler) retireManualPayCard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "card id required")
		return
	}
	row, err := h.Billing.Q.GetManualPayCard(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	row, err = h.Billing.Q.UpdateManualPayCard(r.Context(), sqlc.UpdateManualPayCardParams{
		ID:         row.ID,
		PanFull:    row.PanFull,
		PanLast4:   row.PanLast4,
		HolderName: row.HolderName,
		SortOrder:  row.SortOrder,
		Enabled:    false,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "retire failed")
		return
	}
	claims, _ := FromContext(r.Context())
	if err := h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.card.retire", "manual_pay_card", id.String(),
		map[string]any{"enabled": true}, map[string]any{"enabled": false},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context())); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "audit write failed")
		return
	}
	httpx.Data(w, http.StatusOK, manualPayCardAdminDTO(row))
}

func (h *Handler) listManualPayQueue(w http.ResponseWriter, r *http.Request) {
	_ = h.Billing.Q.ReleaseExpiredManualHolds(r.Context())
	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}
	rows, err := h.Billing.Q.ListOpenManualPayAssignments(r.Context(), limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "queue failed")
		return
	}
	httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) confirmManualPay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "payment id required")
		return
	}
	already, err := h.Billing.ConfirmManualPayment(r.Context(), id, "admin")
	if err != nil {
		if errors.Is(err, billing.ErrManualNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		if errors.Is(err, billing.ErrManualAlreadyDone) {
			httpx.Error(w, http.StatusConflict, "already_finalized", "already finalized")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "confirm failed")
		return
	}
	claims, _ := FromContext(r.Context())
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.confirm", "payment", id.String(),
		nil, map[string]any{"already": already}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]any{"ok": true, "already_confirmed": already})
}

type rejectManualBody struct {
	Note string `json:"note"`
}

func (h *Handler) rejectManualPay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "payment id required")
		return
	}
	var body rejectManualBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.Billing.RejectManualPayment(r.Context(), id, body.Note); err != nil {
		if errors.Is(err, billing.ErrManualNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		if errors.Is(err, billing.ErrManualRejectPaid) {
			httpx.Error(w, http.StatusConflict, "already_paid", "cannot reject paid payment")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "reject failed")
		return
	}
	claims, _ := FromContext(r.Context())
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.reject", "payment", id.String(),
		nil, nil, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) listUnmatchedManualEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Billing.Q.ListUnmatchedManualPayEvents(r.Context(), 50)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "list unmatched failed")
		return
	}
	httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) ignoreManualEvent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "event id required")
		return
	}
	if err := h.Billing.Q.MarkManualPayEventIgnored(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "ignore failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) getManualTgSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.Billing.GetManualTgPublicSettings(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "settings failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type manualTgBody struct {
	APIID           string `json:"api_id"`
	APIHash         string `json:"api_hash"`
	Session         string `json:"session"`
	HumoBotUsername string `json:"humo_bot_username"`
}

func (h *Handler) putManualTgSettings(w http.ResponseWriter, r *http.Request) {
	var body manualTgBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "JSON expected")
		return
	}
	out, err := h.Billing.UpdateManualTgSettings(r.Context(), billing.ManualTgUpdate{
		APIID:           body.APIID,
		APIHash:         body.APIHash,
		Session:         body.Session,
		HumoBotUsername: body.HumoBotUsername,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_settings", err.Error())
		return
	}
	claims, _ := FromContext(r.Context())
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "manual_pay.tg.update", "manual_tg_settings", "1",
		nil, nil, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) testManualTgSettings(w http.ResponseWriter, r *http.Request) {
	ok, detail, err := h.Billing.TestManualTgSettingsDecrypt(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "test failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"ok": ok, "detail": detail})
}
