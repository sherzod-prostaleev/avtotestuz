package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/httpx"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func (s Store) SetB2BMobilePromo(ctx context.Context, orgID uuid.UUID, p b2b.MobilePromo, audit MutationAudit) error {
	if err := b2b.ValidateMobilePromo(p.Enabled, p.URL); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var before b2b.MobilePromo
	if err := tx.QueryRow(ctx, `SELECT mobile_promo_enabled, mobile_promo_url FROM b2b_org WHERE id=$1 FOR UPDATE`, orgID).Scan(&before.Enabled, &before.URL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE b2b_org SET mobile_promo_enabled=$2, mobile_promo_url=$3 WHERE id=$1`, orgID, p.Enabled, p.URL); err != nil {
		return err
	}
	// Audit the setting and actor, but do not copy possibly signed URL query
	// strings into logs. The original URL stays only in the school settings.
	if err := writeAuditTx(ctx, tx, audit, "b2b.orgs.mobile_promo", "b2b_org", orgID.String(),
		map[string]any{"enabled": before.Enabled}, map[string]any{"enabled": p.Enabled, "url_changed": p.URL != before.URL}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) getB2BMobilePromo(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	out, err := h.b2bStore().MobilePromo(r.Context(), id)
	if err != nil {
		writeMobilePromoError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) putB2BMobilePromo(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Enabled *bool   `json:"enabled"`
		URL     *string `json:"url"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Enabled == nil || body.URL == nil {
		httpx.Error(w, 400, "invalid_body", "enabled and url are required")
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		httpx.Error(w, 400, "invalid_body", "one JSON object required")
		return
	}
	p := b2b.MobilePromo{Enabled: *body.Enabled, URL: *body.URL}
	if err := b2b.ValidateMobilePromo(p.Enabled, p.URL); err != nil {
		writeMobilePromoError(w, err)
		return
	}
	if err := p.GenerateQR(); err != nil {
		writeMobilePromoError(w, err)
		return
	}
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, 401, "unauthorized", "missing auth")
		return
	}
	err := h.Svc.Store.SetB2BMobilePromo(r.Context(), id, p, MutationAudit{AdminUserID: claims.AdminUserID, IP: clientIP(r), UA: r.UserAgent(), RequestID: middleware.GetReqID(r.Context())})
	if err != nil {
		writeMobilePromoError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, 200, p)
}

func writeMobilePromoError(w http.ResponseWriter, err error) {
	switch {
	case IsNoRows(err):
		httpx.Error(w, 404, "not_found", "school not found")
	case errors.Is(err, b2b.ErrInvalid):
		httpx.Error(w, 400, "invalid_url", "use an exact HTTP(S) URL up to 512 bytes without whitespace or credentials")
	default:
		httpx.Error(w, 500, "internal", "mobile promotion operation failed")
	}
}
